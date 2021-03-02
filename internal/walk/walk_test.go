package walk

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/apex/log/handlers/cli"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

// Walk with cancelled context will immediately return the cancellation error.
func TestWalkEarlyCancel(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)
	ctx, cancelFn := context.WithCancel(ctx)

	// Cancel it immediately
	cancelFn()

	// Context should now be Done
	// (Sanity check that context behaves as we expect)
	if ctx.Err() == nil {
		t.Fatal("cancelling context did not set error")
	}

	handler := func(item SyncItem) error {
		// We shouldn't ever invoke the handler if we were already cancelled
		t.Error("handler called unexpectedly")
		return nil
	}

	err := Walk(ctx, ".", handler)

	// It should have returned the cancelled error
	if err != ctx.Err() {
		t.Errorf("Did not return expected error, wanted = %v, got = %v", ctx.Err(), err)
	}
}

func TestWalkCancelInProgress(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)
	ctx, cancelFn := context.WithCancel(ctx)

	handler := func(item SyncItem) error {
		// We expect to be called only once, before we've cancelled
		if ctx.Err() == nil {
			cancelFn()
		} else {
			// We shouldn't ever invoke the handler if we were already cancelled
			t.Error("handler called unexpectedly after cancel")
		}
		return nil
	}

	err := Walk(ctx, ".", handler)

	// It should have returned the cancelled error
	if err != ctx.Err() {
		t.Errorf("Did not return expected error, wanted = %v, got = %v", ctx.Err(), err)
	}
}

func TestWalkHandlerError(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)

	handler := func(item SyncItem) error {
		return fmt.Errorf("simulated error")
	}

	err := Walk(ctx, ".", handler)

	// It should have returned the error from handler
	if err.Error() != "simulated error" {
		t.Errorf("returned unexpected error %v", err)
	}
}
