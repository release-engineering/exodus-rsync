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

	err := Walk(ctx, ".", []string{}, []string{}, []string{}, false, handler)

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

	err := Walk(ctx, ".", []string{}, []string{}, []string{}, false, handler)

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

	err := Walk(ctx, ".", []string{}, []string{}, []string{}, false, handler)

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

	err := Walk(ctx, ".", []string{"a(b*"}, []string{}, []string{}, false, handler)

	// It should have caused a regexp error
	msg := "could not process --exclude `a(b*`: error parsing regexp: missing closing ): `^a(b[^/]+$`"
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

	err := Walk(ctx, ".", []string{"*"}, []string{"a(b*"}, []string{}, false, handler)

	// It should have caused a regexp error
	msg := "could not process --include `a(b*`: error parsing regexp: missing closing ): `^a(b[^/]+$`"
	if err.Error() != msg {
		t.Errorf("unexpected success")
	}
}

func TestWalkLinksMatchPattern(t *testing.T) {
	tests := []struct {
		path    string
		pattern string
		isDir   bool
		result  bool
	}{
		// Should match
		{"foo/app.c", `app.*`, false, true},
		{"foo/.some-conf", `.*`, false, true},
		{"foo/bar", `*`, false, true},
		{"/foo/bar", `/foo`, false, true},
		{"foo/bar", `bar`, false, true},
		{"foo/bar", `bar/`, true, true},
		{"foo/bar/baz", `foo/***`, false, true},
		{"test.txt", `*.txt`, false, true},
		{"foo/bars", `bar?`, false, true},
		{"foo/bar/baz.dat", `foo/*/baz.dat`, false, true},
		{"foo/bar/baz/buz/bats.oog", `foo/**/bats.oog`, false, true},
		{"foo/4/bar", `foo/[0-9]/bar`, false, true},
		{"foo/d/bar", `foo/[a-z]/bar`, false, true},
		{"foo/?/bar", `foo/\?/bar`, false, true},
		// Should not match
		{"foo/some.conf", `.*`, false, false},
		{"foo/bar/baz", `.`, false, false},
		{"foo/bar/baz", `bar/`, false, false},
		{"foo/bar/baz", `baz/`, false, false},
		{"foo/4/baz", `foo/\d/baz`, false, false},
		{"foo/bar/bar", `foo/[a-z]+/baz`, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			var errMsg string
			if tt.result {
				errMsg = "pattern `%s` did not match path, '%s'"
			} else {
				errMsg = "pattern `%s` matched path, '%s'"
			}

			matched, err := matchPattern(tt.path, tt.pattern, tt.isDir)
			if err != nil {
				t.Errorf("Unexpected error, %v", err)
			}

			if matched != tt.result {
				t.Errorf(errMsg, tt.pattern, tt.path)
			}
		})
	}
}

func TestWalkLinksFilterPathAll(t *testing.T) {
	ctx := context.Background()
	logger := log.Logger{}
	logger.Handler = cli.New(os.Stdout)

	ctx = log.NewContext(ctx, &logger)

	err := filterPath(log.FromContext(ctx), "file", []string{"*"}, []string{}, false)
	if err != nil && err.Error() != "filtered 'file'" {
		t.Errorf("failed to filter 'file' for exclude pattern `*`")
	}

	err = filterPath(log.FromContext(ctx), "some/dir", []string{"*"}, []string{}, true)
	if err != nil && err.Error() != "skip this directory" {
		t.Errorf("failed to filter 'some/dir' for exclude pattern `*`")
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
