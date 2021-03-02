package rsync

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/log"
)

func TestExec(t *testing.T) {
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
			[]string{"some-src", "some-dest"},
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

			conf := conf.Config{}

			ctx := context.Background()
			ctx = log.NewContext(ctx, log.Package.NewLogger(tt.args))

			err := Package.Exec(ctx, conf, tt.args)
			if err.Error() != "simulated error" {
				t.Error("error not propagated from exec, got =", err)
			}

			// TODO: update this when rsync lookup behavior is implemented
			if gotArgv0 != "/usr/bin/rsync" {
				t.Error("invoked unexpected rsync command", gotArgv0)
			}

			if !reflect.DeepEqual(gotArgv, tt.expectedArgv) {
				t.Error("rsync invoked with wrong arguments", gotArgv)
			}
		})
	}
}
