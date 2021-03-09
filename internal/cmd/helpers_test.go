package cmd

import (
	"context"
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func MockController(t *testing.T) *gomock.Controller {
	oldExt := ext
	t.Cleanup(func() { ext = oldExt })

	return gomock.NewController(t)
}

func RestoreWd(t *testing.T) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal("getwd:", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatal("chdir (cleanup):", err)
		}
	})
}

// Ensure exodus-rsync.conf contains given text for duration of current test.
// Also changes the current working directory to a tempdir.
func SetConfig(t *testing.T, config string) {
	temp := t.TempDir()

	RestoreWd(t)

	if err := os.Chdir(temp); err != nil {
		t.Fatal("chdir:", err)
	}

	if err := os.WriteFile("exodus-rsync.conf", []byte(config), 0644); err != nil {
		t.Fatal("writing config file:", err)
	}
}

// Returns a reasonably configured Context which has a real logger present.
func testContext() context.Context {
	ctx := context.Background()
	ctx = log.NewContext(ctx, ext.log.NewLogger(args.Config{}))
	return ctx
}
