package gw

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// s3UploadPartSize matches the legacy feature/s3/manager MinUploadPartSize so
// multipart threshold and part sizing stay the same after the transfermanager migration.
const s3UploadPartSize = 5 * 1024 * 1024

// exodus-gw requires Content-MD5 and Content-Length on uploads and does not
// support aws-chunked trailing checksums. AWS SDK v2 defaults to automatic CRC32
// checksums (RequestChecksumCalculationWhenSupported), which use aws-chunked on
// HTTPS and cause gateway 500 responses.
func exodusGWChecksumConfig(cfg *aws.Config) {
	cfg.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	cfg.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
}

// contentMD5UploadClient wraps *s3.Client so PutObject and UploadPart always
// include Content-MD5, which the gateway requires for single-part and MPU uploads.
type contentMD5UploadClient struct {
	*s3.Client
}

func (c *contentMD5UploadClient) PutObject(
	ctx context.Context,
	params *s3.PutObjectInput,
	optFns ...func(*s3.Options),
) (*s3.PutObjectOutput, error) {
	if params.ContentMD5 == nil && params.Body != nil {
		md5Sum, err := seekableMD5Base64(params.Body)
		if err != nil {
			return nil, err
		}
		params.ContentMD5 = aws.String(md5Sum)
	}
	return c.Client.PutObject(ctx, params, optFns...)
}

func (c *contentMD5UploadClient) UploadPart(
	ctx context.Context,
	params *s3.UploadPartInput,
	optFns ...func(*s3.Options),
) (*s3.UploadPartOutput, error) {
	if params.ContentMD5 == nil && params.Body != nil {
		md5Sum, err := seekableMD5Base64(params.Body)
		if err != nil {
			return nil, err
		}
		params.ContentMD5 = aws.String(md5Sum)
	}
	return c.Client.UploadPart(ctx, params, optFns...)
}

func seekableMD5Base64(body io.Reader) (string, error) {
	rs, ok := body.(io.ReadSeeker)
	if !ok {
		return "", fmt.Errorf("upload body must be seekable to compute Content-MD5")
	}

	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewinding upload body: %w", err)
	}

	h := md5.New()
	if _, err := io.Copy(h, rs); err != nil {
		return "", fmt.Errorf("computing Content-MD5: %w", err)
	}

	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return "", fmt.Errorf("rewinding upload body after MD5: %w", err)
	}

	return base64.StdEncoding.EncodeToString(h.Sum(nil)), nil
}

func newS3Uploader(s3Client *s3.Client) *transfermanager.Client {
	return transfermanager.New(&contentMD5UploadClient{Client: s3Client}, func(o *transfermanager.Options) {
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.PartSizeBytes = s3UploadPartSize
		o.MultipartUploadThreshold = s3UploadPartSize
	})
}
