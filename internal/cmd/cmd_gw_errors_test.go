package cmd

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

type mockClientConfigurator func(*gomock.Controller, *gw.MockClient)

func setupFailedUpload(_ *gomock.Controller, client *gw.MockClient) {
	client.EXPECT().EnsureUploaded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("simulated error"))
}

func setupFailedNewPublish(_ *gomock.Controller, client *gw.MockClient) {
	// EnsureUploaded succeeds...
	client.EXPECT().EnsureUploaded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil)

	// ...then creating a publish fails
	client.EXPECT().NewPublish(gomock.Any()).Return(nil, fmt.Errorf("simulated error"))
}

func setupFailedAddItems(ctrl *gomock.Controller, client *gw.MockClient) {
	// EnsureUploaded succeeds, and (importantly) must add some items
	client.EXPECT().EnsureUploaded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ interface{}, _ interface{}, onUploaded func(walk.SyncItem) error, _ interface{}) {
			// Simulate that a couple of items were uploaded.
			onUploaded(walk.SyncItem{SrcPath: "file1", Key: "abc123"})
			onUploaded(walk.SyncItem{SrcPath: "file2", Key: "aabbcc"})
		}).
		Return(nil)

	// Creating a publish succeeds
	publish := gw.NewMockPublish(ctrl)
	client.EXPECT().NewPublish(gomock.Any()).Return(publish, nil)

	publish.EXPECT().ID().Return("test-publish").AnyTimes()

	// Publish can't have items added
	publish.EXPECT().AddItems(gomock.Any(), gomock.Any()).Return(fmt.Errorf("simulated error"))
}

func setupFailedCommit(ctrl *gomock.Controller, client *gw.MockClient) {
	// EnsureUploaded succeeds, and (importantly) must add some items
	client.EXPECT().EnsureUploaded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ interface{}, _ interface{}, onUploaded func(walk.SyncItem) error, _ interface{}) {
			// Simulate that a couple of items were uploaded.
			onUploaded(walk.SyncItem{SrcPath: "file1", Key: "abc123"})
			onUploaded(walk.SyncItem{SrcPath: "file2", Key: "aabbcc"})
		}).
		Return(nil)

	// Creating a publish succeeds
	publish := gw.NewMockPublish(ctrl)
	client.EXPECT().NewPublish(gomock.Any()).Return(publish, nil)

	publish.EXPECT().ID().Return("test-publish").AnyTimes()

	// Adding items succeeds
	publish.EXPECT().AddItems(gomock.Any(), gomock.Any()).Return(nil)

	// Committing fails
	publish.EXPECT().Commit(gomock.Any()).Return(fmt.Errorf("simulated error"))
}

func TestMainUploadFailed(t *testing.T) {
	tests := []struct {
		message   string
		setupMock mockClientConfigurator
		exitCode  int
	}{
		{"can't upload files", setupFailedUpload, 25},
		{"can't create publish", setupFailedNewPublish, 62},
		{"can't add items to publish", setupFailedAddItems, 51},
		{"can't commit publish", setupFailedCommit, 71},
	}
	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {

			logs := CaptureLogger(t)
			ctrl := MockController(t)

			mockGw := gw.NewMockInterface(ctrl)
			ext.gw = mockGw

			mockClient := gw.NewMockClient(ctrl)

			SetConfig(t, `
gwcert: $HOME/certs/$USER.crt
gwkey: $HOME/certs/$USER.key
gwurl: https://exodus-gw.example.com/

environments:
- prefix: some-dest
  gwenv: test
`)

			mockGw.EXPECT().NewClient(gomock.Any()).Return(mockClient, nil)

			tt.setupMock(ctrl, mockClient)

			exitCode := Main([]string{
				"exodus-rsync", ".", "some-dest:/foo/bar",
			})

			// It should exit with error.
			if exitCode != tt.exitCode {
				t.Error("returned incorrect exit code", exitCode)
			}

			entry := FindEntry(logs, tt.message)
			if entry == nil {
				t.Fatal("missing expected log message")
			}

			if fmt.Sprint(entry.Fields["error"]) != "simulated error" {
				t.Errorf("unexpected error %v", entry.Fields["error"])
			}
		})
	}
}
