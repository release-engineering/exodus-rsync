package gw

import (
	"context"
	"strings"
	"testing"
)

// Verify we get an error if doJSONRequest is used with something non-JSON-encodable.
func TestClientEncodeError(t *testing.T) {
	// This case can't be triggered using the client public API, so we just invoke this
	// lower level method on an instance directly.
	client := client{}

	// This could be any unmarshallable object
	x := func() {}

	err := client.doJSONRequest(context.TODO(), "POST", "https://example.com/", x, nil, nil)
	if err == nil {
		t.Error("unexpectedly did not fail")
	}

	if !strings.Contains(err.Error(), "unsupported type") {
		t.Errorf("Did not get expected error, got: %v", err)
	}
}
