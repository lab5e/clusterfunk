package cluster

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogMessage(t *testing.T) {
	assert := require.New(t)

	const testData = "hello there"
	lm := NewLogMessage(ProposedShardMap, []byte(testData))
	assert.Equal(lm.MessageType, ProposedShardMap)
	assert.Len(lm.Data, len(testData), "Length is %d, not %d", len(lm.Data), len(testData))

	buf, err := lm.MarshalBinary()
	assert.NoError(err)

	lm2 := NewLogMessage(0, nil)
	assert.NoError(lm2.UnmarshalBinary(buf))

	assert.Equal(lm2.MessageType, ProposedShardMap)
	assert.Len(lm2.Data, len(testData), "Length of lm2 is %d, not %d", len(lm2.Data), len(testData))
	assert.Equal(testData, string(lm2.Data))
}
