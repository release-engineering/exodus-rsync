package cmd

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

func TestMainRawExecRsync(t *testing.T) {
	ctrl := MockController(t)

	mockRsync := rsync.NewMockInterface(ctrl)
	ext.rsync = mockRsync

	// Since --server is provided we should be able to run with just args, and
	// the command name, i.e., "exodus-rsync", should be trimmed from args
	// before reaching RawExec.
	rawArgs := []string{"exodus-rsync", "--server", ".", "some-dest:/foo/bar"}
	expectedArgv := []string{"--server", ".", "some-dest:/foo/bar"}

	// We can't actually simulate the 'rsync successful' case because exec would not
	// normally return if the process could be executed, so just force it to return
	// an error.
	rsyncError := fmt.Errorf("simulated error")

	mockRsync.EXPECT().RawExec(gomock.Any(), expectedArgv).Return(rsyncError)

	got := Main(rawArgs)

	if got != 94 {
		t.Error("returned incorrect exit code", got)
	}
}

func TestMainExecRsync(t *testing.T) {
	ctrl := MockController(t)

	mockConf := conf.NewMockInterface(ctrl)
	mockRsync := rsync.NewMockInterface(ctrl)
	ext.conf = mockConf
	ext.rsync = mockRsync

	emptyConfig := conf.NewMockGlobalConfig(ctrl)

	// Make it return an empty Config with no environments
	mockConf.EXPECT().Load(gomock.Any(), gomock.Any()).Return(emptyConfig, nil)
	emptyConfig.EXPECT().EnvironmentForDest(gomock.Any(), gomock.Any()).Return(nil)
	emptyConfig.EXPECT().LogLevel().AnyTimes().Return("info")
	emptyConfig.EXPECT().Logger().AnyTimes().Return("auto")

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

	mockRsync.EXPECT().Exec(gomock.Any(), args).Return(rsyncError)

	got := Main([]string{
		"exodus-rsync", "--recursive", "--timeout", "1234",
		".", "some-dest:/foo/bar",
	})

	if got != 94 {
		t.Error("returned incorrect exit code", got)
	}
}
