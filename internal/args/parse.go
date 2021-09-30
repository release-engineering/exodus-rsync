package args

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"strings"

	"github.com/alecthomas/kong"
)

const docsURL = "https://github.com/release-engineering/exodus-rsync"

type filterArguments []string

func (f filterArguments) Validate() error {
	validRules := []string{"+", "-"}
	validMods := []string{"", "/"}
	validSeps := []string{" ", "_"}
validation:
	for _, arg := range f {
		for _, rule := range validRules {
			for _, mod := range validMods {
				for _, sep := range validSeps {
					prefix := rule + mod + sep
					if strings.HasPrefix(arg, prefix) && strings.TrimLeft(arg, prefix) != "" {
						continue validation
					}
				}
			}
		}
		// Anything else is not supported
		return fmt.Errorf("unsupported filter '%s'", arg)
	}
	return nil
}

// IgnoredConfig defines arguments which can be accepted for compatibility with rsync,
// but are ignored by exodus-rsync.
type IgnoredConfig struct {
	Archive         bool `short:"a"`
	Recursive       bool `short:"r"`
	Links           bool `short:"l"`
	CopyLinks       bool `short:"L"`
	KeepDirlinks    bool `short:"K"`
	HardLinks       bool `short:"H"`
	Perms           bool `short:"p"`
	Executability   bool `short:"E"`
	Acls            bool `short:"A"`
	Xattrs          bool `short:"X"`
	Owner           bool `short:"o"`
	Group           bool `short:"g"`
	Devices         bool
	Specials        bool
	DevicesSpecials bool   `short:"D"`
	Times           bool   `short:"t"`
	Atimes          bool   `short:"U"`
	Crtimes         bool   `short:"N"`
	OmitDirTimes    bool   `short:"O"`
	Rsh             string `short:"e"`
	Delete          bool
	PruneEmptyDirs  bool `short:"m"`
	Timeout         int
	Compress        bool `short:"z"`
	Stats           bool
	ItemizeChanges  bool `short:"i"`
}

// ExodusConfig defines arguments which are specific to exodus-rsync and not supported
// by rsync. To avoid clashes with rsync, all of these are prefixed with "--exodus"
// and there are no short flags.
type ExodusConfig struct {
	Conf string `help:"Force usage of this configuration file."`

	Publish string `help:"ID of existing exodus-gw publish to join."`
}

// Config contains the subset of arguments which are returned by the parser and
// can affect the behavior of exodus-rsync.
type Config struct {
	// Adjust verbosity.
	Verbose int `short:"v" type:"counter" help:"Increase verbosity; can be provided multiple times."`

	// Appends the source path to the destination path,
	// e.g., /foo/bar/baz.c remote:/tmp => /tmp/foo/bar/baz.c.
	Relative bool `short:"R" help:"use relative path names"`

	DryRun bool `short:"n" help:"Perform a trial run with no changes made"`

	// Mostly ignored, but causes a failure if publish contains any files.
	// See comments where the argument is checked for the explanation why.
	IgnoreExisting bool `hidden:"1"`

	Filter    filterArguments `short:"f" placeholder:"RULE" help:"Add a file-filtering RULE"`
	Exclude   []string        `placeholder:"PATTERN" help:"Exclude files matching this pattern"`
	Include   []string        `placeholder:"PATTERN" help:"Don't exclude files matching this pattern"`
	FilesFrom string          `placeholder:"FILE" help:"Read list of source-file names from FILE"`

	Src  string `arg:"1" placeholder:"SRC" help:"Local path to a file or directory for sync"`
	Dest string `arg:"1" placeholder:"[USER@]HOST:DEST" help:"Remote destination for sync"`

	IgnoredConfig `embed:"1" group:"ignored"`
	ExodusConfig  `embed:"1" prefix:"exodus-"`
}

// processFilterArgs is a helper function that appends the appropriate patterns
// (based on the given rule) from Filter arguments onto the given slice.
func (c *Config) processFilterArgs(rule string, slice []string) []string {
	for _, arg := range c.Filter {
		if strings.HasPrefix(string(arg), rule) {
			slice = append(slice, strings.TrimLeft(arg, "+-/ _"))
		}
	}
	return slice
}

// Excluded exctracts the pattern from Filter arguments and appends it onto Exclude.
func (c *Config) Excluded() []string {
	return c.processFilterArgs("-", c.Exclude)
}

// Included exctracts the pattern from Filter arguments and appends it onto Include.
func (c *Config) Included() []string {
	return c.processFilterArgs("+", c.Include)

}

// DestPath returns only the path portion of the destination argument passed
// on the command-line.
// For example, if invoked with user@host.example.com:/some/dir,
// this will return "/some/dir".
// If relative paths are requested (-R), appends the source path to the
// destination path, e.g., /foo/bar/baz.c remote:/tmp => /tmp/foo/bar/baz.c.
func (c *Config) DestPath() string {
	if strings.Contains(c.Dest, ":") {
		dest := strings.SplitN(c.Dest, ":", 2)[1]
		if c.Relative {
			dest = path.Join(dest, c.Src)
		}
		return dest
	}
	return ""
}

type argStringMapper struct{}

// A custom string decoder for kong. We use this because the default decoder
// has "magic" behavior: it explicitly rejects arguments starting with "-"
// apparently guessing that the caller probably meant for that argument to be a flag
// rather than a value. But we know that we have actual values starting with "-",
// such as in an exclude rule set by --filter.
func (argStringMapper) Decode(ctx *kong.DecodeContext, target reflect.Value) error {
	token := ctx.Scan.Pop()
	if token.IsEOL() {
		return fmt.Errorf("flag %s: missing value", ctx.Value.Name)
	}

	value := token.Value.(string)
	target.SetString(value)
	return nil
}

// Parse will parse provided command-line arguments and either return
// a valid Config object, or call the exit function with a non-zero
// exit code.
func Parse(args []string, version string, exit func(int)) Config {
	oldArgs := os.Args
	defer func() {
		os.Args = oldArgs
	}()

	if exit == nil {
		exit = os.Exit
	}

	os.Args = args
	out := Config{}
	kong.Parse(&out,
		kong.Exit(exit),
		kong.KindMapper(reflect.String, argStringMapper{}),
		kong.Description(
			fmt.Sprintf(
				"exodus-rsync %s, an exodus-aware rsync replacement.\n\nSee also: %s",
				version, docsURL)),

		kong.ExplicitGroups([]kong.Group{

			{Key: "ignored",
				Title: "Ignored flags:",
				Description: "The following arguments are accepted for compatibility with rsync, " +
					"but do not affect the behavior of exodus-rsync.",
			},
		}),
	)

	// DevicesSpecials (-D) enables both --devices and --specials.
	if out.DevicesSpecials {
		out.Devices = true
		out.Specials = true
	}

	return out
}
