package walk

import (
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
)

// Mock hash.Hash.
//go:generate go run -modfile ../../go.tools.mod github.com/golang/mock/mockgen -package $GOPACKAGE -destination hash_mock.go hash Hash

func TestHashError(t *testing.T) {
	ctrl := gomock.NewController(t)

	mockHash := NewMockHash(ctrl)

	// make it fail
	mockHash.EXPECT().Write(gomock.Any()).Return(0, fmt.Errorf("simulated error"))

	_, err := fileHash("walk.go", mockHash)

	// It should propagate the error.
	if fmt.Sprint(err) != "simulated error" {
		t.Errorf("did not get expected error, err = %v", err)
	}
}
