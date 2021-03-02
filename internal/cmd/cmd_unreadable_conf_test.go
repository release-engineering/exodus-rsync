package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

type confFileConfigurator func(*[]string)

func setupNonexistentFile(args *[]string) {
	*args = append(*args, "--exodus-conf", "this-file-does-not-exist.conf")
}

func setupUnreadableFile(_ *[]string) {
	// Just make a config file using the name it'll look for by default, but
	// without read permissions.
	os.WriteFile("exodus-rsync.conf", []byte{}, 0000)
}

func TestMainUnreadableConf(t *testing.T) {
	type args struct {
		rawArgs []string
	}
	tests := []struct {
		name         string
		setupConf    confFileConfigurator
		errorMessage string
	}{
		{"unreadable file", setupUnreadableFile, "permission denied"},
		{"nonexistent file", setupNonexistentFile, "no existing config file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Chdir(t.TempDir()); err != nil {
				t.Fatal("can't enter temporary directory", err)
			}

			logs := CaptureLogger(t)

			args := []string{"exodus-rsync", "-vvv"}
			tt.setupConf(&args)
			args = append(args, "src", "dest")

			// It should fail with this code
			if got := Main(args); got != 23 {
				t.Error("unexpected exit code", got)
			}

			// It should tell us there was a problem with config
			entry := FindEntry(logs, "can't load config")
			if entry == nil {
				t.Fatal("missing expected log message")
			}

			if !strings.Contains(fmt.Sprint(entry.Fields["error"]), tt.errorMessage) {
				t.Fatal("error message not as expected", entry.Fields["error"])
			}
		})
	}
}
