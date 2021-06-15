package gw

import (
	"os"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/conf"
)

func chdirInTest(t *testing.T, path string) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	err = os.Chdir(path)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		os.Chdir(wd)
	})
}

// Returns an implementation of Config which has a valid env defined,
// pointing at a real cert/key in testdata.
func testConfig(t *testing.T) conf.Config {
	ctrl := gomock.NewController(t)
	cfg := conf.NewMockConfig(ctrl)

	cfg.EXPECT().GwCert().AnyTimes().Return("../../test/data/service.pem")
	cfg.EXPECT().GwKey().AnyTimes().Return("../../test/data/service-key.pem")
	cfg.EXPECT().GwURL().AnyTimes().Return("https://exodus-gw.example.com")
	cfg.EXPECT().GwPollInterval().AnyTimes().Return(1)
	cfg.EXPECT().GwEnv().AnyTimes().Return("env")
	cfg.EXPECT().GwBatchSize().AnyTimes().Return(3)
	cfg.EXPECT().LogLevel().AnyTimes().Return("info")
	cfg.EXPECT().Verbosity().AnyTimes().Return(3)

	return cfg
}
