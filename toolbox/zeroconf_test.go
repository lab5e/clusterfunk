package toolbox

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestZeroConf(t *testing.T) {
	assert := require.New(t)

	zr := NewZeroconfRegistry("test-cluster")
	assert.NoError(zr.Register("some-node", "0", 9999))

	assert.Error(zr.Register("some-node", "0", 9998), "Should not be able to register two endpoints")

	assert.NoError(zr.Register("some-node", "1", 9999))

	defer zr.Shutdown()

	results, err := zr.Resolve("some-node", 550*time.Millisecond)
	assert.NoError(err)
	assert.NotNil(results)

	res, err := zr.ResolveFirst("some-node", 250*time.Millisecond)
	assert.NoError(err)
	ip, err := FindPublicIPv4()
	assert.NoError(err)
	assert.Equal(res, fmt.Sprintf("%s:9999", ip))
}
