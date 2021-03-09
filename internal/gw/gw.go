package gw

import (
	"context"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/release-engineering/exodus-rsync/internal/conf"
	"github.com/release-engineering/exodus-rsync/internal/walk"
)

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination mock.go -source $GOFILE

// Interface defines the public interface of this package.
type Interface interface {
	// NewClient creates and returns a new exodus-gw client with the given
	// configuration.
	NewClient(context.Context, conf.Config) (Client, error)

	// NewDryRunClient creates and returns a new exodus-gw client in dry-run
	// mode. This client replaces any write operations with stubs.
	NewDryRunClient(context.Context, conf.Config) (Client, error)
}

type impl struct{}

// Package provides the default implementation of this package's interface.
var Package Interface = impl{}

// External dependencies which may be overridden from tests.
var ext = struct {
	awsSessionProvider func(session.Options) (*session.Session, error)
}{
	session.NewSessionWithOptions,
}

// Client provides a high-level interface to the exodus-gw HTTP API.
type Client interface {
	// EnsureUploaded will process every given item for sync and ensure that the content
	// is present in the target exodus-gw environment.
	//
	// For each item, onUploaded is invoked if the item was uploaded during the call,
	// while onPresent is invoked if the item was already present prior to the call.
	//
	// In either case, returning from the callback with an error will cause EnsureUploaded
	// to stop and return the same error.
	EnsureUploaded(ctx context.Context, items []walk.SyncItem,
		onUploaded func(walk.SyncItem) error,
		onPresent func(walk.SyncItem) error,
	) error

	// NewPublish creates and returns a new publish object within exodus-gw.
	NewPublish(context.Context) (Publish, error)

	// GetPublish returns a handle to an existing publish object within exodus-gw.
	//
	// This function never fails, but it is not guaranteed that the publish object
	// is valid. If an invalid publish ID is given, an error will occur the next
	// time any write operation is attempted on the publish.
	GetPublish(string) Publish
}

// Publish represents a publish object in exodus-gw.
type Publish interface {
	// ID is the unique identifier of a publish.
	ID() string

	// AddItems will add all of the specified items onto this publish.
	// This may involve multiple requests to exodus-gw.
	AddItems(context.Context, []ItemInput) error

	// Commit will cause this publish object to become committed, making all of
	// the included content available from the CDN.
	//
	// The commit operation within exodus-gw is asynchronous. This method will
	// wait for the commit to complete fully and will return nil only if the
	// commit has succeeded.
	Commit(ctx context.Context) error
}

// Task represents a single task object within exodus-gw.
type Task interface {
	// ID is the unique ID of this task.
	ID() string

	// Await will repeatedly refresh the state of this task from exodus-gw
	// and return once the task has reached a terminal state.
	//
	// The return value will be nil if and only if the task succeeded.
	Await(context.Context) error
}
