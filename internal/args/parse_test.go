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
				"-D",
				"x",
				"y"},
			want: Config{Src: "x", Dest: "y",
				// -D should enable DevicesSpecials which enables Devices and Specials
				IgnoredConfig: IgnoredConfig{Recursive: true, Devices: true, Specials: true, DevicesSpecials: true, Delete: true, Compress: true}}},

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
			got := Parse(tc.input, "", nil)
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
			Parse(tc.input, "", func(code int) {
				exitcode = code
			})

			if exitcode == 0 {
				t.Fatal("should have exited with error, did not")
			}
		})
	}
}

func TestDestPath(t *testing.T) {
	tests := []struct {
		name string
		dest string
		want string
	}{
		{"no : in dest", "some-dest", ""},
		{": in dest", "user@somehost:/some/rsync/path", "/some/rsync/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Dest: tt.dest}
			if got := c.DestPath(); got != tt.want {
				t.Errorf("Config.DestPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
