package diag

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	gomock "github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func mockConfig(ctrl *gomock.Controller) conf.Config {
	out := conf.NewMockEnvironmentConfig(ctrl)
	e := out.EXPECT()

	e.GwCert().Return("test-cert").AnyTimes()
	e.GwKey().Return("test-key").AnyTimes()
	e.GwURL().Return("test-url").AnyTimes()
	e.GwEnv().Return("test-env").AnyTimes()
	e.GwPollInterval().Return(123).AnyTimes()
	e.GwBatchSize().Return(234).AnyTimes()
	e.GwMaxAttempts().Return(345).AnyTimes()
	e.GwMaxBackoff().Return(456).AnyTimes()
	e.RsyncMode().Return("mixed").AnyTimes()
	e.LogLevel().Return("debug").AnyTimes()
	e.Logger().Return("syslog").AnyTimes()
	e.Verbosity().Return(3).AnyTimes()
	e.Prefix().Return("test-prefix").AnyTimes()
	e.Strip().Return("").AnyTimes()
	e.UploadThreads().Return(4).AnyTimes()

	return out
}

func TestDiagRun(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	srcPath := path.Clean(wd + "/../../test/data/srctrees")
	filesFromPath := path.Clean(wd + "/../../test/data/source-list.txt")

	ctrl := MockController(t)

	conf := mockConfig(ctrl)
	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	// Default mock state is to fail creating a client.
	newClient := mockGw.EXPECT().NewDryRunClient(gomock.Any(), gomock.Any())
	newClient.Return(nil, fmt.Errorf("simulated error")).AnyTimes()

	args := args.Config{}

	args.Src = srcPath
	args.Dest = "whatever-dest"

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args))

	// Invoke the diagnostic mode with various different setups.
	//
	// The purpose is only to verify that every code path in diagnostic mode can
	// be reached and no crashes occur. The function is expected to generate
	// developer-oriented logs of an unspecified format, we don't try to verify
	// the content of these.

	// Minimal config
	Package.Run(ctx, conf, args)

	// Pointing at a files-from which can be read
	args.FilesFrom = filesFromPath
	Package.Run(ctx, conf, args)

	// Pointing at a files-from which can't be read
	args.FilesFrom = "/some/non-existent/file"
	Package.Run(ctx, conf, args)

	// Next tests will use a GW client
	mockClient := gw.NewMockClient(ctrl)
	whoAmiI := mockClient.EXPECT().WhoAmI(gomock.Any()).AnyTimes()
	newClient.Return(mockClient, nil)

	// Client can be created but whoami fails.
	whoAmiI.Return(nil, fmt.Errorf("whoami error"))
	Package.Run(ctx, conf, args)

	// Client can be created and whoami succeeds.
	whoAmiI.Return(map[string]interface{}{"foo": "bar"}, nil)
	Package.Run(ctx, conf, args)
}

func TestCommandErr(t *testing.T) {
	oldArg0 := os.Args[0]
	defer func() {
		os.Args[0] = oldArg0
	}()

	ctrl := MockController(t)
	conf := mockConfig(ctrl)
	args := args.Config{Src: ".", Dest: "whatever-dest"}
	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args))

	// Simulate that we are installed as 'rsync' in tempDir1.
	tempDir1 := t.TempDir()
	self := tempDir1 + "/rsync"
	err := os.WriteFile(self, []byte("#!/bin/sh\necho hi\n"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	os.Args[0] = self

	// Simulate that we are symlinked where "real" rsync is expected.
	tempDir2 := t.TempDir()
	err = os.Symlink(self, tempDir2+"/rsync")
	if err != nil {
		t.Fatal(err)
	}

	// Add to PATH dir containing self and dir in which "real" rsync is
	// expected.
	t.Cleanup(func() {
		os.Setenv("PATH", os.Getenv("PATH"))
	})
	err = os.Setenv("PATH", tempDir1+":"+tempDir2)
	if err != nil {
		t.Fatalf("could not set PATH, err = %v", err)
	}

	// logCommand can run when errors are returned.
	logCommand(ctx, conf, args)
}
