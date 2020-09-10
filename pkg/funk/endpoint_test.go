package funk

import (
	"fmt"
	"testing"

	"github.com/lab5e/gotoolbox/netutils"
	"github.com/stretchr/testify/require"
)

func TestToPublicEndpoint(t *testing.T) {
	assert := require.New(t)

	ip, err := netutils.FindPublicIPv4()
	assert.NoError(err)

	s, err := ToPublicEndpoint(":9999")
	assert.NoError(err)
	assert.Equal(fmt.Sprintf("%s:9999", ip.String()), s)

	s, err = ToPublicEndpoint("0.0.0.0:9999")
	assert.NoError(err)
	assert.Equal(fmt.Sprintf("%s:9999", ip.String()), s)

	s, err = ToPublicEndpoint("127.0.0.1:9999")
	assert.NoError(err)
	assert.Equal("127.0.0.1:9999", s)

	s, err = ToPublicEndpoint("1.2.3.4:9999")
	assert.NoError(err)
	assert.Equal("1.2.3.4:9999", s)

	_, err = ToPublicEndpoint("something")
	assert.Error(err)
}
