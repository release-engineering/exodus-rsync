package cmd

import (
	"bufio"
	"context"
	"io"
	"os/exec"
	"sync"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

// Mixed publish mode, publishing both via exodus and rsync.
// If either one fails, it kills/cancels the other.
func mixedMain(ctx context.Context, cfg conf.Config, args args.Config) int {
	logger := log.FromContext(ctx)

	ctx, cancelFn := context.WithCancel(ctx)

	wg := sync.WaitGroup{}
	wg.Add(2)

	defer wg.Wait()
	defer cancelFn()

	// If either one of the publishes fails, we call this function to bail out.
	//
	// The main point of this is to ensure that the most relevant log message appears
	// last. If we just let cancel & return happen "naturally", then e.g. if rsync
	// fails, the last few error messages will be about the cancellation of exodus publish
	// and a non-expert reader will probably wrongly conclude that *this* was the problem
	// and miss the error message relating to rsync.
	bailOut := func(cancelMessage, errorMessage string, exitCode int) int {
		logger.Warn(cancelMessage)

		// If one of rsync/exodus is still in progress, cancel it.
		cancelFn()

		// Wait for both to complete/fail.
		wg.Wait()

		// Then log the message.
		logger.Error(errorMessage)

		return exitCode
	}

	var lastCode *chan int
	rsyncCode := make(chan int, 1)
	exodusCode := make(chan int, 1)
	rsyncCmd := ext.rsync.Command(ctx, rsync.Arguments(ctx, args))

	// Let rsync & exodus publishes run in their own goroutines.
	go func() {
		defer wg.Done()
		exodusCode <- exodusMain(ctx, cfg, args)
	}()

	go func() {
		defer wg.Done()
		rsyncCode <- doRsyncCommand(ctx, rsyncCmd)
	}()

	select {
	case code := <-exodusCode:
		if code != 0 {
			return bailOut(
				"Cancelling rsync due to errors in exodus publish...",
				"Publish via exodus-gw failed", code)
		}
		logger.Info("Finished exodus publish, waiting on rsync...")
		lastCode = &rsyncCode
	case code := <-rsyncCode:
		if code != 0 {
			return bailOut(
				"Cancelling exodus publish due to errors in rsync...",
				"Publish via rsync failed", code)
		}
		logger.Info("Finished rsync publish, waiting on exodus...")
		lastCode = &exodusCode
	}

	return <-*lastCode
}

func doRsyncCommand(ctx context.Context, cmd *exec.Cmd) int {
	logger := log.FromContext(ctx)

	var outPipe, errPipe io.ReadCloser

	outPipe, err := cmd.StdoutPipe()
	if err == nil {
		errPipe, err = cmd.StderrPipe()
	}
	if err != nil {
		logger.F("error", err).Error("Can't connect pipes to rsync")
		return 39
	}

	outScanner := bufio.NewScanner(outPipe)
	errScanner := bufio.NewScanner(errPipe)

	err = cmd.Start()
	if err != nil {
		logger.F("error", err).Error("Failed to run rsync")
		return 25
	}

	pid := cmd.Process.Pid

	wg := sync.WaitGroup{}
	wg.Add(2)

	entry := logger.F("rsync", pid)
	type logFunc func(string)

	piper := func(scanner *bufio.Scanner, log logFunc) {
		defer wg.Done()
		for {
			if !scanner.Scan() {
				return
			}
			log(scanner.Text())
		}
	}

	go piper(outScanner, entry.Info)
	go piper(errScanner, entry.Warn)
	wg.Wait()

	err = cmd.Wait()
	if err != nil {
		logger.F("error", err).Error("rsync failed")
		return 130
	}

	return 0
}
