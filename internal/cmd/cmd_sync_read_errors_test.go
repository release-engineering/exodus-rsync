package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/gw"
)

func TestMainSyncUnreadableFile(t *testing.T) {
	SetConfig(t, CONFIG)

	logs := CaptureLogger(t)

	// Make a couple of files, one of which we can't read
	os.Mkdir("src", 0755)
	os.WriteFile("src/file1", []byte("hello"), 0644)
	os.WriteFile("src/file2", []byte("can't read me"), 0000)

	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	args := []string{"rsync", "src", "exodus:/some/target"}

	got := Main(args)

	// It should fail.
	if got != 73 {
		t.Error("returned incorrect exit code", got)
	}

	// It should have told us why.
	entry := FindEntry(logs, "can't read files for sync")
	if entry == nil {
		t.Fatal("missing expected log message")
	}

	err := fmt.Sprint(entry.Fields["error"])
	if !strings.Contains(err, "checksum src/file2:") {
		t.Error("unexpected error message", err)
	}
}

func TestMainSyncUnreadableFilesFrom(t *testing.T) {
	SetConfig(t, CONFIG)

	logs := CaptureLogger(t)

	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	args := []string{"rsync", "--files-from", "/sources.txt", ".", "exodus:/some/target"}

	got := Main(args)

	// It should fail.
	if got != 73 {
		t.Error("returned incorrect exit code", got)
	}

	// It should have told us why.
	entry := FindEntry(logs, "can't read --files-from file")
	if entry == nil {
		t.Fatal("missing expected log message")
	}

	err := fmt.Sprint(entry.Fields["error"])
	if !strings.Contains(err, "no such file or directory") {
		t.Error("unexpected error message", err)
	}
}

func TestMainSyncUnreadableDir(t *testing.T) {
	SetConfig(t, CONFIG)

	logs := CaptureLogger(t)

	// Make a directory which cannot be read
	os.Mkdir("src", 0755)
	os.Mkdir("src/unreadable-dir", 0000)

	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	args := []string{"rsync", "src", "exodus:/some/target"}

	got := Main(args)

	// It should fail.
	if got != 73 {
		t.Error("returned incorrect exit code", got)
	}

	// It should have told us why.
	entry := FindEntry(logs, "can't read files for sync")
	if entry == nil {
		t.Fatal("missing expected log message")
	}

	err := fmt.Sprint(entry.Fields["error"])
	if !strings.Contains(err, "open src/unreadable-dir:") {
		t.Error("unexpected error message", err)
	}
}
