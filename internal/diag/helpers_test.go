package diag

import (
	"testing"

	gomock "github.com/golang/mock/gomock"
)

func MockController(t *testing.T) *gomock.Controller {
	oldExt := ext
	t.Cleanup(func() { ext = oldExt })

	return gomock.NewController(t)
}
