package snapstest

import (
	"testing"

	"github.com/gkampitakis/ciinfo"
	"github.com/stretchr/testify/assert"
)

func TestUpdateScripts(t *testing.T) {
	t.Setenv("UPDATE_SNAPS", "true")
	assert.Equal(t, !ciinfo.IsCI, UpdateScripts(), "эталоны переписываются не по правилу go-snaps")
	t.Setenv("UPDATE_SNAPS", "false")
	assert.False(t, UpdateScripts(), "эталоны переписываются без UPDATE_SNAPS")
}
