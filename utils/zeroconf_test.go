package utils

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestZeroConf(t *testing.T) {
	assert := require.New(t)

	zr := NewZeroconfRegistry("test-cluster")
	assert.NoError(zr.Register("some-node", 9999))

	assert.Error(zr.Register("another-node", 9998), "Should not be able to register two endpoints")

	defer zr.Shutdown()

	results, err := zr.Resolve(250 * time.Millisecond)
	assert.NoError(err)
	assert.True(len(results) > 1)

	res, err := zr.ResolveFirst(250 * time.Millisecond)
	assert.NoError(err)
	ip, err := FindPublicIPv4()
	assert.NoError(err)
	assert.Equal(res, fmt.Sprintf("%s:9999", ip))
}
