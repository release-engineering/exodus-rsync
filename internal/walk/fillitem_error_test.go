package walk

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
)

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination direntry_mock.go io/fs DirEntry

func TestFillItemInfoError(t *testing.T) {
	ctrl := gomock.NewController(t)

	entry := NewMockDirEntry(ctrl)

	// make it fail
	entry.EXPECT().Info().Return(nil, fmt.Errorf("simulated error"))

	item := walkItem{}
	item.SrcPath = "some/file"
	item.Entry = entry
	c := make(chan syncItemPrivate)
	err := fillItem(context.TODO(), c, item)

	// It should propagate the error.
	if fmt.Sprint(err) != "get file info for some/file: simulated error" {
		t.Errorf("did not get expected error, err = %v", err)
	}
}
