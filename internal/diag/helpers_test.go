package diag

import (
	"testing"

	gomock "go.uber.org/mock/gomock"
)

func MockController(t *testing.T) *gomock.Controller {
	oldExt := ext
	t.Cleanup(func() { ext = oldExt })

	return gomock.NewController(t)
}
