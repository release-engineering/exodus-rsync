package conf

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/stretchr/testify/assert"
)

func TestCandidatePaths(t *testing.T) {
	// It should not crash
	paths := candidatePaths()

	assert.Len(t, paths, 3)
	assert.Contains(t, paths, "exodus-rsync.conf")
}

func TestOverrideValues(t *testing.T) {
	dir := t.TempDir()
	filename := filepath.Join(dir, "test.conf")

	err := os.WriteFile(filename, []byte(`

# Some global values.
gwenv: global-env
gwurl: https://exodus-gw.example.com
gwcert: global-cert
gwkey: global-key
gwbatchsize: 100

environments:
- prefix: dest
  gwenv: one-env
  gwkey: override-key
  gwpollinterval: 123
  rsyncmode: mixed

`), 0755)

	if err != nil {
		t.Fatalf("could not write config file for test: %v", err)
	}

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	var cfg GlobalConfig
	cfg, err = loadFromPath(filename, args.Config{})

	if err != nil {
		t.Fatalf("could not load config file: %v", err)
	}

	env := cfg.EnvironmentForDest(ctx, "dest:/foo/bar")

	// It should be able to get the environment.
	if env == nil {
		t.Fatalf("Couldn't get environment")
	}

	// It shouldn't be able to get environments which had no matching prefix.
	missingEnv := cfg.EnvironmentForDest(ctx, "nomatch-dest:/foo/bar")
	if missingEnv != nil {
		t.Fatalf("Unexpectedly found this environment for nomatch-dest: %v", missingEnv)
	}

	// Helper for following equality tests
	assertEqual := func(name string, x, y interface{}) {
		t.Run(name, func(t *testing.T) {
			if !reflect.DeepEqual(x, y) {
				t.Errorf("actual: %v, expected: %v", x, y)
			}
		})
	}

	// Global values should be as expected.
	assertEqual("global gwcert", cfg.GwCert(), "global-cert")
	assertEqual("global gwkey", cfg.GwKey(), "global-key")
	assertEqual("global gwenv", cfg.GwEnv(), "global-env")
	assertEqual("global gwpollinterval", cfg.GwPollInterval(), 5000)
	assertEqual("global rsyncmode", cfg.RsyncMode(), "exodus")

	// Values can be overridden in environment.
	assertEqual("env gwenv", env.GwEnv(), "one-env")
	assertEqual("env gwkey", env.GwKey(), "override-key")
	assertEqual("env gwpollinterval", env.GwPollInterval(), 123)
	assertEqual("env rsyncmode", env.RsyncMode(), "mixed")

	// For values which are NOT overridden, they should be equal to global.
	assertEqual("env gwurl", env.GwURL(), cfg.GwURL())
	assertEqual("env gwcert", env.GwCert(), cfg.GwCert())
	assertEqual("env gwbatchsize", env.GwBatchSize(), cfg.GwBatchSize())
}

func TestDefaultsFromParent(t *testing.T) {
	cfg := globalConfig{}

	cfg.GwCertRaw = "cert"
	cfg.GwPollIntervalRaw = 123
	cfg.args.Verbose = 1

	env := environment{parent: &cfg}

	if env.GwCert() != "cert" {
		t.Errorf("did not get GwCert from parent")
	}
	if env.GwPollInterval() != 123 {
		t.Errorf("did not get GwPollInterval from parent")
	}
	if env.Verbosity() != 1 {
		t.Errorf("did not get args.Verbose from parent")
	}
}
