package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/gw"
)

func TestMainSyncIgnoreExisting(t *testing.T) {
	SetConfig(t, CONFIG)

	logs := CaptureLogger(t)

	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}

	t.Run("ignore-existing OK if no files", func(t *testing.T) {
		os.Mkdir("nofiles", 0755)
		os.Mkdir("nofiles/subdir", 0755)

		args := []string{"rsync", "--ignore-existing", "nofiles", "exodus:/some/target"}

		mockGw.EXPECT().NewClient(EnvMatcher{"best-env"}).Return(&client, nil)
		got := Main(args)

		// It should succeed, though not actually do anything.
		if got != 0 {
			t.Errorf("unexpectedly failed with code = %v", got)
		}
	})

	t.Run("ignore-existing fails if files exist", func(t *testing.T) {
		os.Mkdir("files", 0755)
		os.WriteFile("files/file1", []byte("hello"), 0644)

		args := []string{"rsync", "--ignore-existing", "files", "exodus:/some/target"}

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

		err := fmt.Sprint(entry.Fields["error"])
		if !strings.Contains(err, "--ignore-existing is not supported") {
			t.Error("unexpected error message", err)
		}
	})

}
