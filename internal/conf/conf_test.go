package conf

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCandidatePaths(t *testing.T) {
	// It should not crash
	paths := candidatePaths()

	assert.Len(t, paths, 3)
	assert.Contains(t, paths, "exodus-rsync.conf")
}
