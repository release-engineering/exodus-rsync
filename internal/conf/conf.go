package conf

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/adrg/xdg"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"gopkg.in/yaml.v3"
)

// TODO: make everything in Environment able to override Config

// Environment contains configuration relevant to a single target host/environment.
type Environment struct {
	Prefix string
	GwEnv  string

	// Global config in which this environment is contained.
	Config *Config
}

// Config contains parsed content of an exodus-rsync configuration file.
type Config struct {
	// Path to certificate used to authenticate with exodus-gw.
	GwCert string

	// Path to private key used to authenticate with exodus-gw.
	GwKey string

	// Base URL of exodus-gw service in use.
	GwURL string

	// Configuration for each environment.
	Environments []Environment
}

func candidatePaths() []string {
	return []string{
		"exodus-rsync.conf",
		xdg.ConfigHome + "/exodus-rsync.conf",
		"/etc/exodus-rsync.conf",
	}
}

func loadFromPath(path string) (Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return Config{}, fmt.Errorf("can't open %s: %w", path, err)
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	out := Config{}
	err = dec.Decode(&out)
	if err != nil {
		return Config{}, fmt.Errorf("can't parse %s: %w", path, err)
	}

	// A bit of normalization...
	for {
		if !strings.HasSuffix(out.GwURL, "/") {
			break
		}
		out.GwURL = strings.TrimSuffix(out.GwURL, "/")
	}

	// A few vars support env var expansion for convenience
	out.GwCert = os.ExpandEnv(out.GwCert)
	out.GwKey = os.ExpandEnv(out.GwKey)

	// Fill in the Environment parent references
	prefs := map[string]bool{}
	for i := range out.Environments {
		env := &out.Environments[i]
		if prefs[env.Prefix] {
			return Config{}, fmt.Errorf("duplicate environment definitions for '%s'", env.Prefix)
		}
		prefs[env.Prefix] = true
		out.Environments[i].Config = &out
	}

	return out, nil
}

// Load will load and return configuration from the most appropriate
// exodus-rsync config file.
func Load(ctx context.Context) (Config, error) {
	logger := log.FromContext(ctx)

	candidates := candidatePaths()

	for _, candidate := range candidatePaths() {
		_, err := os.Stat(candidate)
		if err == nil {
			logger.F("path", candidate).Debug("loading config")
			return loadFromPath(candidate)
		}
		logger.F("path", candidate, "error", err).Debug("config file not usable")
	}

	return Config{}, fmt.Errorf("no existing config file in: %s", strings.Join(candidates, ", "))
}

func (c *Config) EnvironmentForDest(ctx context.Context, dest string) *Environment {
	logger := log.FromContext(ctx)

	for i := range c.Environments {
		out := &c.Environments[i]
		if strings.HasPrefix(dest, out.Prefix+":") {
			return out
		}
	}

	logger.F("dest", dest).Debug("no matching environment in config")

	return nil
}
