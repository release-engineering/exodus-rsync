package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// Tests the main entry point to the exodus-rsync command.
//
// Note that this package is just an exiting wrapper around non-exiting
// functions under the internal packages. This test is mainly to get
// the coverage to 100%, while more meaningful tests are implemented
// elsewhere in the relevant packages.
func Test_main(t *testing.T) {
	// Main function reads from os.Args, overwrite during the test.
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	// Main function is expected to exit successfully when running with --help.
	// Exit is translated to a panic during test. Recover it to allow the test
	// to pass.
	defer func() {
		why := fmt.Sprint(recover())
		// This is the only valid reason to panic.
		if !strings.Contains(why, "os.Exit(0)") {
			t.Fatal(why)
		}
	}()

	// Main should exit successfully when invoked with --help.
	os.Args = []string{"exodus-rsync", "--help"}
	main()
}
