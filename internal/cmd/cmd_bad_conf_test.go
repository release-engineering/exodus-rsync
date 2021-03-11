package cmd

import (
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
			SetConfig(t, tt.conf)

			args := []string{"exodus-rsync", "-v", "src", "dest"}

			if got := Main(args); got != 23 {
				t.Error("unexpected exit code", got)
			}
		})
	}
}

func TestMainBadRsyncMode(t *testing.T) {
	SetConfig(t, `
rsyncmode: invalid

environments:
- prefix: dest
`)

	args := []string{"exodus-rsync", "-vvv", "src", "dest:/quux"}

	if got := Main(args); got != 95 {
		t.Error("unexpected exit code", got)
	}
}
