package gw

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func TestClientPublishErrors(t *testing.T) {
	cfg := conf.Config{
		GwURL:          "https://exodus-gw.example.com",
		GwCert:         "../../test/data/service.pem",
		GwKey:          "../../test/data/service-key.pem",
		GwPollInterval: 1,
	}
	env := conf.Environment{Config: &cfg, GwEnv: "env"}

	clientIface, err := Package.NewClient(env)
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
			Body:       io.NopCloser(strings.NewReader("")),
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

	t.Run("missing link for commit", func(t *testing.T) {
		// Create a publish object directly without filling in any Links.
		publish := publish{client: clientIface.(*client)}

		err := publish.Commit(ctx)

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

	t.Run("commit request fails", func(t *testing.T) {
		publish := publish{client: clientIface.(*client)}

		// Putting an incorrect 'commit' URL is enough to make the GW fake
		// return a 404 error.
		publish.raw.Links = make(map[string]string)
		publish.raw.Links["commit"] = "/some/invalid/url"

		err := publish.Commit(ctx)

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "404 Not Found") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
	})

}
