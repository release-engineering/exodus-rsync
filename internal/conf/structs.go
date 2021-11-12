package conf

import (
	"fmt"
	"strings"

	"github.com/release-engineering/exodus-rsync/internal/args"
)

type sharedConfig struct {
	GwEnvRaw          string `yaml:"gwenv"`
	GwCertRaw         string `yaml:"gwcert"`
	GwKeyRaw          string `yaml:"gwkey"`
	GwURLRaw          string `yaml:"gwurl"`
	GwPollIntervalRaw int    `yaml:"gwpollinterval"`
	GwBatchSizeRaw    int    `yaml:"gwbatchsize"`
	RsyncModeRaw      string `yaml:"rsyncmode"`
	LogLevelRaw       string `yaml:"loglevel"`
	LoggerRaw         string `yaml:"logger"`
	DiagRaw           bool   `yaml:"diag"`
	StripRaw          string `yaml:"strip"`
}

type environment struct {
	sharedConfig `yaml:",inline"`
	args         args.Config `embed:"1"`

	PrefixRaw string `yaml:"prefix"`

	parent *globalConfig
}

type globalConfig struct {
	sharedConfig `yaml:",inline"`
	args         args.Config `embed:"1"`

	// Configuration for each environment.
	EnvironmentsRaw []environment `yaml:"environments"`
}

// MissingConfigFile is an error type for cases in which no config file is found.
type MissingConfigFile struct {
	// Configuration file paths that were checked.
	candidates []string
}

func (m *MissingConfigFile) Error() string {
	return fmt.Sprintf("no existing config file in: %s", strings.Join(m.candidates, ", "))
}

func (g *globalConfig) GwCert() string {
	return g.GwCertRaw
}

func (g *globalConfig) GwKey() string {
	return g.GwKeyRaw
}

func (g *globalConfig) GwURL() string {
	return g.GwURLRaw
}

func (g *globalConfig) GwEnv() string {
	return g.GwEnvRaw
}

func (g *globalConfig) GwPollInterval() int {
	return nonEmptyInt(g.GwPollIntervalRaw, 5000)
}

func (g *globalConfig) GwBatchSize() int {
	return nonEmptyInt(g.GwBatchSizeRaw, 10000)
}

func nonEmptyString(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func nonEmptyInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func (g *globalConfig) RsyncMode() string {
	return nonEmptyString(g.RsyncModeRaw, "exodus")
}

func (g *globalConfig) LogLevel() string {
	return nonEmptyString(g.LogLevelRaw, "info")
}

func (g *globalConfig) Logger() string {
	return nonEmptyString(g.LoggerRaw, "auto")
}

func (g *globalConfig) Verbosity() int {
	return g.args.Verbose
}

func (g *globalConfig) Diag() bool {
	return g.args.Diag || g.DiagRaw
}

func (g *globalConfig) Strip() string {
	return g.StripRaw
}

func (e *environment) GwCert() string {
	return nonEmptyString(e.GwCertRaw, e.parent.GwCert())
}

func (e *environment) GwKey() string {
	return nonEmptyString(e.GwKeyRaw, e.parent.GwKey())
}

func (e *environment) GwURL() string {
	return nonEmptyString(e.GwURLRaw, e.parent.GwURL())
}

func (e *environment) GwEnv() string {
	return nonEmptyString(e.GwEnvRaw, e.parent.GwEnv())
}

func (e *environment) GwPollInterval() int {
	return nonEmptyInt(e.GwPollIntervalRaw, e.parent.GwPollInterval())
}

func (e *environment) GwBatchSize() int {
	return nonEmptyInt(e.GwBatchSizeRaw, e.parent.GwBatchSize())
}

func (e *environment) RsyncMode() string {
	return nonEmptyString(e.RsyncModeRaw, e.parent.RsyncMode())
}

func (e *environment) LogLevel() string {
	return nonEmptyString(e.LogLevelRaw, e.parent.LogLevel())
}

func (e *environment) Logger() string {
	return nonEmptyString(e.LoggerRaw, e.parent.Logger())
}

func (e *environment) Verbosity() int {
	return nonEmptyInt(e.args.Verbose, e.parent.Verbosity())
}

func (e *environment) Diag() bool {
	return e.DiagRaw || e.parent.Diag()
}

func (e *environment) Prefix() string {
	return e.PrefixRaw
}

func (e *environment) Strip() string {
	// If the 'strip:' key is defined in the global config, the environment's prefix will not
	// be stripped from the destination path by default. The prefix is only stripped from the
	// destination path if the 'strip:' key is undefined.
	return nonEmptyString(nonEmptyString(e.StripRaw, e.parent.Strip()), e.PrefixRaw)
}
