package cmd

import (
	"os"
	"path"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/gw"
)

func TestMainDryRunSync(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewDryRunClient(EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	args := []string{
		"rsync",
		"--dry-run",
		srcPath + "/",
		"exodus:/some/target",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

}
