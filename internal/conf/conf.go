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

// TODO: make everything in Environment able to override Config

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination mock.go -source $GOFILE

// Interface defines the public interface of this package.
type Interface interface {
	// Load will load and return configuration from the most appropriate
	// exodus-rsync config file.
	Load(context.Context, args.Config) (Config, error)
}

type impl struct{}

// Package provides the default implementation of this package's interface.
var Package Interface = impl{}

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

	// How often to poll for task updates, in milliseconds.
	GwPollInterval int

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
		return Config{}, err
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	out := Config{GwPollInterval: 5000}
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

func (impl) Load(ctx context.Context, args args.Config) (Config, error) {
	logger := log.FromContext(ctx)

	candidates := candidatePaths()
	if args.Conf != "" {
		candidates = []string{args.Conf}
	}

	for _, candidate := range candidates {
		_, err := os.Stat(candidate)
		if err == nil {
			logger.F("path", candidate).Debug("loading config")
			return loadFromPath(candidate)
		}
		logger.F("path", candidate, "error", err).Debug("config file not usable")
	}

	return Config{}, fmt.Errorf("no existing config file in: %s", strings.Join(candidates, ", "))
}

// EnvironmentForDest finds and returns an Environment matching the specified rsync
// destination, or nil if no Environment matches.
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
