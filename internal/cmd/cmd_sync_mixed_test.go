package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/rsync"
)

type fakeRsync struct {
	delegate rsync.Interface
	prefix   []string
}

func (r *fakeRsync) Command(ctx context.Context, args []string) *exec.Cmd {
	cmd := r.delegate.Command(ctx, args)

	cmd.Path = r.prefix[0]
	cmd.Args = append(r.prefix, cmd.Args...)

	if !strings.Contains(cmd.Path, "/") {
		newPath, err := exec.LookPath(cmd.Path)
		if err != nil {
			panic(err)
		}
		cmd.Path = newPath
	}

	return cmd
}

func (r *fakeRsync) Exec(ctx context.Context, args args.Config) error {
	return fmt.Errorf("this test is not supposed to Exec")
}

func (r *fakeRsync) RawExec(ctx context.Context, args []string) error {
	return fmt.Errorf("this test is not supposed to RawExec")
}

func TestMainSyncMixedOk(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	rsync := &fakeRsync{delegate: ext.rsync}

	// Make it just run "echo" instead of real rsync, thus forcing rsync
	// to "succeed". Sleep is to get predictable timing, with exodus publish
	// finishing first - assists in reaching 100% coverage.
	rsync.prefix = []string{"/bin/sh", "-c", "sleep 2; echo FAKE RSYNC", "--"}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	log := CaptureLogger(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	ext.rsync = rsync

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/links")

	args := []string{
		"rsync",
		"-vvv",
		srcPath + "/",
		"exodus-mixed:/dest",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// It should have created one publish.
	if len(client.publishes) != 1 {
		t.Error("expected to create 1 publish, instead created", len(client.publishes))
	}

	p := client.publishes[0]

	// Build up a URI => Key mapping of what was published
	itemMap := make(map[string]string)
	for _, item := range p.items {
		if _, ok := itemMap[item.WebURI]; ok {
			t.Error("tried to publish this URI more than once:", item.WebURI)
		}
		itemMap[item.WebURI] = item.ObjectKey
	}

	// It should have been exactly this
	expectedItems := map[string]string{
		"/dest/link-to-regular-file":          "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/some/dir/link-to-somefile":     "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/dest/some/somefile":                 "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/dest/subdir/regular-file":           "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/subdir/rand1":                  "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/dest/subdir/rand2":                  "f3a5340ae2a400803b8150f455ad285d173cbdcf62c8e9a214b30f467f45b310",
		"/dest/subdir2/dir-link/regular-file": "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/subdir2/dir-link/rand1":        "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/dest/subdir2/dir-link/rand2":        "f3a5340ae2a400803b8150f455ad285d173cbdcf62c8e9a214b30f467f45b310",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}

	// It should have logged the output of rsync, which can be found
	// as entries with 'rsync' field
	rsyncText := ""
	for _, entry := range log.Entries {
		_, ok := entry.Fields["rsync"]
		if ok {
			rsyncText = rsyncText + entry.Message + "\n"
		}
	}

	if !strings.Contains(rsyncText, "FAKE RSYNC") {
		t.Errorf("Did not generate expected rsync logs, got: %v", rsyncText)
	}
}
