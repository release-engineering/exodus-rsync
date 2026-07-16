package gw

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/release-engineering/exodus-rsync/internal/args"
	"github.com/release-engineering/exodus-rsync/internal/log"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

type setupBlobs func(blobMap)

func defaultBlobs(blobMap) {}

func typicalError(blobs blobMap) {
	// Interacting with this blob gives an error.
	blobs["abc123"] = []error{fmt.Errorf("simulated error")}
}

func putError(blobs blobMap) {
	// Querying this blob says it doesn't exist, but then uploading it fails.
	blobs["abc123"] = []error{
		errFakeObjectMissing,
		fmt.Errorf("simulated error"),
	}
}

func TestClientUploadErrors(t *testing.T) {
	client, srv := newClientWithFakeS3(t)

	chdirInTest(t, "../../test/data/srctrees/just-files")

	ctx := context.Background()
	ctx = log.NewContext(ctx, log.Package.NewLogger(args.Config{}))

	tests := []struct {
		name      string
		items     []walk.SyncItem
		setup     setupBlobs
		wantError string
	}{

		{"error checking blob",
			[]walk.SyncItem{{SrcPath: "some-file", Key: "abc123"}},
			typicalError,
			"api error InternalServerError: Internal Server Error"},
		{"nonexistent file",
			[]walk.SyncItem{{SrcPath: "nonexistent-file", Key: "abc123"}},
			defaultBlobs,
			"open nonexistent-file: no such file or directory"},
		{"PUT fails",
			[]walk.SyncItem{{SrcPath: "hello-copy-one", Key: "abc123"}},
			putError,
			"api error InternalError: simulated error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv.Reset()
			tt.setup(srv.Blobs())

			err := client.EnsureUploaded(ctx, tt.items, func(item walk.SyncItem) error {
				t.Fatal("unexpectedly uploaded something", item)
				return nil
			}, func(item walk.SyncItem) error {
				t.Fatal("unexpectedly found blob", item)
				return nil
			}, func(item walk.SyncItem) error {
				t.Fatal("unexpectedly created duplicate blob", item)
				return nil
			})

			// It should tell us why it failed
			if !strings.Contains(fmt.Sprint(err), tt.wantError) {
				t.Errorf("did not get expected error, got err = %v", err)
			}
		})
	}

}
