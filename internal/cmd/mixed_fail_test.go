package cmd

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/gw"
)

func TestExodusFailsFirst(t *testing.T) {
	ctrl := MockController(t)
	cfg := conf.NewMockConfig(ctrl)

	// Force exodus publish to fail by setting up broken cert/key path.
	cfg.EXPECT().GwCert().Return("/not/exist/cert")
	cfg.EXPECT().GwKey().Return("/not/exist/key")

	// Force rsync to succeed.
	rsync := &fakeRsync{delegate: ext.rsync}
	rsync.prefix = []string{"/bin/sh", "-c", "sleep 5; echo done", "--"}
	ext.rsync = rsync

	logs := CaptureLogger(t)
	ctx := testContext()

	out := mixedMain(ctx, cfg, args.Config{})

	if out != 101 {
		t.Errorf("got unexpected exit code %v", out)
	}

	// It should tell us this
	if FindEntry(logs, "Cancelling rsync due to errors in exodus publish...") == nil {
		t.Error("missing rsync cancel log")
	}

	// Very last message should explain that it was the exodus publish which failed
	last := logs.Entries[len(logs.Entries)-1]
	if last.Message != "Publish via exodus-gw failed" {
		t.Errorf("unexpected final message: %v", last.Message)
	}

}

func TestExodusFailsLater(t *testing.T) {
	ctrl := MockController(t)
	cfg := conf.NewMockConfig(ctrl)

	// Force exodus publish to fail by setting up broken cert/key path,
	// and also make it a little slower than rsync.
	cfg.EXPECT().GwCert().DoAndReturn(func() string {
		time.Sleep(time.Millisecond * 500)
		return "/not/exist/cert"
	})

	cfg.EXPECT().GwKey().Return("/not/exist/key")

	// Force rsync to succeed.
	rsync := &fakeRsync{delegate: ext.rsync}
	rsync.prefix = []string{"echo"}
	ext.rsync = rsync

	logs := CaptureLogger(t)
	ctx := testContext()

	out := mixedMain(ctx, cfg, args.Config{})

	if out != 101 {
		t.Errorf("got unexpected exit code %v", out)
	}

	// This time it should NOT tell us that rsync was cancelled, because it was already
	// completed by the time the exodus publish failed
	if FindEntry(logs, "Cancelling rsync due to errors in exodus publish...") != nil {
		t.Error("got unexpected rsync cancel log")
	}
}

func TestRsyncFailsFirst(t *testing.T) {
	ctrl := MockController(t)
	cfg := conf.NewMockConfig(ctrl)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	logs := CaptureLogger(t)
	ctx := testContext()

	mockGw.EXPECT().NewClient(gomock.Any()).Do(func(_ ...interface{}) {
		// Make creating a new client slow to be sure that rsync fails first.
		time.Sleep(time.Millisecond * 500)
	})

	// Force rsync to fail.
	rsync := &fakeRsync{delegate: ext.rsync}
	rsync.prefix = []string{"false"}
	ext.rsync = rsync

	out := mixedMain(ctx, cfg, args.Config{})

	if out != 130 {
		t.Errorf("got unexpected exit code %v", out)
	}

	// It should tell us this
	if FindEntry(logs, "Cancelling exodus publish due to errors in rsync...") == nil {
		t.Error("missing exodus cancel log")
	}

	// Very last message should explain that it was the rsync publish which failed
	last := logs.Entries[len(logs.Entries)-1]
	if last.Message != "Publish via rsync failed" {
		t.Errorf("unexpected final message: %v", last.Message)
	}

}
