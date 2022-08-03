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
		logger.F("key", item.Key).Debug("blob is present")
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

type uploadResult struct {
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
	for item := range items {
		if err := c.uploadBlob(ctx, item); err != nil {
			results <- uploadResult{err, item}
			break
		}
		results <- uploadResult{nil, item}
		log.FromContext(ctx).F("worker", workerID, "goroutines", runtime.NumGoroutine(), "key", item.Key).Debug("upload thread")
	}
	wg.Done()
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
	// The final collection of items to upload

	numThreads := c.cfg.UploadThreads()
	var wg sync.WaitGroup
	results := make(chan uploadResult, len(items))
	jobs := make(chan walk.SyncItem, len(items))

	for i := 0; i < numThreads; i++ {
		wg.Add(1)
		go c.uploadWorker(ctx, jobs, results, &wg, i+1)
	}

	for _, item := range items {
		if item.Key == "" && item.LinkTo != "" {
			log.FromContext(ctx).F("uri", item.SrcPath).Debug("Skipping unfollowed symlink")
			continue
		}

		// Determine if the blob is already present in the bucket
		have, err := c.haveBlob(ctx, item)
		if err != nil {
			return fmt.Errorf("checking for presence of %s: %w", item.Key, err)
		}
		// If so, do not include it in the final set of items to upload
		if have {
			if err = onPresent(item); err != nil {
				return err
			}
			continue
		}

		// Determine if the item already exists in the final set of items to upload
		// If so, prevent a costly re-upload of the item
		if _, ok := processedItems[item.Key]; ok {
			log.FromContext(ctx).F("uri", item.SrcPath).Debug("Skipping duplicate item")
			if err := onDuplicate(item); err != nil {
				return err
			}
			continue
		}
		processedItems[item.Key] = item
		jobs <- item
	}
	close(jobs)
	// Wait for all goroutines to complete
	wg.Wait()
	close(results)
	// Check for errors
	for result := range results {
		if result.Error != nil {
			return result.Error
		}
		err := onUploaded(result.Item)
		if err != nil {
			return err
		}
	}
	return nil
}

func (impl) NewClient(ctx context.Context, cfg conf.Config) (Client, error) {
	cert, err := tls.LoadX509KeyPair(cfg.GwCert(), cfg.GwKey())
	if err != nil {
		return nil, fmt.Errorf("can't load cert/key: %w", err)
	}

	out := &client{cfg: cfg}

	transport := http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates: []tls.Certificate{cert},
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
