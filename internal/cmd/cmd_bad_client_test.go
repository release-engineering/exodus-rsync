package cmd

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/gw"
)

// Command fails if exodus-gw client can't be initialized.
func TestMainBadClient(t *testing.T) {
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
		"exodus-rsync", ".", "some-dest:/foo/bar",
	})

	// It should exit.
	if got != 101 {
		t.Error("returned incorrect exit code", got)
	}

	// TODO: test log entries.
}
