package gw

import (
	"context"
	"strings"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func TestClientTaskErrors(t *testing.T) {
	cfg := testConfig(t)

	clientIface, err := Package.NewClient(context.Background(), cfg)
	if clientIface == nil {
		t.Errorf("failed to create client, err = %v", err)
	}

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	t.Run("can't Await without link", func(t *testing.T) {
		task := task{client: clientIface.(*client)}
		task.raw.State = "IN_PROGRESS"
		task.raw.ID = "1234"

		err := task.Await(ctx)

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "polling task 1234: task object is missing 'self'") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
	})

	t.Run("Await propagates cancel", func(t *testing.T) {
		cancelCtx, cancelFn := context.WithCancel(ctx)
		cancelFn()

		task := task{client: clientIface.(*client)}
		task.raw.State = "IN_PROGRESS"
		task.raw.ID = "1234"

		err := task.Await(cancelCtx)

		if err == nil {
			t.Error("Unexpectedly failed to return an error")
		}
		if !strings.Contains(err.Error(), "canceled") {
			t.Errorf("Did not get expected error, got: %v", err)
		}
	})

}
