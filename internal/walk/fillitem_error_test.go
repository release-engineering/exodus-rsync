package walk

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path"
	"testing"

	"github.com/golang/mock/gomock"
)

//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination fs_mock.go io/fs DirEntry,FileInfo

func TestFillItemInfoError(t *testing.T) {
	ctrl := gomock.NewController(t)

	entry := NewMockDirEntry(ctrl)

	// make it fail
	entry.EXPECT().Info().Return(nil, fmt.Errorf("simulated error"))

	item := walkItem{}
	item.SrcPath = "some/file"
	item.Entry = entry
	c := make(chan syncItemPrivate)
	err := fillItem(context.TODO(), c, item, false)

	// It should propagate the error.
	if fmt.Sprint(err) != "get file info for some/file: simulated error" {
		t.Errorf("did not get expected error, err = %v", err)
	}
}

func TestFillItemReadlinkError(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	src := path.Clean(wd + "/../../test/data/rand1")
	entry := NewMockDirEntry(gomock.NewController(t))
	info := NewMockFileInfo(gomock.NewController(t))

	info.EXPECT().Mode().Return(fs.ModeSymlink)
	entry.EXPECT().Info().Return(info, nil)
	entry.EXPECT().Type().Return(fs.ModeSymlink)

	item := walkItem{SrcPath: src, Entry: entry}

	c := make(chan syncItemPrivate)
	err = fillItem(context.TODO(), c, item, true)

	// It should propagate the error.
	if fmt.Sprint(err) != "readlink "+src+": invalid argument" {
		t.Errorf("did not get expected error, err = %v", err)
	}
}
