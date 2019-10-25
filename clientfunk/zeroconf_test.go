package clientfunk

import (
	"testing"

	"github.com/stalehd/clusterfunk/funk"
	"github.com/stalehd/clusterfunk/toolbox"
	"github.com/stretchr/testify/require"
)

func TestManagementZCLookup(t *testing.T) {
	assert := require.New(t)
	reg := toolbox.NewZeroconfRegistry("test")
	assert.NotNil(reg)

	assert.NoError(reg.Register(funk.ZeroconfManagementKind, "1", 1234))
	defer reg.Shutdown()

	s, err := ZeroconfManagementLookup("test")
	assert.NoError(err)
	assert.NotEmpty(s)
}
