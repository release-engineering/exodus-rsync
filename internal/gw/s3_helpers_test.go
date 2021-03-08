package gw

import (
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/s3"
)

type blobMap map[string][]error

type fakeS3 struct {
	t  *testing.T
	mu sync.Mutex

	blobs blobMap
}

func newFakeS3(t *testing.T, client *client) *fakeS3 {
	out := fakeS3{t: t, blobs: make(blobMap)}

	out.install(client)

	return &out
}

func (f *fakeS3) reset() {
	f.blobs = make(blobMap)
}

func (f *fakeS3) install(client *client) {
	handlers := &client.s3.Client.Handlers

	// Clear all default handlers, preventing requests from
	// actually being sent or parsed.
	handlers.Clear()

	// This handler is invoked to unpack the response from AWS into
	// an output object and is an appropriate place to hook in our own logic.
	handlers.Unmarshal.PushBack(f.unmarshal)
}

func (f *fakeS3) unmarshal(r *request.Request) {
	switch v := r.Params.(type) {
	case *s3.HeadObjectInput:
		f.headObject(r, v)
	case *s3.PutObjectInput:
		f.putObject(r, v)
	default:
		r.Error = awserr.New("NotImplemented", "not supported by fake S3", nil)
	}
}

func (f *fakeS3) headObject(r *request.Request, input *s3.HeadObjectInput) {
	f.mu.Lock()
	defer f.mu.Unlock()

	errors, haveBlob := f.blobs[*input.Key]
	if !haveBlob {
		r.Error = awserr.New("NotFound", "object not found", nil)
		return
	}

	if errors == nil || len(errors) == 0 {
		// No specific instructions for this blob, don't need to do anything.
		return
	}

	// Pop the next error.
	err := errors[0]
	f.blobs[*input.Key] = errors[1:]

	if err != nil {
		r.Error = err
	}
}

func (f *fakeS3) putObject(r *request.Request, input *s3.PutObjectInput) {
	f.mu.Lock()
	defer f.mu.Unlock()

	errors, haveBlob := f.blobs[*input.Key]
	if !haveBlob {
		// Mark that we have this blob, and don't return any errors for it.
		f.blobs[*input.Key] = make([]error, 0)
	}

	if errors == nil || len(errors) == 0 {
		// No specific instructions for this blob, write succeeds
		return
	}

	// Pop the next error.
	err := errors[0]
	f.blobs[*input.Key] = errors[1:]

	if err != nil {
		r.Error = err
	}
}

func newClientWithFakeS3(t *testing.T) (*client, *fakeS3) {
	cfg := testConfig(t)

	iface, err := Package.NewClient(cfg)
	if err != nil {
		t.Fatal("creating client:", err)
	}

	out := iface.(*client)

	return out, newFakeS3(t, out)
}
