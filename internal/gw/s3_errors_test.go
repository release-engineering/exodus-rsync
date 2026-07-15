package gw

import (
	"context"
	"net/http"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

func TestIsNotFound(t *testing.T) {
	t.Parallel()

	resp404 := &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{
			Response: &http.Response{StatusCode: http.StatusNotFound},
		},
	}

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "NoSuchKey typed", err: &types.NoSuchKey{}, want: true},
		{name: "NotFound typed", err: &types.NotFound{}, want: true},
		{name: "GenericAPIError NotFound", err: &smithy.GenericAPIError{Code: "NotFound"}, want: true},
		{name: "GenericAPIError NoSuchKey", err: &smithy.GenericAPIError{Code: "NoSuchKey"}, want: true},
		{name: "GenericAPIError 404", err: &smithy.GenericAPIError{Code: "404"}, want: true},
		{name: "GenericAPIError other", err: &smithy.GenericAPIError{Code: "AccessDenied"}, want: false},
		{name: "bare HTTP 404", err: resp404, want: true},
		{name: "HTTP 403", err: &smithyhttp.ResponseError{
			Response: &smithyhttp.Response{Response: &http.Response{StatusCode: http.StatusForbidden}},
		}, want: false},
		{name: "network timeout", err: context.DeadlineExceeded, want: false},
		{name: "deserialization error wrapping 404", err: &smithy.DeserializationError{Err: resp404}, want: true},
		{name: "operation error wrapping 404", err: &smithy.OperationError{
			ServiceID: "S3", OperationName: "HeadObject", Err: resp404,
		}, want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isNotFound(tc.err); got != tc.want {
				t.Fatalf("isNotFound() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsNotFoundClientExodusGWHeadMiss(t *testing.T) {
	t.Parallel()

	srv := newFakeS3HTTPServer(t, fakeS3ServerConfig{Blobs: blobMap{}})
	defer srv.Close()

	client := newTestS3ClientAnonymous(t, srv.URL, srv.Client())
	_, err := client.HeadObject(context.Background(), headObjectInput("pre", "missing-key"))
	if err == nil {
		t.Fatal("expected HeadObject error for missing key")
	}
	if !isNotFound(err) {
		t.Fatalf("isNotFound() = false, want true; err=%v", err)
	}
}
