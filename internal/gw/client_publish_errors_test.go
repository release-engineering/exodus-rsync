package gw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

type (
	mockReadCloser struct {
		expectedData []byte
		expectedErr  error
	}
)

func (mrc *mockReadCloser) Read(p []byte) (n int, err error) {
	copy(p, mrc.expectedData)
	return 0, mrc.expectedErr
}
func (mrc *mockReadCloser) Close() error { return nil }

func TestClientPublishErrors(t *testing.T) {
	cfg := testConfig(t)

	clientIface, err := Package.NewClient(context.Background(), cfg)
	if clientIface == nil {
		t.Errorf("failed to create client, err = %v", err)
	}

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	gw := newFakeGw(t, clientIface.(*client))

	t.Run("unable to prepare request", func(t *testing.T) {
		// Passing a nil context is used here as a way to make http.NewRequestWithContext fail.
		_, err := clientIface.NewPublish(nil)

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
	})

	t.Run("low-level error from request", func(t *testing.T) {
		gw.nextHTTPError = fmt.Errorf("simulated error")

		_, err := clientIface.NewPublish(ctx)

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "simulated error") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "https://exodus-gw.example.com/env/publish") {
			t.Errorf("Error did not include URL, got: %v", err)
		}
	})

	t.Run("unsuccessful HTTP response", func(t *testing.T) {
		gw.nextHTTPResponse = &http.Response{
			Status:     "418 I'm a teapot",
			StatusCode: 418,
			Body: io.NopCloser(&mockReadCloser{
				expectedData: []byte(""),
				expectedErr:  fmt.Errorf("Failed to read"),
			}),
		}

		_, err := clientIface.NewPublish(ctx)

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "I'm a teapot") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "https://exodus-gw.example.com/env/publish") {
			t.Errorf("Error did not include URL, got: %v", err)
		}
	})

	t.Run("invalid JSON in response", func(t *testing.T) {
		gw.nextHTTPResponse = &http.Response{
			Status:     "200 OK",
			StatusCode: 200,
			Body:       io.NopCloser(strings.NewReader("['oops', this is not valid JSON")),
		}

		_, err := clientIface.NewPublish(ctx)

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "invalid character") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
		if !strings.Contains(err.Error(), "https://exodus-gw.example.com/env/publish") {
			t.Errorf("Error did not include URL, got: %v", err)
		}
	})

	t.Run("get request fails", func(t *testing.T) {
		gw.nextHTTPResponse = &http.Response{
			Status:     "404 Publish not found",
			StatusCode: 404,
			Body:       io.NopCloser(strings.NewReader("'oops', publish not found")),
		}

		_, err := clientIface.GetPublish(ctx, "3e0a4539-be4a-437e-a45f-6d72f7192f17")
		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
	})

	t.Run("can recover by retrying", func(t *testing.T) {
		// Force an EOF, then a 500 error, and finally a successful
		// response. The caller should only see the success.
		gw.nextHTTPError = io.EOF
		gw.nextHTTPResponse = &http.Response{
			Status:     "500 Internal Server Error",
			StatusCode: 500,
			Body:       io.NopCloser(strings.NewReader("some error")),
		}

		gw.publishes["some-id"] = &fakePublish{id: "some-id"}

		p, err := clientIface.GetPublish(ctx, "some-id")
		if err != nil {
			t.Errorf("failed to get publish, err = %v", err)
		}
		id := p.ID()
		if id != "some-id" {
			t.Errorf("got unexpected id %s", id)
		}
	})

	t.Run("missing link for commit", func(t *testing.T) {
		// Create a publish object directly without filling in any Links.
		publish := publish{client: clientIface.(*client)}

		err := publish.Commit(ctx, "")

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "not eligible for commit") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
	})

	t.Run("missing link for update", func(t *testing.T) {
		publish := publish{client: clientIface.(*client)}

		err := publish.AddItems(ctx, []ItemInput{})

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "missing 'self'") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
	})

	t.Run("HTTP error during AddItems", func(t *testing.T) {
		publish := publish{client: clientIface.(*client)}
		publish.raw.Links = make(map[string]string)
		publish.raw.Links["self"] = "/publish/1234"

		gw.nextHTTPResponse = &http.Response{
			Status:     "409 Conflict",
			StatusCode: 409,
			Body: io.NopCloser(strings.NewReader(
				"{\"detail\": \"Publish in unexpected state\"}",
			)),
		}

		err := publish.AddItems(ctx, []ItemInput{{"/some/uri", "abc123", "mime/type", ""}})

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "Publish in unexpected state") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
	})

	t.Run("commit request fails", func(t *testing.T) {
		publish := publish{client: clientIface.(*client)}

		// Putting an incorrect 'commit' URL is enough to make the GW fake
		// return a 404 error.
		publish.raw.Links = make(map[string]string)
		publish.raw.Links["commit"] = "/some/invalid/url"

		err := publish.Commit(ctx, "")

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "404 Not Found") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
	})

}
