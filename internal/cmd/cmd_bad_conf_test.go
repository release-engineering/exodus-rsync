package cmd

import (
	"os"
	"testing"
)

func TestMainBadConf(t *testing.T) {
	type args struct {
		rawArgs []string
	}
	tests := []struct {
		name string
		conf string
	}{
		{"not valid yaml",
			"[this: is not valid yaml"},

		{"duplicate environments",
			`
environments:
- prefix: exodus
  gwenv: test
- prefix: exodus
  gwenv: test2
`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			confFile := dir + "/bad.conf"

			if err := os.WriteFile(confFile, []byte(tt.conf), 0644); err != nil {
				t.Fatal(err)
			}

			args := []string{"exodus-rsync", "-vvv", "--exodus-conf", confFile, "src", "dest"}

			if got := Main(args); got != 23 {
				t.Error("unexpected exit code", got)
			}
		})
	}
}
