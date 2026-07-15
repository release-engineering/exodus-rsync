package gw

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

// blobMap maps object keys to a queue of errors returned on successive S3 operations.
// A nil error in the queue means success for that operation.
type blobMap map[string][]error

// errFakeObjectMissing is popped from a blob queue to simulate a missing object on HEAD.
var errFakeObjectMissing = errors.New("fake S3 object missing")

// exodus-gw HEAD-miss shape: HTTP 404, Content-Type application/xml, Content-Length 226, empty body.
const fakeExodusGWHeadMissContentLength = "226"

// fakeS3Request records one HTTP request handled by the fake server.
type fakeS3Request struct {
	Method   string
	Path     string
	RawQuery string
	Header   http.Header
}

type fakeMPUSession struct {
	bucket string
	key    string
}

type fakeS3ServerConfig struct {
	Blobs blobMap
	// MPUFailUploadParts makes every UploadPart return an S3 InternalError.
	MPUFailUploadParts bool
	// MPUFailComplete makes CompleteMultipartUpload return an S3 InternalError
	// without committing the object (the MPU session remains until abort).
	MPUFailComplete bool
}

type fakeS3HTTPServer struct {
	*httptest.Server
	client *http.Client

	mu         sync.Mutex
	blobs      blobMap
	requests   []fakeS3Request
	mpus       map[string]*fakeMPUSession
	nextUpload int

	mpuFailUploadParts bool
	mpuFailComplete    bool
}

func (s *fakeS3HTTPServer) Client() *http.Client {
	return s.client
}

func (s *fakeS3HTTPServer) Blobs() blobMap {
	return s.blobs
}

func (s *fakeS3HTTPServer) Requests() []fakeS3Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]fakeS3Request, len(s.requests))
	copy(out, s.requests)
	return out
}

func (s *fakeS3HTTPServer) Reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.blobs = make(blobMap)
	s.requests = nil
	s.mpus = make(map[string]*fakeMPUSession)
	s.nextUpload = 0
}

func newFakeS3HTTPServer(t *testing.T, cfg fakeS3ServerConfig) *fakeS3HTTPServer {
	t.Helper()

	if cfg.Blobs == nil {
		cfg.Blobs = make(blobMap)
	}

	fake := &fakeS3HTTPServer{
		blobs:              cfg.Blobs,
		mpus:               make(map[string]*fakeMPUSession),
		mpuFailUploadParts: cfg.MPUFailUploadParts,
		mpuFailComplete:    cfg.MPUFailComplete,
	}

	fake.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fake.mu.Lock()
		defer fake.mu.Unlock()

		fake.requests = append(fake.requests, fakeS3Request{
			Method:   r.Method,
			Path:     r.URL.Path,
			RawQuery: r.URL.RawQuery,
			Header:   r.Header.Clone(),
		})

		parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/"), "/", 2)
		if len(parts) < 2 {
			http.Error(w, "bad path", http.StatusBadRequest)
			return
		}
		bucket := parts[0]
		key := parts[1]
		if i := strings.Index(key, "?"); i >= 0 {
			key = key[:i]
		}

		q := r.URL.Query()

		switch r.Method {
		case http.MethodHead:
			handleFakeS3Head(w, key, fake.blobs)
		case http.MethodPost:
			switch {
			case q.Has("uploads"):
				handleFakeS3CreateMPU(w, bucket, key, fake)
			case q.Get("uploadId") != "":
				handleFakeS3CompleteMPU(w, r, q.Get("uploadId"), fake)
			default:
				http.Error(w, "unsupported POST", http.StatusBadRequest)
			}
		case http.MethodPut:
			if q.Get("partNumber") != "" && q.Get("uploadId") != "" {
				handleFakeS3UploadPart(w, r, fake)
				return
			}
			handleFakeS3Put(w, r, key, fake.blobs)
		case http.MethodDelete:
			if q.Get("uploadId") != "" {
				handleFakeS3AbortMPU(w, q.Get("uploadId"), fake)
				return
			}
			w.WriteHeader(http.StatusMethodNotAllowed)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	fake.client = fake.Server.Client()

	return fake
}

func handleFakeS3Head(w http.ResponseWriter, key string, blobs blobMap) {
	errors, haveBlob := blobs[key]
	if !haveBlob {
		writeFakeS3NotFound(w)
		return
	}

	if errors == nil || len(errors) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := errors[0]
	blobs[key] = errors[1:]
	if err != nil {
		writeFakeS3OperationError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleFakeS3Put(w http.ResponseWriter, r *http.Request, key string, blobs blobMap) {
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()

	errors, haveBlob := blobs[key]
	if !haveBlob {
		blobs[key] = nil
		w.WriteHeader(http.StatusOK)
		return
	}

	if errors == nil || len(errors) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	err := errors[0]
	blobs[key] = errors[1:]
	if err != nil {
		writeFakeS3OperationError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func handleFakeS3CreateMPU(w http.ResponseWriter, bucket, key string, fake *fakeS3HTTPServer) {
	fake.nextUpload++
	uploadID := fmt.Sprintf("fake-mpu-upload-%d", fake.nextUpload)
	fake.mpus[uploadID] = &fakeMPUSession{bucket: bucket, key: key}

	body := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<InitiateMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`+
			`<Bucket>%s</Bucket><Key>%s</Key><UploadId>%s</UploadId>`+
			`</InitiateMultipartUploadResult>`,
		xmlEscape(bucket), xmlEscape(key), xmlEscape(uploadID),
	)
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func handleFakeS3UploadPart(w http.ResponseWriter, r *http.Request, fake *fakeS3HTTPServer) {
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()

	uploadID := r.URL.Query().Get("uploadId")
	if _, ok := fake.mpus[uploadID]; !ok {
		writeFakeS3XMLError(w, http.StatusNotFound, "NoSuchUpload", "upload not found")
		return
	}

	if fake.mpuFailUploadParts {
		writeFakeS3XMLError(w, http.StatusInternalServerError, "InternalError", "simulated part failure")
		return
	}

	body := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<UploadPartResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">` +
		`<ETag>"fake-part-etag"</ETag></UploadPartResult>`
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.Header().Set("ETag", `"fake-part-etag"`)
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func handleFakeS3CompleteMPU(w http.ResponseWriter, r *http.Request, uploadID string, fake *fakeS3HTTPServer) {
	_, _ = io.Copy(io.Discard, r.Body)
	_ = r.Body.Close()

	sess, ok := fake.mpus[uploadID]
	if !ok {
		writeFakeS3XMLError(w, http.StatusNotFound, "NoSuchUpload", "upload not found")
		return
	}

	if fake.mpuFailComplete {
		writeFakeS3XMLError(w, http.StatusInternalServerError, "InternalError", "simulated complete failure")
		return
	}

	fake.blobs[sess.key] = nil
	delete(fake.mpus, uploadID)

	body := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<CompleteMultipartUploadResult xmlns="http://s3.amazonaws.com/doc/2006-03-01/">`+
			`<Location>/%s/%s</Location><Bucket>%s</Bucket><Key>%s</Key>`+
			`<ETag>"fake-complete-etag"</ETag></CompleteMultipartUploadResult>`,
		xmlEscape(sess.bucket), xmlEscape(sess.key),
		xmlEscape(sess.bucket), xmlEscape(sess.key),
	)
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, body)
}

func handleFakeS3AbortMPU(w http.ResponseWriter, uploadID string, fake *fakeS3HTTPServer) {
	if _, ok := fake.mpus[uploadID]; !ok {
		writeFakeS3XMLError(w, http.StatusNotFound, "NoSuchUpload", "upload not found")
		return
	}

	delete(fake.mpus, uploadID)
	w.WriteHeader(http.StatusNoContent)
}

func writeFakeS3OperationError(w http.ResponseWriter, err error) {
	if errors.Is(err, errFakeObjectMissing) {
		writeFakeS3NotFound(w)
		return
	}
	writeFakeS3XMLError(w, http.StatusInternalServerError, "InternalError", err.Error())
}

// writeFakeS3NotFound matches exodus-gw HEAD miss: 404, application/xml,
// Content-Length 226, empty body (not AWS S3's XML NoSuchKey document).
func writeFakeS3NotFound(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", fakeExodusGWHeadMissContentLength)
	w.WriteHeader(http.StatusNotFound)
}

// writeFakeS3XMLError returns a standard S3 API error document. PUT failures
// surface Code and Message through the v2 SDK; HEAD 5xx generally do not.
func writeFakeS3XMLError(w http.ResponseWriter, status int, code, message string) {
	body := fmt.Sprintf(
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<Error><Code>%s</Code><Message>%s</Message></Error>`,
		xmlEscape(code), xmlEscape(message),
	)
	w.Header().Set("Content-Type", "application/xml")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(status)
	_, _ = io.WriteString(w, body)
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func newTestS3Client(t *testing.T, endpoint string, creds aws.CredentialsProvider, httpClient *http.Client, retryMaxAttempts int) *s3.Client {
	t.Helper()

	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	if retryMaxAttempts == 0 {
		retryMaxAttempts = 3
	}

	cfg := aws.Config{
		Region:           "us-east-1",
		Credentials:      creds,
		HTTPClient:       httpClient,
		BaseEndpoint:     aws.String(endpoint),
		RetryMaxAttempts: retryMaxAttempts,
	}
	exodusGWChecksumConfig(&cfg)

	return s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.UsePathStyle = true
	})
}

func newTestS3ClientAnonymous(t *testing.T, endpoint string, httpClient *http.Client) *s3.Client {
	t.Helper()
	return newTestS3Client(t, endpoint, aws.AnonymousCredentials{}, httpClient, 3)
}

func headObjectInput(bucket, key string) *s3.HeadObjectInput {
	return &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	}
}

func newClientWithFakeS3(t *testing.T) (*client, *fakeS3HTTPServer) {
	t.Helper()

	srv := newFakeS3HTTPServer(t, fakeS3ServerConfig{})
	cfg := testConfig(t)

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	iface, err := Package.NewClient(ctx, cfg)
	if err != nil {
		t.Fatal("creating client:", err)
	}

	out := iface.(*client)
	attachClientToFakeS3(t, out, srv)
	return out, srv
}

func attachClientToFakeS3(t *testing.T, c *client, srv *fakeS3HTTPServer) {
	t.Helper()
	// Disable SDK retries so queued fake errors are not consumed by retry attempts.
	c.s3 = newTestS3Client(t, srv.URL, aws.AnonymousCredentials{}, srv.Client(), 1)
	c.uploader = newS3Uploader(c.s3)
}

func TestCredentialParityNoSigningHeaders(t *testing.T) {
	t.Parallel()

	var captured http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestS3ClientAnonymous(t, srv.URL, srv.Client())
	_, err := client.HeadObject(context.Background(), headObjectInput("env", "key"))
	if err != nil {
		t.Fatalf("HeadObject: %v", err)
	}
	if captured == nil {
		t.Fatal("expected anonymous HeadObject request to reach fake server")
	}
	for _, name := range []string{"Authorization", "X-Amz-Date", "X-Amz-Security-Token"} {
		if v := captured.Get(name); v != "" {
			t.Fatalf("unexpected signing header %s: %q", name, v)
		}
	}

	// SDK v2 rejects empty static credentials before a request is sent; production
	// uses AnonymousCredentials instead (see client.go).
	captured = nil
	staticClient := newTestS3Client(t, srv.URL, credentials.NewStaticCredentialsProvider("", "", ""), srv.Client(), 3)
	_, err = staticClient.HeadObject(context.Background(), headObjectInput("env", "key"))
	if err == nil {
		t.Fatal("expected static empty creds to be rejected before request")
	}
	if captured != nil {
		t.Fatal("unexpected request sent with empty static credentials")
	}
	if !strings.Contains(err.Error(), "static credentials are empty") {
		t.Fatalf("unexpected static creds error: %v", err)
	}
}

func TestPutObjectSendsContentMD5(t *testing.T) {
	t.Parallel()

	var captured http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newTestS3ClientAnonymous(t, srv.URL, srv.Client())
	uploader := newS3Uploader(client)

	body := strings.NewReader("exodus-gw smoke payload")
	_, err := uploader.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
		Bucket: aws.String("pre"),
		Key:    aws.String("6211cd5d84900da8a2934ad24ee764df8a5f6e84c2adbf5d64c7cbbbbf9b3c00"),
		Body:   body,
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	if captured.Get("Content-Md5") == "" {
		t.Fatal("expected Content-MD5 header on PUT")
	}
	for _, name := range []string{"X-Amz-Checksum-Crc32", "Content-Encoding"} {
		if v := captured.Get(name); v != "" {
			t.Fatalf("unexpected header %s: %q (gateway requires plain Content-MD5)", name, v)
		}
	}
}

func TestS3RequestURLPathStyle(t *testing.T) {
	t.Parallel()

	const bucket = "pre"
	const key = "abc123def456abc123def456abc123def456abc123def456abc123def456abcd"

	srv := newFakeS3HTTPServer(t, fakeS3ServerConfig{})
	defer srv.Close()

	// BaseEndpoint is the upload root, as with GwURL()+"/upload" in production.
	client := newTestS3ClientAnonymous(t, srv.URL, srv.Client())

	_, err := client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		t.Fatal("expected HEAD miss error")
	}

	uploader := newS3Uploader(client)
	_, err = uploader.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   strings.NewReader("small payload"),
	})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}

	wantPath := "/" + bucket + "/" + key
	var sawHead, sawPut bool
	for _, req := range srv.Requests() {
		if req.Path != wantPath {
			continue
		}
		switch req.Method {
		case http.MethodHead:
			sawHead = true
		case http.MethodPut:
			if !strings.Contains(req.RawQuery, "partNumber=") {
				sawPut = true
			}
		}
	}

	if !sawHead {
		t.Fatalf("expected path-style HEAD %s, got %#v", wantPath, srv.Requests())
	}
	if !sawPut {
		t.Fatalf("expected path-style PUT %s, got %#v", wantPath, srv.Requests())
	}
}

func TestMultipartUploadSendsContentMD5(t *testing.T) {
	t.Parallel()

	const bucket = "pre"
	const key = "mpu-key-mpu-key-mpu-key-mpu-key-mpu-key-mpu-key-mpu-key12"

	srv := newFakeS3HTTPServer(t, fakeS3ServerConfig{})
	defer srv.Close()

	client := newTestS3ClientAnonymous(t, srv.URL, srv.Client())
	uploader := newS3Uploader(client)

	// s3UploadPartSize is 5 MiB; one byte over forces multipart upload.
	payload := make([]byte, s3UploadPartSize+1)
	_, err := uploader.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(payload),
	})
	if err != nil {
		t.Fatalf("multipart upload: %v", err)
	}

	var sawCreate, sawPart, sawComplete bool
	var partContentMD5 string
	for _, req := range srv.Requests() {
		switch req.Method {
		case http.MethodPost:
			if strings.Contains(req.RawQuery, "uploads=") {
				sawCreate = true
			}
			if strings.Contains(req.RawQuery, "uploadId=") &&
				!strings.Contains(req.RawQuery, "uploads=") {
				sawComplete = true
			}
		case http.MethodPut:
			if strings.Contains(req.RawQuery, "partNumber=") {
				sawPart = true
				partContentMD5 = req.Header.Get("Content-Md5")
			}
		}
	}

	if !sawCreate {
		t.Fatal("expected CreateMultipartUpload (?uploads=)")
	}
	if !sawPart {
		t.Fatal("expected UploadPart (?partNumber=)")
	}
	if !sawComplete {
		t.Fatal("expected CompleteMultipartUpload (?uploadId=)")
	}
	if partContentMD5 == "" {
		t.Fatal("expected Content-MD5 on UploadPart")
	}
}

func mpuUploadIDFromRequests(requests []fakeS3Request) string {
	for _, req := range requests {
		if req.RawQuery == "" {
			continue
		}
		q, err := url.ParseQuery(req.RawQuery)
		if err != nil {
			continue
		}
		if id := q.Get("uploadId"); id != "" {
			return id
		}
	}
	return ""
}

func sawMPUAbort(requests []fakeS3Request, uploadID string) bool {
	for _, req := range requests {
		if req.Method != http.MethodDelete {
			continue
		}
		q, err := url.ParseQuery(req.RawQuery)
		if err != nil {
			continue
		}
		if q.Get("uploadId") == uploadID {
			return true
		}
	}
	return false
}

func TestMultipartUploadPartFailureAborts(t *testing.T) {
	t.Parallel()

	const bucket = "pre"
	const key = "mpu-part-fail-key-mpu-part-fail-key-mpu-part-fail-key12"

	srv := newFakeS3HTTPServer(t, fakeS3ServerConfig{MPUFailUploadParts: true})
	defer srv.Close()

	client := newTestS3ClientAnonymous(t, srv.URL, srv.Client())
	uploader := newS3Uploader(client)

	payload := make([]byte, s3UploadPartSize+1)
	_, err := uploader.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(payload),
	})
	if err == nil {
		t.Fatal("expected multipart upload to fail")
	}
	if !strings.Contains(err.Error(), "simulated part failure") {
		t.Fatalf("unexpected error: %v", err)
	}

	uploadID := mpuUploadIDFromRequests(srv.Requests())
	if uploadID == "" {
		t.Fatalf("expected uploadId in recorded requests, got %#v", srv.Requests())
	}
	if !sawMPUAbort(srv.Requests(), uploadID) {
		t.Fatalf("expected AbortMultipartUpload for %s, got %#v", uploadID, srv.Requests())
	}
	if _, ok := srv.Blobs()[key]; ok {
		t.Fatal("object should not exist after failed multipart upload")
	}
}

func TestMultipartUploadCompleteFailureAborts(t *testing.T) {
	t.Parallel()

	const bucket = "pre"
	const key = "mpu-complete-fail-key-mpu-complete-fail-key-mpu-complete12"

	srv := newFakeS3HTTPServer(t, fakeS3ServerConfig{MPUFailComplete: true})
	defer srv.Close()

	client := newTestS3ClientAnonymous(t, srv.URL, srv.Client())
	uploader := newS3Uploader(client)

	payload := make([]byte, s3UploadPartSize+1)
	_, err := uploader.UploadObject(context.Background(), &transfermanager.UploadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(payload),
	})
	if err == nil {
		t.Fatal("expected multipart upload to fail")
	}
	if !strings.Contains(err.Error(), "simulated complete failure") {
		t.Fatalf("unexpected error: %v", err)
	}

	uploadID := mpuUploadIDFromRequests(srv.Requests())
	if uploadID == "" {
		t.Fatalf("expected uploadId in recorded requests, got %#v", srv.Requests())
	}
	if !sawMPUAbort(srv.Requests(), uploadID) {
		t.Fatalf("expected AbortMultipartUpload for %s, got %#v", uploadID, srv.Requests())
	}
	if _, ok := srv.Blobs()[key]; ok {
		t.Fatal("object should not exist after failed complete")
	}
}
