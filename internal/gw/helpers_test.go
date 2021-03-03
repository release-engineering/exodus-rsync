package gw

import (
	"os"
	"testing"
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
