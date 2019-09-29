package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogMessage(t *testing.T) {
	assert := require.New(t)

	const testData = "hello there"
	const endpoint = "1.2.3.4:1234"
	lm := NewLogMessage(ProposedShardMap, endpoint, []byte(testData))
	assert.Equal(lm.MessageType, ProposedShardMap)
	assert.Equal(endpoint, lm.AckEndpoint)
	assert.Len(lm.Data, len(testData), "Length is %d, not %d", len(lm.Data), len(testData))

	buf, err := lm.MarshalBinary()
	assert.NoError(err)

	lm2 := NewLogMessage(0, "", nil)
	assert.NoError(lm2.UnmarshalBinary(buf))

	assert.Equal(lm2.MessageType, ProposedShardMap)
	assert.Equal(lm2.AckEndpoint, lm.AckEndpoint)
	assert.Len(lm2.Data, len(testData), "Length of lm2 is %d, not %d", len(lm2.Data), len(testData))
	assert.Equal(testData, string(lm2.Data))
}
