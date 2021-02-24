package args

import (
	"reflect"
	"testing"
)

func TestParseOk(t *testing.T) {
	tests := map[string]struct {
		input []string
		want  Config
	}{
		"trivial": {input: []string{"exodus-rsync", "some-src", "some-dest"},
			want: Config{Src: "some-src", Dest: "some-dest"}},

		"ignored args": {
			input: []string{
				"exodus-rsync",
				"--recursive",
				"-z",
				"--delete",
				"x",
				"y"},
			want: Config{Src: "x", Dest: "y",
				IgnoredConfig: IgnoredConfig{Recursive: true, Compress: true, Delete: true}}},

		"verbose": {
			input: []string{
				"exodus-rsync",
				"-vv", "--verbose",
				"x",
				"y"},
			want: Config{Verbose: 3, Src: "x", Dest: "y"}},

		"tolerable filter": {
			input: []string{
				"exodus-rsync",
				"--filter",
				"+ */",
				"x",
				"y"},
			want: Config{Src: "x", Dest: "y", Filter: "+ */"}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := Parse(tc.input, nil)
			if !reflect.DeepEqual(tc.want, got) {
				t.Fatalf("expected: %v, got: %v", tc.want, got)
			}
		})
	}
}

func TestParseErrors(t *testing.T) {
	tests := map[string]struct {
		input []string
	}{
		"missing src dest": {[]string{"exodus-rsync"}},

		"bad filter": {[]string{"exodus-rsync", "--filter", "quux", "x", "y"}},
	}

	for name, tc := range tests {
		exitcode := 0

		t.Run(name, func(t *testing.T) {
			Parse(tc.input, func(code int) {
				exitcode = code
			})

			if exitcode == 0 {
				t.Fatal("should have exited with error, did not")
			}
		})
	}
}
