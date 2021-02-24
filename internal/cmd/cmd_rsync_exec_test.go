package cmd

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

func MockController(t *testing.T) *gomock.Controller {
	oldExt := ext
	t.Cleanup(func() { ext = oldExt })

	return gomock.NewController(t)
}

func TestMainExecRsync(t *testing.T) {
	ctrl := MockController(t)

	mockConf := conf.NewMockInterface(ctrl)
	mockRsync := rsync.NewMockInterface(ctrl)
	ext.conf = mockConf
	ext.rsync = mockRsync

	emptyConfig := conf.Config{}

	// Make it return an empty Config - this means no environment will match.
	mockConf.EXPECT().Load(gomock.Any(), gomock.Any()).Return(emptyConfig, nil)

	// Since no environment matches, we expect it to run rsync and it should pass
	// through whatever arguments we're giving it.
	args := args.Config{}
	args.Recursive = true
	args.Timeout = 1234
	args.Src = "."
	args.Dest = "some-dest:/foo/bar"

	// We can't actually simulate the 'rsync successful' case because exec would not
	// normally return if the process could be executed, so just force it to return
	// an error.
	rsyncError := fmt.Errorf("simulated error")

	mockRsync.EXPECT().Exec(gomock.Any(), emptyConfig, args).Return(rsyncError)

	got := Main([]string{
		"exodus-rsync", "--recursive", "--timeout", "1234",
		".", "some-dest:/foo/bar",
	})

	if got != 94 {
		t.Error("returned incorrect exit code", got)
	}
}
