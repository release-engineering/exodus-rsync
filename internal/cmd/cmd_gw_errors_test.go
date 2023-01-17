package cmd

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/release-engineering/exodus-rsync/internal/gw"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

type mockClientConfigurator func(*gomock.Controller, *gw.MockClient)

func setupFailedUpload(ctrl *gomock.Controller, client *gw.MockClient) {
	// Creating a publish succeeds
	publish := gw.NewMockPublish(ctrl)
	client.EXPECT().NewPublish(gomock.Any()).Return(publish, nil)

	publish.EXPECT().ID().Return("3e0a4539-be4a-437e-a45f-6d72f7192f17").AnyTimes()

	client.EXPECT().EnsureUploaded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("simulated error"))
}

func setupFailedNewPublish(_ *gomock.Controller, client *gw.MockClient) {
	client.EXPECT().NewPublish(gomock.Any()).Return(nil, fmt.Errorf("simulated error"))
}

func setupFailedAddItems(ctrl *gomock.Controller, client *gw.MockClient) {
	// EnsureUploaded succeeds, and (importantly) must add some items
	client.EXPECT().EnsureUploaded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ interface{}, _ interface{}, onUploaded func(walk.SyncItem) error, onPresent func(walk.SyncItem) error, onDuplicate func(walk.SyncItem) error) {
			// Simulate that a couple of items were uploaded.
			onUploaded(walk.SyncItem{SrcPath: "file1", Key: "abc123"})
			onUploaded(walk.SyncItem{SrcPath: "file2", Key: "aabbcc"})
			onDuplicate(walk.SyncItem{SrcPath: "file3", Key: "abc123"})
			onPresent(walk.SyncItem{SrcPath: "file4", Key: "a1b2c3"})
		}).
		Return(nil)

	// Creating a publish succeeds
	publish := gw.NewMockPublish(ctrl)
	client.EXPECT().NewPublish(gomock.Any()).Return(publish, nil)

	publish.EXPECT().ID().Return("3e0a4539-be4a-437e-a45f-6d72f7192f17").AnyTimes()

	// Publish can't have items added
	publish.EXPECT().AddItems(gomock.Any(), gomock.Any()).Return(fmt.Errorf("simulated error"))
}

func setupFailedCommit(ctrl *gomock.Controller, client *gw.MockClient) {
	// EnsureUploaded succeeds, and (importantly) must add some items
	client.EXPECT().EnsureUploaded(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(_ interface{}, _ interface{}, onUploaded func(walk.SyncItem) error, _ interface{}, _ interface{}) {
			// Simulate that a couple of items were uploaded.
			onUploaded(walk.SyncItem{SrcPath: "file1", Key: "abc123"})
			onUploaded(walk.SyncItem{SrcPath: "file2", Key: "aabbcc"})
		}).
		Return(nil)

	// Creating a publish succeeds
	publish := gw.NewMockPublish(ctrl)
	client.EXPECT().NewPublish(gomock.Any()).Return(publish, nil)

	publish.EXPECT().ID().Return("3e0a4539-be4a-437e-a45f-6d72f7192f17").AnyTimes()

	// Adding items succeeds
	publish.EXPECT().AddItems(gomock.Any(), gomock.Any()).Return(nil)

	// Committing fails
	publish.EXPECT().Commit(gomock.Any()).Return(fmt.Errorf("simulated error"))
}

func setupFailedJoinPublish(ctrl *gomock.Controller, client *gw.MockClient) {
	publish := gw.NewMockPublish(ctrl)

	client.EXPECT().GetPublish(gomock.Any(), gomock.Any()).Return(publish, fmt.Errorf("simulated error"))
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

			mockGw.EXPECT().NewClient(gomock.Any(), gomock.Any()).Return(mockClient, nil)

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

func TestMainJoinPublishFailed(t *testing.T) {
	test := struct {
		message   string
		setupMock mockClientConfigurator
		exitCode  int
	}{"can't join publish", setupFailedJoinPublish, 67}

	t.Run(test.message, func(t *testing.T) {

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

		mockGw.EXPECT().NewClient(gomock.Any(), gomock.Any()).Return(mockClient, nil)

		test.setupMock(ctrl, mockClient)

		exitCode := Main([]string{
			"exodus-rsync", ".", "some-dest:/foo/bar", "--exodus-publish", "3e0a4539-be4a-437e-a45f-6d72f7192f17",
		})

		// It should exit with error.
		if exitCode != test.exitCode {
			t.Error("returned incorrect exit code", exitCode)
		}

		entry := FindEntry(logs, test.message)
		if entry == nil {
			t.Fatal("missing expected log message")
		}

		if fmt.Sprint(entry.Fields["error"]) != "simulated error" {
			t.Errorf("unexpected error %v", entry.Fields["error"])
		}
	})
}
