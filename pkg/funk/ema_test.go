package funk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEMA(t *testing.T) {
	assert := require.New(t)

	ema := newEMACalculator(100)
	assert.Equal(100.0, ema.Add(100.0))
	assert.Equal(100.0, ema.Add(100.0))
	assert.Equal(100.0, ema.Add(100.0))
	assert.Equal(80.0, ema.Add(50.0))
	assert.Equal(70.0, ema.Add(50.0))
	assert.InDelta(64.286, ema.Add(50.0), 0.5)
	assert.InDelta(60.714, ema.Add(50.0), 0.5)
	assert.InDelta(58.333, ema.Add(50.0), 0.5)
}
