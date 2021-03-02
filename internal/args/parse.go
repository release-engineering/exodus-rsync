package args

import (
	"fmt"
	"os"
	"strings"

	"github.com/alecthomas/kong"
)

type filterArgument string

func (f filterArgument) Validate() error {
	if f == "+ */" {
		// This is OK as it means nothing is filtered
		return nil
	}
	// Anything else is not supported
	return fmt.Errorf("unsupported filter '%s'", f)
}

// IgnoredConfig defines arguments which can be accepted for compatibility with rsync,
// but are ignored by exodus-rsync.
type IgnoredConfig struct {
	Recursive      bool `short:"r"`
	Times          bool `short:"t"`
	Delete         bool
	KeepDirlinks   bool   `short:"K"`
	OmitDirTimes   bool   `short:"O"`
	Compress       bool   `short:"z"`
	ItemizeChanges bool   `short:"i"`
	Rsh            string `short:"e"`
	CopyLinks      bool   `short:"L"`
	Stats          bool
	Timeout        int
	Archive        bool `short:"a"`
}

// ExodusConfig defines arguments which are specific to exodus-rsync and not supported
// by rsync. To avoid clashes with rsync, all of these are prefixed with "--exodus"
// and there are no short flags.
type ExodusConfig struct {
	Conf string `help:"Force usage of this configuration file."`
}

// Config contains the subset of arguments which are returned by the parser and
// can affect the behavior of exodus-rsync.
type Config struct {
	// Adjust verbosity.
	Verbose int `short:"v" type:"counter" help:"Increase verbosity; can be provided multiple times."`

	// TODO: fail if publish contains any files.
	// Note: the story with this is that some tools use an approach like this to implement
	// a "remote mkdir":
	// mkdir -p root/empty-dir
	// rsync --ignore-existing root host:/dest/some/dir/which/should/be/created
	// Since directories don't actually exist in exodus, that should be a no-op which
	// successfully does nothing.  But any *other* attempted usage of --ignore-existing
	// should be an error (we don't have the capability to skip content which is
	// already present).
	IgnoreExisting bool `hidden:"1"`

	// TODO: --dry-run, -n

	Src  string `arg:"1" placeholder:"SRC" help:"Local path to a file or directory for sync"`
	Dest string `arg:"1" placeholder:"[USER@]HOST:DEST" help:"Remote destination for sync"`

	// This should be parsed but not exposed
	Filter filterArgument `short:"f" hidden:"1"`

	IgnoredConfig `embed:"1" group:"ignored"`
	ExodusConfig  `embed:"1" prefix:"exodus-"`
}

// DestPath returns only the path portion of the destination argument passed
// on the command-line.
// For example, if invoked with user@host.example.com:/some/dir,
// this will return "/some/dir".
func (c *Config) DestPath() string {
	if strings.Contains(c.Dest, ":") {
		return strings.SplitN(c.Dest, ":", 2)[1]
	}
	return ""
}

// Parse will parse provided command-line arguments and either return
// a valid Config object, or call the exit function with a non-zero
// exit code.
func Parse(args []string, exit func(int)) Config {
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
		kong.ExplicitGroups([]kong.Group{

			{Key: "ignored",
				Title: "Ignored flags:",
				Description: "The following arguments are accepted for compatibility with rsync, " +
					"but do not affect the behavior of exodus-rsync.",
			},
		}),
	)

	return out
}
