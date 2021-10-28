package cmd

import (
	"os"
	"path"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/diag"
	"github.com/release-engineering/exodus-rsync/internal/gw"
)

func TestMainDiag(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockDiag := diag.NewMockInterface(ctrl)
	ext.diag = mockDiag

	// It should invoke the diagnostic mode.
	mockDiag.EXPECT().Run(gomock.Any(), gomock.Any(), gomock.Any())

	// Diagnostic mode should go ahead with the rest of the publish afterward,
	// so we also expect a gw client to be used.
	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewDryRunClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	args := []string{
		"rsync",
		"--dry-run",
		"--exodus-diag",
		srcPath + "/",
		"exodus:/some/target",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}
}
