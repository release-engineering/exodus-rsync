package rsync

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

// If an rsync command can't be located, /usr/bin/rsync is used
// as fallback.
func TestCommandFallback(t *testing.T) {
	// No PATH at all means no rsync in PATH
	setPath(t, "")

	ctx := log.NewContext(context.Background(), log.Package.NewLogger(args.Config{}))

	cmd, err := Package.Command(ctx, []string{})
	if err != nil {
		t.Fatal(err)
	}

	if cmd.Path != "/usr/bin/rsync" {
		t.Errorf("command returned unexpected path %v", cmd.Path)
	}
}

func TestCommandAvoidSelf(t *testing.T) {
	oldArg0 := os.Args[0]
	defer func() {
		os.Args[0] = oldArg0
	}()

	tempDir := t.TempDir()

	// Simulate that we are installed as 'rsync' in the tempdir
	self := tempDir + "/rsync"
	err := os.WriteFile(self, []byte("#!/bin/sh\necho hi\n"), 0755)
	if err != nil {
		t.Fatal(err)
	}
	os.Args[0] = self

	// Add dir containing self to path *and also* test/bin, which contains
	// the "real" rsync in the context of this test
	setPath(t, tempDir+":"+testBinPath(t))

	// Sanity check: naive lookup of rsync should find self
	foundRsync, err := exec.LookPath("rsync")
	if err != nil {
		t.Fatal(err)
	}
	if foundRsync != self {
		t.Fatalf("sanity check of test setup failed, lookup of rsync returned %v", foundRsync)
	}

	ctx := log.NewContext(context.Background(), log.Package.NewLogger(args.Config{}))

	cmd, err := Package.Command(ctx, []string{})
	if err != nil {
		t.Fatal(err)
	}

	// Rather than looking up ourselves as a plain LookPath did, it should be smart
	// enough to remove self from path and find the "real" rsync
	if cmd.Path != testBinPath(t)+"/rsync" {
		t.Errorf("command returned unexpected path %v", cmd.Path)
	}
}

func TestCommandFindsSelf(t *testing.T) {
	oldArg0 := os.Args[0]
	defer func() {
		os.Args[0] = oldArg0
	}()

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
	setPath(t, tempDir1+":"+tempDir2)

	// Command will fail to find rsync and return an error.
	ctx := log.NewContext(context.Background(), log.Package.NewLogger(args.Config{}))
	cmd, err := Package.Command(ctx, []string{})
	if cmd != nil {
		t.Fatalf("expected no command, got %v", cmd)
	}
	if err.Error() != "an 'rsync' command is required but could not be found" {
		t.Fatalf("did not get expected error, got %v", err)
	}
}

func TestPanic2ErrWithError(t *testing.T) {
	var err error

	func() {
		defer panic2err(&err)
		panic(fmt.Errorf("simulated error"))
	}()

	// It should put the error into the pointer.
	if err.Error() != "simulated error" {
		t.Errorf("did not get expected error, got %v", err)
	}
}

func TestPanic2ErrWithOther(t *testing.T) {
	var err error
	var recovered interface{}

	func() {
		defer func() {
			recovered = recover()
		}()

		func() {
			defer panic2err(&err)
			panic("whatever")
		}()
	}()

	// It should not put anything in err, since it wasn't an error.
	if err != nil {
		t.Errorf("unexpectedly got error %v", err)
	}

	// It should propagate whatever was originally panicked.
	if recovered.(string) != "whatever" {
		t.Errorf("unexpected value from recover: %v", recovered)
	}
}
