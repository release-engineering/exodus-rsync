package args

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

func TestParseOk(t *testing.T) {
	tests := map[string]struct {
		input []string
		want  Config
	}{
		"trivial": {input: []string{"exodus-rsync", "some-src", "some-dest"},
			want: Config{Src: "some-src", Dest: "some-dest"}},

		"ignored args": {
			// At least all compound names should be in long-form to ensure rsync compatibility.
			input: []string{
				"exodus-rsync",
				"-arpEogDtz",
				"--copy-links",
				"--keep-dirlinks",
				"--hard-links",
				"--acls",
				"--xattrs",
				"--atimes",
				"--crtimes",
				"--omit-dir-times",
				"--rsh", "abc",
				"--delete",
				"--prune-empty-dirs",
				"--timeout", "123",
				"--stats",
				"--itemize-changes",
				"x",
				"y"},
			want: Config{Src: "x", Dest: "y",
				IgnoredConfig: IgnoredConfig{
					Archive:         true,
					Recursive:       true,
					CopyLinks:       true,
					KeepDirlinks:    true,
					HardLinks:       true,
					Perms:           true,
					Executability:   true,
					Acls:            true,
					Xattrs:          true,
					Owner:           true,
					Group:           true,
					Devices:         true,
					Specials:        true,
					DevicesSpecials: true,
					Times:           true,
					Atimes:          true,
					Crtimes:         true,
					OmitDirTimes:    true,
					Rsh:             "abc",
					Delete:          true,
					PruneEmptyDirs:  true,
					Timeout:         123,
					Compress:        true,
					Stats:           true,
					ItemizeChanges:  true,
				}}},

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

		"links": {
			input: []string{
				"exodus-rsync",
				"-l",
				"x",
				"y"},
			want: Config{Links: true, Src: "x", Dest: "y"}},

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
				"--filter", "+ **/hi/**",
				"--filter=-/_*",
				"x",
				"y"},
			want: Config{Src: "x", Dest: "y", Filter: []string{"+ **/hi/**", "-/_*"}}},
		"with publish": {
			input: []string{
				"exodus-rsync",
				"--exodus-publish",
				"3e0a4539-be4a-437e-a45f-6d72f7192f17",
				"x",
				"y",
			},
			want: Config{Src: "x", Dest: "y", ExodusConfig: ExodusConfig{Publish: "3e0a4539-be4a-437e-a45f-6d72f7192f17"}},
		},
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
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{Src: tc.src, Dest: tc.dest, Relative: tc.rel}
			if got := c.DestPath(); got != tc.want {
				t.Errorf("Config.DestPath() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestStringMapDecodeError(t *testing.T) {
	err := argStringMapper{}.Decode(
		&kong.DecodeContext{Value: &kong.Value{}, Scan: &kong.Scanner{}},
		reflect.Value{},
	)

	// Scan holds no tokens; no flag or flag value to decode.
	if err.Error() != "flag : missing value" {
		t.Fatalf("didn't get expected error, got %s", err.Error())
	}
}

func TestConfigValidationErrors(t *testing.T) {
	var exitcode int

	config := Parse([]string{"exodus-rsync", "some-src", strings.Repeat("some-dest", 250), "--exodus-publish", "zombies"}, "", func(code int) {
		exitcode = code
	})
	if exitcode != 0 {
		t.Fatalf("unexpected parsing error: %d\nconfig: %v", exitcode, config)
	}

	expected := []string{
		"validation error(s):",
		"Key: 'Config.Dest' Error:Field validation for 'Dest' failed on the 'max' tag",
		"Key: 'Config.ExodusConfig.Publish' Error:Field validation for 'Publish' failed on the 'uuid' tag",
	}
	err := config.ValidateConfig()
	if err == nil {
		t.Fatalf("didn't raise a validation error")
	}
	for _, str := range expected {
		if !strings.Contains(err.Error(), str) {
			t.Fatalf("didn't get expected error, got %s", err.Error())
		}
	}
}
