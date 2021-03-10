package rsync

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func addTestBinPath(t *testing.T) {
	oldPath := os.Getenv("PATH")
	newPath := "../../test/bin:" + oldPath

	t.Cleanup(func() {
		os.Setenv("PATH", oldPath)
	})
	err := os.Setenv("PATH", newPath)
	if err != nil {
		t.Fatalf("could not set PATH, err = %v", err)
	}
}

func TestExec(t *testing.T) {
	ctrl := gomock.NewController(t)
	conf := conf.NewMockConfig(ctrl)

	addTestBinPath(t)

	tests := []struct {
		name         string
		args         args.Config
		expectedArgv []string
	}{
		{"basic",
			args.Config{
				Src:  "some-src",
				Dest: "some-dest",
			},
			[]string{"../../test/bin/rsync", "some-src", "some-dest"},
		},

		{"all args",
			args.Config{
				Src:     "src",
				Dest:    "dest",
				Verbose: 3,
				IgnoredConfig: args.IgnoredConfig{
					Recursive:      true,
					Times:          true,
					Delete:         true,
					KeepDirlinks:   true,
					OmitDirTimes:   true,
					Compress:       true,
					ItemizeChanges: true,
					Rsh:            "some-rsh",
					CopyLinks:      true,
					Stats:          true,
					Timeout:        1234,
					Archive:        true,
				},
				IgnoreExisting: true,
				Filter:         "some-filter",
			},
			[]string{
				"../../test/bin/rsync",
				"--recursive", "--times", "--delete", "--keep-dirlinks", "--omit-dir-times",
				"--compress", "--itemize-changes", "--rsh", "some-rsh", "--copy-links",
				"--stats", "--timeout", "1234", "--archive", "-vvv", "--ignore-existing",
				"--filter", "some-filter", "src", "dest"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotArgv0 string
			var gotArgv []string

			oldExec := ext.exec
			ext.exec = func(argv0 string, argv, _ []string) error {
				gotArgv0 = argv0
				gotArgv = argv
				return fmt.Errorf("simulated error")
			}
			defer func() { ext.exec = oldExec }()

			ctx := context.Background()
			ctx = log.NewContext(ctx, log.Package.NewLogger(tt.args))

			err := Package.Exec(ctx, conf, tt.args)
			if err.Error() != "simulated error" {
				t.Error("error not propagated from exec, got =", err)
			}

			if gotArgv0 != "../../test/bin/rsync" {
				t.Error("invoked unexpected rsync command", gotArgv0)
			}

			if !reflect.DeepEqual(gotArgv, tt.expectedArgv) {
				t.Error("rsync invoked with wrong arguments", gotArgv)
			}
		})
	}
}
