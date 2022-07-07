package gw

import (
	"context"
	"fmt"
	"reflect"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

func TestClientTypicalUpload(t *testing.T) {
	client, _ := newClientWithFakeS3(t)

	chdirInTest(t, "../../test/data/srctrees/just-files")

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	// Note: these files have to actually exist because they will be
	// opened by the client for sending. However this test does not require
	// the key to match the actual checksums.
	items := []walk.SyncItem{
		{SrcPath: "hello-copy-one", Key: "abc123"},
		{SrcPath: "hello-copy-two", Key: "abc123"},
		{SrcPath: "subdir/some-binary", Key: "aabbcc"},
	}

	uploaded := make([]walk.SyncItem, 0)
	present := make([]walk.SyncItem, 0)
	duplicate := make([]walk.SyncItem, 0)

	err := client.EnsureUploaded(ctx, items, func(item walk.SyncItem) error {
		uploaded = append(uploaded, item)
		return nil
	}, func(item walk.SyncItem) error {
		present = append(present, item)
		return nil
	}, func(item walk.SyncItem) error {
		duplicate = append(duplicate, item)
		return nil
	})

	if err != nil {
		t.Errorf("got unexpected error %v", err)
	}

	// It should have uploaded these
	if !reflect.DeepEqual(uploaded, []walk.SyncItem{items[0], items[2]}) && !reflect.DeepEqual(uploaded, []walk.SyncItem{items[2], items[0]}) {
		t.Errorf("unexpected set of uploaded items: %v", uploaded)
	}

	// While this one counts as already present, since there was a duplicate
	// object in the publish and only the first is uploaded.
	if !reflect.DeepEqual(duplicate, []walk.SyncItem{items[1]}) {
		t.Errorf("unexpected set of present items: %v", present)
	}
}

func TestClientUploadWithLinks(t *testing.T) {
	client, _ := newClientWithFakeS3(t)

	chdirInTest(t, "../../test/data/srctrees/links")

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	// Note: these files have to actually exist because they will be
	// opened by the client for sending. However this test does not require
	// the key to match the actual checksums.
	items := []walk.SyncItem{
		{SrcPath: "link-to-regular-file", LinkTo: "subdir/regular-file"},
		{SrcPath: "subdir/rand2", LinkTo: "../../../rand2"},
		{SrcPath: "subdir/regular-file", Key: "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"},
		{SrcPath: "subdir2/dir-link", LinkTo: "../subdir"},
		{SrcPath: "subdir/rand1", LinkTo: "../../../rand1:"},
	}

	uploaded := make([]walk.SyncItem, 0)
	present := make([]walk.SyncItem, 0)
	duplicate := make([]walk.SyncItem, 0)

	err := client.EnsureUploaded(ctx, items, func(item walk.SyncItem) error {
		uploaded = append(uploaded, item)
		return nil
	}, func(item walk.SyncItem) error {
		present = append(present, item)
		return nil
	}, func(item walk.SyncItem) error {
		duplicate = append(duplicate, item)
		return nil
	})

	if err != nil {
		t.Errorf("got unexpected error %v", err)
	}

	// It should have uploaded just the one
	if !reflect.DeepEqual(uploaded, []walk.SyncItem{items[2]}) {
		t.Errorf("unexpected set of uploaded items: %v", uploaded)
	}
}

func TestClientPresentItem(t *testing.T) {
	client, s3 := newClientWithFakeS3(t)

	chdirInTest(t, "../../test/data/srctrees/just-files")

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	items := []walk.SyncItem{
		{SrcPath: "existing-blob", Key: "a1b2c3"},
		{SrcPath: "hello-copy-one", Key: "abc123"},
		{SrcPath: "hello-copy-two", Key: "abc123"},
		{SrcPath: "subdir/some-binary", Key: "aabbcc"},
	}

	uploaded := make([]walk.SyncItem, 0)
	present := make([]walk.SyncItem, 0)
	duplicate := make([]walk.SyncItem, 0)

	s3.blobs = blobMap{"a1b2c3": []error{nil}}

	err := client.EnsureUploaded(ctx, items, func(item walk.SyncItem) error {
		uploaded = append(uploaded, item)
		return nil
	}, func(item walk.SyncItem) error {
		present = append(present, item)
		return nil
	}, func(item walk.SyncItem) error {
		duplicate = append(duplicate, item)
		return nil
	})

	if err != nil {
		t.Errorf("got expected error %v", err)
	}
	// It should determine that we should upload only two items
	if !reflect.DeepEqual(uploaded, []walk.SyncItem{items[1], items[3]}) && !reflect.DeepEqual(uploaded, []walk.SyncItem{items[3], items[1]}) {
		t.Errorf("unexpected set of uploaded items: %v", uploaded)
	}

	// Only one item already existed in the s3 bucket
	if !reflect.DeepEqual(present, []walk.SyncItem{items[0]}) {
		t.Errorf("unexpected set of uploaded items: %v", present)
	}

	// Only one item is already slated for upload
	if !reflect.DeepEqual(duplicate, []walk.SyncItem{items[2]}) {
		t.Errorf("unexpected set of uploaded items: %v", duplicate)
	}
}

func TestClientUploadCallbackError(t *testing.T) {
	client, _ := newClientWithFakeS3(t)

	chdirInTest(t, "../../test/data/srctrees/just-files")

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	items := []walk.SyncItem{
		{SrcPath: "hello-copy-one", Key: "abc123"},
		{SrcPath: "hello-copy-two", Key: "abc123"},
		{SrcPath: "subdir/some-binary", Key: "aabbcc"},
	}

	err := client.EnsureUploaded(ctx, items, func(item walk.SyncItem) error {
		return fmt.Errorf("error from callback")
	}, func(item walk.SyncItem) error {
		return nil
	}, func(item walk.SyncItem) error {
		return nil
	})

	if err.Error() != "error from callback" {
		t.Errorf("did not get expected error, got: %v", err)
	}
}

func TestClientUploadPresentCallbackError(t *testing.T) {
	client, s3 := newClientWithFakeS3(t)

	chdirInTest(t, "../../test/data/srctrees/just-files")

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	items := []walk.SyncItem{
		{SrcPath: "hello-copy-one", Key: "abc123"},
		{SrcPath: "hello-copy-two", Key: "abc123"},
		{SrcPath: "subdir/some-binary", Key: "aabbcc"},
	}

	s3.blobs = blobMap{"aabbcc": []error{nil}}

	err := client.EnsureUploaded(ctx, items, func(item walk.SyncItem) error {
		return nil
	}, func(item walk.SyncItem) error {
		return fmt.Errorf("error from callback")
	}, func(item walk.SyncItem) error {
		return nil
	})

	if err.Error() != "error from callback" {
		t.Errorf("did not get expected error, got: %v", err)
	}
}

func TestClientUploadDuplicateCallbackError(t *testing.T) {
	client, _ := newClientWithFakeS3(t)

	chdirInTest(t, "../../test/data/srctrees/just-files")

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	items := []walk.SyncItem{
		{SrcPath: "hello-copy-one", Key: "abc123"},
		{SrcPath: "hello-copy-two", Key: "abc123"},
		{SrcPath: "subdir/some-binary", Key: "aabbcc"},
	}

	err := client.EnsureUploaded(ctx, items, func(item walk.SyncItem) error {
		return nil
	}, func(item walk.SyncItem) error {
		return nil
	}, func(item walk.SyncItem) error {
		return fmt.Errorf("error from callback")

	})

	if err.Error() != "error from callback" {
		t.Errorf("did not get expected error, got: %v", err)
	}
}
