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
		// TODO: attempt to read body here to log something about the failure
		return fmt.Errorf("%s %s: %s", req.Method, req.URL, resp.Status)
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(target)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL, err)
	}

	return nil
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

func (c *client) EnsureUploaded(
	ctx context.Context,
	items []walk.SyncItem,
	onUploaded func(walk.SyncItem) error,
	onPresent func(walk.SyncItem) error,
) error {
	// TODO: concurrency
	for _, item := range items {
		// Check if it's present
		have, err := c.haveBlob(ctx, item)
		if err != nil {
			return fmt.Errorf("checking for presence of %s: %w", item.Key, err)
		}

		if have {
			if err = onPresent(item); err != nil {
				return err
			}
			continue
		}

		if err = c.uploadBlob(ctx, item); err != nil {
			return err
		}
		if err = onUploaded(item); err != nil {
			return err
		}
	}

	return nil
}

func (impl) NewClient(_ context.Context, cfg conf.Config) (Client, error) {
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

	sess, err := ext.awsSessionProvider(session.Options{
		SharedConfigState: session.SharedConfigDisable,
		Config: aws.Config{
			Endpoint:         aws.String(cfg.GwURL() + "/upload"),
			S3ForcePathStyle: aws.Bool(true),

			Region:      aws.String("us-east-1"),
			Credentials: credentials.AnonymousCredentials,
			HTTPClient:  out.httpClient,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create AWS session: %w", err)
	}

	out.s3 = s3.New(sess)
	out.uploader = s3manager.NewUploaderWithClient(out.s3)

	return out, nil
}
