package gw

import (
	"errors"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// isNotFound reports whether err from an S3 HeadObject call means the object
// does not exist. exodus-gw is not AWS S3; treat HTTP 404 as authoritative when
// typed not-found errors are absent or deserialization fails on an empty body.
func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var notFound *types.NotFound
	if errors.As(err, &notFound) {
		return true
	}

	var apiErr *smithy.GenericAPIError
	if errors.As(err, &apiErr) {
		switch apiErr.Code {
		case "NotFound", "NoSuchKey", "404":
			return true
		}
	}

	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == 404 {
		return true
	}

	return false
}
