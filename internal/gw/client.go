package gw

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

type client struct {
	cfg        conf.Config
	httpClient *http.Client
	s3         *s3.S3
	uploader   *s3manager.Uploader
	dryRun     bool
}

func (c *client) doJSONRequest(ctx context.Context, method string, url string, body interface{}, target interface{}) error {
	var bodyReader io.Reader
	if body == nil {
		bodyReader = nil
	} else {
		buf := bytes.Buffer{}
		enc := json.NewEncoder(&buf)
		if err := enc.Encode(body); err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
		bodyReader = &buf
	}

	fullURL := c.cfg.GwURL() + url
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)

	if err != nil {
		return fmt.Errorf("preparing request to %s: %w", fullURL, err)
	}

	req.Header["Accept"] = []string{"application/json"}
	req.Header["Content-Type"] = []string{"application/json"}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		byteSlice, err := io.ReadAll(io.LimitReader(resp.Body, 2000))
		if err != nil {
			log.FromContext(ctx).F("error", err).Debugf(
				"No body in response for '%s %s'", req.Method, req.URL,
			)
		} else if len(byteSlice) > 0 {
			return fmt.Errorf("%s %s: %s, %s", req.Method, req.URL, resp.Status, byteSlice)
		}
		return fmt.Errorf("%s %s: %s", req.Method, req.URL, resp.Status)
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(target)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}

	return nil
}

func (c *client) WhoAmI(ctx context.Context) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	err := c.doJSONRequest(ctx, "GET", "/whoami", nil, &out)
	return out, err
}

func (c *client) haveBlob(ctx context.Context, item walk.SyncItem) (bool, error) {
	logger := log.FromContext(ctx)

	_, err := c.s3.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(c.cfg.GwEnv()),
		Key:    aws.String(item.Key),
	})

	if err == nil {
		logger.F("key", item.Key).Info("Skipping upload, blob is present")
		return true, nil
	}

	awsErr, isAwsErr := err.(awserr.Error)

	if isAwsErr && awsErr.Code() == "NotFound" {
		// Fine, object doesn't exist yet
		logger.F("key", item.Key).Debug("blob is not present")
		return false, nil
	}

	// Anything else is unusual
	logger.F("error", err, "key", item.Key).Warn("S3 HEAD unexpected error")

	return false, err
}

func (c *client) uploadBlob(ctx context.Context, item walk.SyncItem) error {
	logger := log.FromContext(ctx)

	var err error

	defer logger.F("src", item.SrcPath, "key", item.Key).Trace("Uploading").Stop(&err)

	if c.dryRun {
		return nil
	}

	file, err := os.Open(item.SrcPath)
	if err != nil {
		return err
	}
	defer file.Close()

	res, err := c.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket: aws.String(c.cfg.GwEnv()),
		Key:    &item.Key,
		Body:   file,
	})

	if err != nil {
		return fmt.Errorf("upload %s: %w", item.SrcPath, err)
	}

	logger.F("location", res.Location).Debug("uploaded blob")

	return nil
}

type uploadState int

const (
	uploaded  = iota // uploaded successfully
	present          // skipped because already present
	duplicate        // skipped because it's handled by another item in the same publish
	failed           // tried to upload and failed
)

type uploadResult struct {
	State uploadState
	Error error
	Item  walk.SyncItem
}

func (c *client) uploadWorker(
	ctx context.Context,
	items <-chan walk.SyncItem,
	results chan<- uploadResult,
	wg *sync.WaitGroup,
	workerID int,
) {
	defer wg.Done()

	for item := range items {
		// Determine if the blob is already present in the bucket
		have, err := c.haveBlob(ctx, item)
		if err != nil {
			results <- uploadResult{
				failed,
				fmt.Errorf("checking for presence of %s: %w", item.Key, err),
				item}
			return
		}

		// If so, no need to upload it
		if have {
			results <- uploadResult{present, nil, item}
			continue
		}

		if err := c.uploadBlob(ctx, item); err != nil {
			results <- uploadResult{failed, err, item}
			break
		}

		results <- uploadResult{uploaded, nil, item}
		log.FromContext(ctx).F("worker", workerID, "goroutines", runtime.NumGoroutine(), "key", item.Key).Debug("upload thread")
	}
}

func readUploadResults(
	out chan<- error,
	cancelFn func(),
	results <-chan uploadResult,
	onUploaded func(walk.SyncItem) error,
	onPresent func(walk.SyncItem) error,
	onDuplicate func(walk.SyncItem) error,
) {
	writtenOut := false
	sendError := func(err error) {
		if !writtenOut {
			out <- err
			writtenOut = true
		}
	}

	defer close(out)
	defer sendError(nil)

	for result := range results {
		if result.State == failed {
			sendError(result.Error)
			cancelFn()
		}

		callback := func(walk.SyncItem) error {
			return nil
		}

		switch result.State {
		case present:
			callback = onPresent
		case duplicate:
			callback = onDuplicate
		case uploaded:
			callback = onUploaded
		}

		callbackErr := callback(result.Item)

		if callbackErr != nil {
			sendError(callbackErr)
			cancelFn()
		}
	}
}

func (c *client) EnsureUploaded(
	ctx context.Context,
	items []walk.SyncItem,
	onUploaded func(walk.SyncItem) error,
	onPresent func(walk.SyncItem) error,
	onDuplicate func(walk.SyncItem) error,
) error {
	// Maintain a map of items processed thus far
	processedItems := make(map[string]walk.SyncItem)

	numThreads := c.cfg.UploadThreads()
	var wg sync.WaitGroup
	results := make(chan uploadResult, len(items))
	jobs := make(chan walk.SyncItem, len(items))

	// Make a child context so we can cancel all uploads at once if an error occurs
	// in any of them.
	uploadCtx, uploadCancel := context.WithCancel(ctx)

	// These goroutines are responsible for handling each item by reading
	// from 'jobs' and writing a result per item to 'results'.
	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go c.uploadWorker(uploadCtx, jobs, results, &wg, i+1)
	}

	// This goroutine is responsible for reading all the results as they come into
	// 'results' channel and executing callbacks as needed, as well as calculating
	// the final error state.
	//
	// Ensures that callbacks are invoked as quickly as possible, but only
	// from a single goroutine.
	out := make(chan error, 1)
	go readUploadResults(
		out, uploadCancel, results,
		onUploaded, onPresent, onDuplicate)

	// Now send all the items
	for _, item := range items {
		if item.Key == "" && item.LinkTo != "" {
			log.FromContext(ctx).F("uri", item.SrcPath).Debug("Skipping unfollowed symlink")
			continue
		}

		// Determine if the item already exists in the final set of items to upload
		// If so, ensure we put it on the queue only once
		if _, ok := processedItems[item.Key]; ok {
			log.FromContext(ctx).F("uri", item.SrcPath).Debug("Skipping duplicate item")
			// This can bypass 'jobs' completely and go straight to 'results' as we
			// know there's nothing to be done.
			results <- uploadResult{duplicate, nil, item}
			continue
		}

		processedItems[item.Key] = item
		jobs <- item
	}

	// Let the uploaders know there are no more items to process.
	close(jobs)

	// Wait for uploaders to complete.
	wg.Wait()

	// Let the results reader know there are no more results coming.
	close(results)

	// Block for the result reader to complete and return whatever
	// error (or nil) it calculated.
	return <-out
}

func (impl) NewClient(ctx context.Context, cfg conf.Config) (Client, error) {
	cert, err := tls.LoadX509KeyPair(cfg.GwCert(), cfg.GwKey())
	if err != nil {
		return nil, fmt.Errorf("can't load cert/key: %w", err)
	}

	out := &client{cfg: cfg}

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: true,
		},
	}
	out.httpClient = &http.Client{Transport: &transport}

	awsLogLevel := aws.LogOff
	if cfg.Verbosity() > 2 || cfg.LogLevel() == "trace" {
		awsLogLevel = aws.LogDebug
	}

	sess, err := ext.awsSessionProvider(session.Options{
		SharedConfigState: session.SharedConfigDisable,
		Config: aws.Config{
			Endpoint:         aws.String(cfg.GwURL() + "/upload"),
			S3ForcePathStyle: aws.Bool(true),
			Region:           aws.String("us-east-1"),
			Credentials:      credentials.AnonymousCredentials,
			HTTPClient:       out.httpClient,
			Logger:           log.FromContext(ctx),
			LogLevel:         aws.LogLevel(awsLogLevel),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create AWS session: %w", err)
	}

	out.s3 = s3.New(sess)
	out.uploader = s3manager.NewUploaderWithClient(out.s3)

	return out, nil
}
