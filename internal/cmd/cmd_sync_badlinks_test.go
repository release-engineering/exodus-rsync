package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/gw"
)

func TestMainSyncBadLinks(t *testing.T) {
	SetConfig(t, CONFIG)

	logs := CaptureLogger(t)

	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}

	t.Run("fails on broken symlink", func(t *testing.T) {
		os.Mkdir("broken-link", 0755)
		err := os.Symlink("/this/file/does/not/exist", "broken-link/src")
		if err != nil {
			t.Fatalf("can't make symlink, err = %v", err)
		}

		args := []string{"rsync", "-vvv", "broken-link", "exodus:/some/target"}

		mockGw.EXPECT().NewClient(EnvMatcher{"best-env"}).Return(&client, nil)
		got := Main(args)

		// It should fail
		if got != 73 {
			t.Errorf("got unexpected exit code = %v", got)
		}

		// It should tell us why
		entry := FindEntry(logs, "can't read files for sync")
		if entry == nil {
			t.Error("missing expected log message")
		}

		errMessage := fmt.Sprint(entry.Fields["error"])
		if !strings.Contains(errMessage, "resolving link broken-link/src") {
			t.Error("unexpected error message", errMessage)
		}
	})

}
