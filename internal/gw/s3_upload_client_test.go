package gw

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

type errSeeker struct {
	inner     io.ReadSeeker
	seekErr   error
	seekCalls int
	failAfter int
}

func (e *errSeeker) Read(p []byte) (int, error) {
	return e.inner.Read(p)
}

func (e *errSeeker) Seek(offset int64, whence int) (int64, error) {
	e.seekCalls++
	if e.seekErr != nil && e.seekCalls > e.failAfter {
		return 0, e.seekErr
	}
	return e.inner.Seek(offset, whence)
}

type failReadSeeker struct{}

func (failReadSeeker) Read([]byte) (int, error)       { return 0, errors.New("read failed") }
func (failReadSeeker) Seek(int64, int) (int64, error) { return 0, nil }

type plainReader struct{}

func (plainReader) Read(p []byte) (int, error) {
	return copy(p, "payload"), io.EOF
}

func TestSeekableMD5Base64(t *testing.T) {
	t.Parallel()

	_, err := seekableMD5Base64(plainReader{})
	if err == nil || !strings.Contains(err.Error(), "seekable") {
		t.Fatalf("expected non-seekable error, got %v", err)
	}

	sum, err := seekableMD5Base64(strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sum == "" {
		t.Fatal("expected non-empty MD5")
	}
}

func TestSeekableMD5Base64SeekErrors(t *testing.T) {
	t.Parallel()

	_, err := seekableMD5Base64(&errSeeker{
		inner:     strings.NewReader("payload"),
		seekErr:   errors.New("seek failed"),
		failAfter: 0,
	})
	if err == nil || !strings.Contains(err.Error(), "rewinding upload body") {
		t.Fatalf("expected initial seek error, got %v", err)
	}

	_, err = seekableMD5Base64(&errSeeker{
		inner:     strings.NewReader("payload"),
		seekErr:   errors.New("seek failed again"),
		failAfter: 1,
	})
	if err == nil || !strings.Contains(err.Error(), "rewinding upload body after MD5") {
		t.Fatalf("expected post-MD5 seek error, got %v", err)
	}
}

func TestSeekableMD5Base64ReadError(t *testing.T) {
	t.Parallel()

	_, err := seekableMD5Base64(failReadSeeker{})
	if err == nil || !strings.Contains(err.Error(), "computing Content-MD5") {
		t.Fatalf("expected read error, got %v", err)
	}
}

func TestContentMD5UploadClientErrors(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &contentMD5UploadClient{
		Client: newTestS3ClientAnonymous(t, srv.URL, srv.Client()),
	}
	ctx := context.Background()

	_, err := client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String("pre"),
		Key:    aws.String("key"),
		Body:   plainReader{},
	})
	if err == nil || !strings.Contains(err.Error(), "seekable") {
		t.Fatalf("PutObject expected seekable error, got %v", err)
	}

	_, err = client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String("pre"),
		Key:        aws.String("key"),
		UploadId:   aws.String("upload"),
		PartNumber: aws.Int32(1),
		Body:       plainReader{},
	})
	if err == nil || !strings.Contains(err.Error(), "seekable") {
		t.Fatalf("UploadPart expected seekable error, got %v", err)
	}
}

func TestContentMD5UploadClientPreservesPresetMD5(t *testing.T) {
	t.Parallel()

	var captured http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := &contentMD5UploadClient{
		Client: newTestS3ClientAnonymous(t, srv.URL, srv.Client()),
	}

	const preset = "preset-md5-value"
	_, err := client.PutObject(context.Background(), &s3.PutObjectInput{
		Bucket:     aws.String("pre"),
		Key:        aws.String("key"),
		Body:       strings.NewReader("payload"),
		ContentMD5: aws.String(preset),
	})
	if err != nil {
		t.Fatalf("PutObject: %v", err)
	}
	if got := captured.Get("Content-Md5"); got != preset {
		t.Fatalf("expected preset Content-MD5 %q, got %q", preset, got)
	}
}
