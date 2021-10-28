package gw

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func TestClientWhoAmI(t *testing.T) {
	cfg := testConfig(t)

	clientIface, err := Package.NewClient(context.Background(), cfg)
	if clientIface == nil {
		t.Errorf("failed to create client, err = %v", err)
	}

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	gw := newFakeGw(t, clientIface.(*client))

	gw.nextHTTPResponse = &http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("{\"a\": \"b\", \"c\": \"d\"}")),
	}

	whoami, err := clientIface.WhoAmI(ctx)

	// It should have succeeded.
	if err != nil {
		t.Errorf("whoami failed: %v", err)
	}

	// The return value should be simply the raw data decoded from JSON.
	expected := map[string]interface{}{
		"a": "b",
		"c": "d",
	}
	if !reflect.DeepEqual(whoami, expected) {
		t.Errorf("unexpected whoami response, actual: %v, expected: %v", whoami, expected)
	}
}
