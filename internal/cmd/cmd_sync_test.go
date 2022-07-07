package cmd

import (
	"context"
	"fmt"
	"os"
	"path"
	"reflect"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

const CONFIG string = `
environments:
- prefix: exodus
  gwenv: best-env

- prefix: exodus-mixed
  gwenv: best-env
  rsyncmode: mixed

- prefix: somehost:/cdn/root
  gwenv: best-env
  rsyncmode: exodus

- prefix: otherhost:/foo/bar/baz
  gwenv: best-env
  rsyncmode: exodus
  strip: otherhost:/foo
`

type EnvMatcher struct {
	name string
}

func (m EnvMatcher) Matches(x interface{}) bool {
	env, ok := x.(conf.EnvironmentConfig)
	if !ok {
		return false
	}
	return env.GwEnv() == m.name
}

func (m EnvMatcher) String() string {
	return fmt.Sprintf("Environment '%s'", m.name)
}

type FakeClient struct {
	blobs     map[string]string
	publishes []FakePublish
}

type FakePublish struct {
	items     []gw.ItemInput
	committed int
	id        string
}

type BrokenPublish struct {
	id string
}

func (c *FakeClient) EnsureUploaded(ctx context.Context, items []walk.SyncItem,
	onUploaded func(walk.SyncItem) error,
	onExisting func(walk.SyncItem) error,
	onDuplicate func(walk.SyncItem) error,
) error {
	var err error
	processedItems := make(map[string]walk.SyncItem)

	for _, item := range items {
		if _, ok := processedItems[item.Key]; ok {
			err = onDuplicate(item)
		} else if _, ok := c.blobs[item.Key]; ok {
			err = onExisting(item)
		} else {
			c.blobs[item.Key] = item.SrcPath
			processedItems[item.Key] = item
			err = onUploaded(item)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *FakeClient) NewPublish(ctx context.Context) (gw.Publish, error) {
	c.publishes = append(c.publishes, FakePublish{id: "some-publish"})
	return &c.publishes[len(c.publishes)-1], nil
}

func (c *FakeClient) GetPublish(id string) gw.Publish {
	for idx := range c.publishes {
		if c.publishes[idx].id == id {
			return &c.publishes[idx]
		}
	}
	// Didn't find any, then return a broken one
	return &BrokenPublish{id: id}
}

func (c *FakeClient) WhoAmI(context.Context) (map[string]interface{}, error) {
	out := make(map[string]interface{})
	out["whoami"] = "fake-info"
	return out, nil
}

func (p *FakePublish) AddItems(ctx context.Context, items []gw.ItemInput) error {
	if p.committed != 0 {
		return fmt.Errorf("attempted to modify committed publish")
	}
	p.items = append(p.items, items...)
	return nil
}

func (p *BrokenPublish) AddItems(_ context.Context, _ []gw.ItemInput) error {
	return fmt.Errorf("invalid publish")
}

func (p *BrokenPublish) Commit(_ context.Context) error {
	return fmt.Errorf("invalid publish")
}

func (p *FakePublish) Commit(ctx context.Context) error {
	p.committed++
	return nil
}

func (p *FakePublish) ID() string {
	return p.id
}

func (p *BrokenPublish) ID() string {
	return p.id
}

func TestMainTypicalSync(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	args := []string{
		"rsync",
		srcPath + "/",
		"exodus:/some/target",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// Check paths of some blobs we expected to deal with.
	binPath := client.blobs["c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6"]
	helloPath := client.blobs["5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"]

	// It should have uploaded the binary from here
	if binPath != srcPath+"/subdir/some-binary" {
		t.Error("binary uploaded from unexpected path", binPath)
	}

	// For the hello file, since there were two copies, it's undefined which one of them
	// was used for the upload - but should be one of them.
	if helloPath != srcPath+"/hello-copy-one" && helloPath != srcPath+"/hello-copy-two" {
		t.Error("hello uploaded from unexpected path", helloPath)
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
		"/some/target/subdir/some-binary": "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
		"/some/target/hello-copy-one":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/some/target/hello-copy-two":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncFilter(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees")

	args := []string{
		"rsync",
		"--filter", "+ */",
		"--filter", "+/ **/hello-copy*",
		"--filter", "- *",
		srcPath + "/",
		"exodus:/some/target",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// Check paths of some blobs we expected to deal with.
	helloPath := client.blobs["5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"]

	// For the hello file, since there were two copies, it's undefined which one of them
	// was used for the upload - but should be one of them.
	if helloPath != srcPath+"/just-files/hello-copy-one" && helloPath != srcPath+"/just-files/hello-copy-two" {
		t.Error("hello uploaded from unexpected path", helloPath)
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
		"/some/target/just-files/hello-copy-one": "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/some/target/just-files/hello-copy-two": "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncFilterIsRelative(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	// Nothing should match --exclude, as filtered paths are relative.
	args := []string{
		"rsync",
		"--exclude", path.Clean(wd),
		srcPath + "/",
		"exodus:/some/target",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// Check paths of some blobs we expected to deal with.
	helloPath := client.blobs["5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"]

	// For the hello file, since there were two copies, it's undefined which one of them
	// was used for the upload - but should be one of them.
	if helloPath != srcPath+"/hello-copy-one" && helloPath != srcPath+"/hello-copy-two" {
		t.Error("hello uploaded from unexpected path", helloPath)
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
		"/some/target/hello-copy-one":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/some/target/hello-copy-two":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/some/target/subdir/some-binary": "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncFollowsLinks(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/links")

	args := []string{
		"rsync",
		"-vvv",
		srcPath + "/",
		"exodus:/dest",
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
		"/dest/subdir/regular-file":           "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/subdir/rand1":                  "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/dest/subdir/rand2":                  "f3a5340ae2a400803b8150f455ad285d173cbdcf62c8e9a214b30f467f45b310",
		"/dest/subdir2/dir-link/regular-file": "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/subdir2/dir-link/rand1":        "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/dest/subdir2/dir-link/rand2":        "f3a5340ae2a400803b8150f455ad285d173cbdcf62c8e9a214b30f467f45b310",
		"/dest/some/somefile":                 "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/dest/some/dir/link-to-somefile":     "57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncDontFollowLinks(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/links")

	args := []string{
		"rsync",
		"-lvvv",
		srcPath + "/",
		"somehost:/cdn/root/some/target",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// Check paths of some blobs we expected to deal with.
	binPath := client.blobs["5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"]

	// It should have uploaded the binary from here
	if binPath != srcPath+"/subdir/regular-file" {
		t.Error("binary uploaded from unexpected path", binPath)
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

		if item.LinkTo != "" {
			itemMap[item.WebURI] = "link-" + item.LinkTo
		} else if item.ObjectKey != "" {
			itemMap[item.WebURI] = "key-" + item.ObjectKey
		} else {
			t.Error("no object_key or link_to generated:", item.WebURI)
		}
	}

	// It should have been exactly this
	expectedItems := map[string]string{
		"/some/target/link-to-regular-file":      "link-/some/target/subdir/regular-file",
		"/some/target/some/somefile":             "key-57921e8a0929eaff5003cc9dd528c3421296055a4de2ba72429dc7f41bfa8411",
		"/some/target/some/dir/link-to-somefile": "link-/some/target/some/somefile",
		"/some/target/subdir/rand1":              "link-/rand1",
		"/some/target/subdir/rand2":              "link-/rand2",
		"/some/target/subdir/regular-file":       "key-5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/some/target/subdir2/dir-link":          "link-/some/target/subdir",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

// When src tree has no trailing slash, the basename is repeated as a directory
// name on the destination.
func TestMainSyncNoSlash(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	args := []string{
		"rsync",
		"-vvv",
		srcPath,
		"exodus:/dest",
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
		"/dest/just-files/hello-copy-one":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/just-files/hello-copy-two":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/just-files/subdir/some-binary": "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncFilesFrom(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data")
	filesFromPath := path.Clean(wd + "/../../test/data/source-list.txt")

	args := []string{
		"rsync",
		"-vvv",
		"--files-from", filesFromPath,
		srcPath,
		"exodus:/dest",
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

	// Build up a URI => Key mapping of what was published.
	itemMap := make(map[string]string)
	for _, item := range p.items {
		if _, ok := itemMap[item.WebURI]; ok {
			t.Error("tried to publish this URI more than once:", item.WebURI)
		}
		itemMap[item.WebURI] = item.ObjectKey
	}

	// Paths should be comprised of the dest and the path written in the file.
	expectPath1 := path.Join("/dest", "srctrees/just-files/subdir/some-binary")
	expectPath2 := path.Join("/dest", "srctrees/some.conf")

	// It should have been exactly this.
	expectedItems := map[string]string{
		expectPath1: "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
		expectPath2: "4cfe7dba345453b9e2e7a505084238095511ef673e03b6a016f871afe2dfa599",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once).
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncRelative(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files/subdir")

	args := []string{
		"rsync",
		"-vvv",
		"--relative",
		srcPath + "/",
		"exodus:/dest",
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

	// Build up a URI => Key mapping of what was published.
	itemMap := make(map[string]string)
	for _, item := range p.items {
		if _, ok := itemMap[item.WebURI]; ok {
			t.Error("tried to publish this URI more than once:", item.WebURI)
		}
		itemMap[item.WebURI] = item.ObjectKey
	}

	// Full source path should be preserved due to --relative.
	expectPath := path.Join("/dest", srcPath, "some-binary")

	// It should have been exactly this.
	expectedItems := map[string]string{
		expectPath: "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once).
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncJoinPublish(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	// Set up that this publish already exists.
	client.publishes = []FakePublish{{items: make([]gw.ItemInput, 0), id: "abc123"}}

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	args := []string{
		"rsync",
		"-vvv",
		"--exodus-publish", "abc123",
		srcPath,
		"exodus:/dest",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// It should have left the one publish there without creating any more
	if len(client.publishes) != 1 {
		t.Error("should have used 1 existing publish, instead have", len(client.publishes))
	}

	p := client.publishes[0]

	// It should NOT have committed the publish since it already existed
	if p.committed != 0 {
		t.Error("publish committed unexpectedly? p.committed ==", p.committed)
	}

	// Build up a URI => Key mapping of what was published
	itemMap := make(map[string]string)
	for _, item := range p.items {
		if _, ok := itemMap[item.WebURI]; ok {
			t.Error("tried to publish this URI more than once:", item.WebURI)
		}
		itemMap[item.WebURI] = item.ObjectKey
	}

	// It should have added these items to the publish, as normal
	expectedItems := map[string]string{
		"/dest/just-files/hello-copy-one":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/just-files/hello-copy-two":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/dest/just-files/subdir/some-binary": "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}
}

func TestMainStripFromPrefix(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	args := []string{
		"rsync",
		srcPath + "/",
		"otherhost:/foo/bar/baz/my/dest",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// Check paths of some blobs we expected to deal with.
	binPath := client.blobs["c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6"]
	helloPath := client.blobs["5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"]

	// It should have uploaded the binary from here
	if binPath != srcPath+"/subdir/some-binary" {
		t.Error("binary uploaded from unexpected path", binPath)
	}

	// For the hello file, since there were two copies, it's undefined which one of them
	// was used for the upload - but should be one of them.
	if helloPath != srcPath+"/hello-copy-one" && helloPath != srcPath+"/hello-copy-two" {
		t.Error("hello uploaded from unexpected path", helloPath)
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
		"/bar/baz/my/dest/subdir/some-binary": "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
		"/bar/baz/my/dest/hello-copy-one":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/bar/baz/my/dest/hello-copy-two":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainStripDefaultPrefix(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	args := []string{
		"rsync",
		"-vvv",
		srcPath + "/",
		"somehost:/cdn/root/my/dest",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	// Check paths of some blobs we expected to deal with.
	binPath := client.blobs["c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6"]
	helloPath := client.blobs["5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"]

	// It should have uploaded the binary from here
	if binPath != srcPath+"/subdir/some-binary" {
		t.Error("binary uploaded from unexpected path", binPath)
	}

	// For the hello file, since there were two copies, it's undefined which one of them
	// was used for the upload - but should be one of them.
	if helloPath != srcPath+"/hello-copy-one" && helloPath != srcPath+"/hello-copy-two" {
		t.Error("hello uploaded from unexpected path", helloPath)
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
		"/my/dest/subdir/some-binary": "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
		"/my/dest/hello-copy-one":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/my/dest/hello-copy-two":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainSyncSingleFile(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	srcPath := path.Clean(wd + "/../../test/data/srctrees/single-file/test")

	args := []string{
		"rsync",
		"-vvv",
		srcPath,
		"exodus:/dest/test",
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
		"/dest/test": "98ea6e4f216f2fb4b69fff9b3a44842c38686ca685f3f55dc48c5d3fb1107be4",
	}

	if !reflect.DeepEqual(itemMap, expectedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}

	// It should have committed the publish (once)
	if p.committed != 1 {
		t.Error("expected to commit publish (once), instead p.committed ==", p.committed)
	}
}

func TestMainTypicalSyncWithExistingItems(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	SetConfig(t, CONFIG)
	ctrl := MockController(t)

	mockGw := gw.NewMockInterface(ctrl)
	ext.gw = mockGw

	srcPath := path.Clean(wd + "/../../test/data/srctrees/just-files")

	client := FakeClient{blobs: make(map[string]string)}
	mockGw.EXPECT().NewClient(gomock.Any(), EnvMatcher{"best-env"}).Return(&client, nil)

	// The hello file already exists in our bucket
	client.blobs["5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"] = "/some/other/source/some-file"

	args := []string{
		"rsync",
		srcPath + "/",
		"exodus:/some/target",
	}

	got := Main(args)

	// It should complete successfully.
	if got != 0 {
		t.Error("returned incorrect exit code", got)
	}

	p := client.publishes[0]
	blobs := client.blobs

	// Build up a URI => Key mapping of what was uploaded
	itemMap := make(map[string]string)
	for _, item := range p.items {
		if _, ok := itemMap[item.WebURI]; ok {
			t.Error("tried to publish this URI more than once:", item.WebURI)
		}
		itemMap[item.WebURI] = item.ObjectKey
	}

	// Only the binary file should have been uploaded.
	// The hello file already existed in the bucket and thus should not have been re-uploaded.
	expectedUploadedItems := map[string]string{
		"5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03": "/some/other/source/some-file",
		"c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6": srcPath + "/subdir/some-binary",
	}
	if !reflect.DeepEqual(blobs, expectedUploadedItems) {
		t.Error("did not upload expected items, uploaded:", blobs)
	}

	// The hello files should be published despite already existing in the bucket
	expectedPublishedItems := map[string]string{
		"/some/target/subdir/some-binary": "c66f610d98b2c9fe0175a3e99ba64d7fc7de45046515ff325be56329a9347dd6",
		"/some/target/hello-copy-one":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
		"/some/target/hello-copy-two":     "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
	}
	if !reflect.DeepEqual(itemMap, expectedPublishedItems) {
		t.Error("did not publish expected items, published:", itemMap)
	}
}
