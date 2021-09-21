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

	err := Walk(ctx, ".", []string{}, []string{}, []string{}, handler)

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

	err := Walk(ctx, ".", []string{}, []string{}, []string{}, handler)

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

	err := Walk(ctx, ".", []string{}, []string{}, []string{}, handler)

	// It should have returned the error from handler
	if err.Error() != "simulated error" {
		t.Errorf("returned unexpected error %v", err)
	}
}

func TestWalkExcludeMatchError(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)

	handler := func(item SyncItem) error {
		return nil
	}

	err := Walk(ctx, ".", []string{"a(b"}, []string{}, []string{}, handler)

	// It should have caused a regexp error
	msg := "could not process --exclude `a(b`: error parsing regexp: missing closing ): `a(b`"
	if err.Error() != msg {
		t.Errorf("unexpected success")
	}
}

func TestWalkIncludeMatchError(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)

	handler := func(item SyncItem) error {
		return nil
	}

	err := Walk(ctx, ".", []string{"*"}, []string{"a(b"}, []string{}, handler)

	// It should have caused a regexp error
	msg := "could not process --include `a(b`: error parsing regexp: missing closing ): `a(b`"
	if err.Error() != msg {
		t.Errorf("unexpected success")
	}
}

func TestWalkLinksMatchPattern(t *testing.T) {
	tests := []struct {
		path    string
		pattern string
	}{
		{"/foo/bar/baz", "."},
		{"/foo/bar", "/foo"},
		{"foo/bar", "bar/"},
		{"foo/bar", `foo/***`},
		{"test.txt", `.txt`},
		{"test?.txt", `\?.txt`},
		{"foo/bars", `bar?`},
		{"foo/bar/baz/buzz/bats.oog", `foo/**/bats.oog`},
		{"foo/4/baz", `foo/[0-9]/baz`},
		{"foo/bar/baz", `foo/[a-z]+/baz`},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			matched, _ := matchPattern(tt.path, tt.pattern)
			if matched == false {
				t.Errorf("'%s' did not match, '%s", tt.pattern, tt.path)
			}
		})
	}
}

func TestWalkLinksFilterPathAll(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)

	err := filterPath(log.FromContext(ctx), "/some/dir", []string{"*"}, []string{}, true)
	if err != nil && err.Error() != "filtered '/some/dir'" {
		t.Errorf("failed to filter '/some/dir' for exclude pattern `*`")
	}
}

func TestWalkLinksFilterPathInclude(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)

	err := filterPath(log.FromContext(ctx), "/some/dir", []string{"*"}, []string{"*/", "**/dir"}, true)
	if err != nil {
		t.Errorf("unexpected error `%s`", err)
	}
}
