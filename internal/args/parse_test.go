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
				"-rDmz",
				"--delete",
				"x",
				"y"},
			want: Config{Src: "x", Dest: "y",
				// -D should enable DevicesSpecials which enables Devices and Specials
				IgnoredConfig: IgnoredConfig{Recursive: true, Devices: true, Specials: true, DevicesSpecials: true, PruneEmptyDirs: true, Compress: true, Delete: true}}},

		"verbose": {
			input: []string{
				"exodus-rsync",
				"-vv", "--verbose",
				"x",
				"y"},
			want: Config{Verbose: 3, Src: "x", Dest: "y"}},

		"relative": {
			input: []string{
				"exodus-rsync",
				"--relative",
				"x",
				"y"},
			want: Config{Relative: true, Src: "x", Dest: "y"}},

		"exclude": {
			input: []string{
				"exodus-rsync",
				"--exclude",
				".*",
				"--exclude",
				"*.conf",
				"x",
				"y"},
			want: Config{Exclude: []string{".*", "*.conf"}, Src: "x", Dest: "y"}},

		"files-from": {
			input: []string{
				"exodus-rsync",
				"--files-from",
				"sources.txt",
				"x",
				"y"},
			want: Config{FilesFrom: "sources.txt", Src: "x", Dest: "y"}},

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
		src  string
		dest string
		rel  bool
		want string
	}{
		{"no : in dest", ".", "some-dest", true, ""},
		{": in dest", ".", "user@somehost:/some/rsync/path", false, "/some/rsync/path"},
		{"relative dest", "/some/path", "user@somehost:/rsync/", true, "/rsync/some/path"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := Config{Src: tt.src, Dest: tt.dest, Relative: tt.rel}
			if got := c.DestPath(); got != tt.want {
				t.Errorf("Config.DestPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
