package cmd

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/gw"
)

// Command fails if exodus-gw client can't be initialized.
func TestMainBadClient(t *testing.T) {
	logs := CaptureLogger(t)

	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	// Set up a valid config file with a matching prefix, so it tries to use exodus-gw.
	SetConfig(t, `
gwcert: $HOME/certs/$USER.crt
gwkey: $HOME/certs/$USER.key
gwurl: https://exodus-gw.example.com/

environments:
- prefix: some-dest
  gwenv: test
`)

	mockGw.EXPECT().NewClient(gomock.Any()).Return(nil, fmt.Errorf("client error"))

	got := Main([]string{
		"exodus-rsync", "-vvv", ".", "some-dest:/foo/bar",
	})

	// It should exit.
	if got != 101 {
		t.Error("returned incorrect exit code", got)
	}

	// It should tell us why.
	entry := FindEntry(logs, "can't initialize exodus-gw client")
	if entry == nil {
		t.Errorf("Missing expected log message")
	}

	if entry.Fields["error"].(error).Error() != "client error" {
		t.Errorf("Did not get expected error, got: %v", entry.Fields["error"])
	}
}
