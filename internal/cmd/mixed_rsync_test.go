package cmd

import (
	"context"
	"os/exec"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func TestRsyncPipeFail(t *testing.T) {
	cmd := exec.Command("echo", "hello")

	// Get a stdout pipe, this will break doRsyncCommand because it can
	// only be done once.
	cmd.StdoutPipe()

	code := doRsyncCommand(testContext(), cmd)
	if code != 39 {
		t.Errorf("got unexpected exit code %v", code)
	}
}

func TestRsyncStartFail(t *testing.T) {
	cmd := exec.Command("/non/existent/binary")

	code := doRsyncCommand(testContext(), cmd)
	if code != 25 {
		t.Errorf("got unexpected exit code %v", code)
	}
}

func TestRsyncFails(t *testing.T) {
	cmd := exec.Command("false")

	code := doRsyncCommand(testContext(), cmd)
	if code != 130 {
		t.Errorf("got unexpected exit code %v", code)
	}
}

func TestRsyncPipes(t *testing.T) {
	cmd := exec.Command("/bin/sh", "-c", "echo out1; echo err1 1>&2; echo out2; echo err2 1>&2")

	logs := CaptureLogger(t)
	ctx := log.NewContext(context.Background(), ext.log.NewLogger(args.Config{}))

	code := doRsyncCommand(ctx, cmd)

	// It should succeed
	if code != 0 {
		t.Errorf("got unexpected exit code %v", code)
	}

	// Stdout and stderr should have gone to loggers
	if len(logs.Entries) == 0 {
		t.Fatal("Did not get any log messages")
	}

	infoText := ""
	warnText := ""

	for _, entry := range logs.Entries {
		if entry.Level == log.InfoLevel {
			infoText = infoText + entry.Message + "\n"
		}
		if entry.Level == log.WarnLevel {
			warnText = warnText + entry.Message + "\n"
		}
	}

	if infoText != "out1\nout2\n" {
		t.Errorf("Did not get expected stdout logs, got: %v", infoText)
	}
	if warnText != "err1\nerr2\n" {
		t.Errorf("Did not get expected stderr logs, got: %v", warnText)
	}

}
