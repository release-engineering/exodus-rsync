package cmd

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
)

func MockController(t *testing.T) *gomock.Controller {
	oldExt := ext
	t.Cleanup(func() { ext = oldExt })

	return gomock.NewController(t)
}

// Ensure exodus-rsync.conf contains given text for duration of current test.
// Also changes the current working directory to a tempdir.
func SetConfig(t *testing.T, config string) {
	oldDir, err := os.Getwd()
	if err != nil {
		t.Fatal("getwd:", err)
	}

	temp := t.TempDir()

	if err = os.Chdir(temp); err != nil {
		t.Fatal("chdir:", err)
	}

	t.Cleanup(func() {
		if err := os.Chdir(oldDir); err != nil {
			t.Fatal("chdir (cleanup):", err)
		}
	})

	if err = os.WriteFile("exodus-rsync.conf", []byte(config), 0644); err != nil {
		t.Fatal("writing config file:", err)
	}
}
