package conf

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/adrg/xdg"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"gopkg.in/yaml.v3"
)

func candidatePaths() []string {
	return []string{
		"exodus-rsync.conf",
		xdg.ConfigHome + "/exodus-rsync.conf",
		"/etc/exodus-rsync.conf",
	}
}

func loadFromPath(path string, args args.Config) (*globalConfig, error) {
	file, err := os.Open(path)
	if err != nil {
		return &globalConfig{}, err
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	out := &globalConfig{}
	out.args = args

	err = dec.Decode(&out)
	if err != nil {
		return &globalConfig{}, fmt.Errorf("can't parse %s: %w", path, err)
	}

	// A bit of normalization...
	for {
		if !strings.HasSuffix(out.GwURLRaw, "/") {
			break
		}
		out.GwURLRaw = strings.TrimSuffix(out.GwURLRaw, "/")
	}

	// A few vars support env var expansion for convenience
	out.GwCertRaw = os.ExpandEnv(out.GwCertRaw)
	out.GwKeyRaw = os.ExpandEnv(out.GwKeyRaw)
	out.GwURLRaw = os.ExpandEnv(out.GwURLRaw)
	out.GwEnvRaw = os.ExpandEnv(out.GwEnvRaw)

	// Fill in the Environment parent references
	prefs := map[string]bool{}
	for i := range out.EnvironmentsRaw {
		env := &out.EnvironmentsRaw[i]
		if !strings.HasPrefix(env.Prefix(), out.Strip()) {
			return nil, fmt.Errorf("cannot strip '%s' prefix from '%s'", out.Strip(), env.Prefix())
		}
		if prefs[env.Prefix()] {
			return nil, fmt.Errorf("duplicate environment definitions for '%s'", env.Prefix())
		}
		prefs[env.Prefix()] = true
		out.EnvironmentsRaw[i].parent = out

	}

	return out, nil
}

func (impl) Load(ctx context.Context, args args.Config) (GlobalConfig, error) {
	logger := log.FromContext(ctx)

	candidates := candidatePaths()
	if args.Conf != "" {
		candidates = []string{args.Conf}
	}

	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err == nil {
			logger.F("path", candidate).Debug("loading config")
			return loadFromPath(candidate, args)
		}
		logger.F("path", candidate, "error", err).Debug("config file not usable")
	}

	return nil, &MissingConfigFile{candidates: candidates}
}

// EnvironmentForDest finds and returns an Environment matching the specified rsync
// destination, or nil if no Environment matches.
func (c *globalConfig) EnvironmentForDest(ctx context.Context, dest string) EnvironmentConfig {
	logger := log.FromContext(ctx)

	for i := range c.EnvironmentsRaw {
		out := &c.EnvironmentsRaw[i]
		prefix := out.Prefix()
		if !strings.Contains(prefix, ":") {
			prefix = prefix + ":"
		}
		if strings.HasPrefix(dest, prefix) {
			return out
		}
	}

	logger.F("dest", dest).Debug("no matching environment in config")

	return nil
}
