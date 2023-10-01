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
gwurl: $TEST_EXODUS_GW_URL
gwcert: global-cert
gwkey: global-key
gwbatchsize: 100
gwcommit: abc
strip: dest:/foo

environments:
- prefix: dest:/foo/bar/baz
  gwenv: $TEST_EXODUS_GW_ENV
  gwkey: override-key
  gwpollinterval: 123
  gwcommit: cba
  rsyncmode: mixed
  strip: dest:/foo/bar
  uploadthreads: 6

`), 0755)

	if err != nil {
		t.Fatalf("could not write config file for test: %v", err)
	}

	oldURL := os.Getenv("TEST_EXODUS_GW_URL")
	err = os.Setenv("TEST_EXODUS_GW_URL", "https://exodus-gw.example.com")
	if err != nil {
		t.Fatalf("could not set TEST_EXODUS_GW_URL, err = %v", err)
	}
	oldEnv := os.Getenv("TEST_EXODUS_GW_ENV")
	err = os.Setenv("TEST_EXODUS_GW_ENV", "one-env")
	if err != nil {
		t.Fatalf("could not set TEST_EXODUS_GW_ENV, err = %v", err)
	}

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	var cfg GlobalConfig
	cfg, err = loadFromPath(filename, args.Config{})

	if err != nil {
		t.Fatalf("could not load config file: %v", err)
	}

	env := cfg.EnvironmentForDest(ctx, "dest:/foo/bar/baz")

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
	assertEqual("global gwcommit", cfg.GwCommit(), "abc")
	assertEqual("global rsyncmode", cfg.RsyncMode(), "exodus")
	assertEqual("global strip", cfg.Strip(), "dest:/foo")
	assertEqual("global uploadthreads", cfg.UploadThreads(), 4)

	// Values can be overridden in environment.
	assertEqual("env gwenv", env.GwEnv(), "one-env")
	assertEqual("env gwkey", env.GwKey(), "override-key")
	assertEqual("env gwpollinterval", env.GwPollInterval(), 123)
	assertEqual("env gwcommit", env.GwCommit(), "cba")
	assertEqual("env rsyncmode", env.RsyncMode(), "mixed")
	assertEqual("env strip", env.Strip(), "dest:/foo/bar")
	assertEqual("env uploadthreads", env.UploadThreads(), 6)

	// For values which are NOT overridden, they should be equal to global.
	assertEqual("env gwurl", env.GwURL(), cfg.GwURL())
	assertEqual("env gwcert", env.GwCert(), cfg.GwCert())
	assertEqual("env gwbatchsize", env.GwBatchSize(), cfg.GwBatchSize())

	t.Cleanup(func() {
		os.Setenv("TEST_EXODUS_GW_ENV", oldEnv)
		os.Setenv("TEST_EXODUS_GW_URL", oldURL)
	})
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
