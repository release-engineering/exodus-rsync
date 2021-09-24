package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

func TestMainUnreadableConf(t *testing.T) {
	RestoreWd(t)

	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal("can't enter temporary directory", err)
	}

	logs := CaptureLogger(t)

	// Make a config file using the name it'll look for by default, but
	// without read permissions.
	os.WriteFile("exodus-rsync.conf", []byte{}, 0000)

	args := []string{"exodus-rsync", "-vvv"}
	args = append(args, ".", "some-dest:/foo/bar")

	// It should fail with this code
	if got := Main(args); got != 23 {
		t.Error("unexpected exit code", got)
	}

	// It should tell us there was a problem with config
	entry := FindEntry(logs, "can't load config")
	if entry == nil {
		t.Fatal("missing expected log message")
	}

	if !strings.Contains(fmt.Sprint(entry.Fields["error"]), "permission denied") {
		t.Fatal("error message not as expected", entry.Fields["error"])
	}
}

func TestMainNonexistentConf(t *testing.T) {
	logs := CaptureLogger(t)

	ctrl := MockController(t)

	mockRsync := rsync.NewMockInterface(ctrl)
	ext.rsync = mockRsync

	// Since no config file can be found, we expect it to run rsync and it should pass
	// through whatever arguments we're giving it.
	rawArgs := []string{"exodus-rsync", "-vvv"}
	rawArgs = append(rawArgs, ".", "some-dest:/foo/bar")
	rawArgs = append(rawArgs, "--exodus-conf", "this-file-does-not-exist.conf")
	parsedArgs := args.Parse(rawArgs, version, nil)

	// We can't actually simulate the 'rsync successful' case because exec would not
	// normally return if the process could be executed, so just force it to return
	// an error.
	rsyncError := fmt.Errorf("simulated error")

	mockRsync.EXPECT().Exec(gomock.Any(), parsedArgs).Return(rsyncError)

	got := Main(rawArgs)

	if got != 94 {
		t.Error("returned incorrect exit code", got)
	}

	// It should tell us there was a problem with config
	entry := FindEntry(logs, "setting rsyncmode to 'rsync'")
	if entry == nil {
		t.Fatal("missing expected log message")
	}
	if !strings.Contains(fmt.Sprint(entry.Fields["error"]), "no existing config file") {
		t.Fatal("error message not as expected", entry.Fields["error"])
	}
}
