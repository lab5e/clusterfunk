package funk

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// classic EMA results below
/*
var emaTable [][]float64 = [][]float64{
	[]float64{100.0, 100.0, 0.0},
	[]float64{100.0, 100.0, 0.0},
	[]float64{100.0, 100.0, 0.0},
	[]float64{50.0, 80.0, 0.0},
	[]float64{50.0, 70.0, 0.0},
	[]float64{50.0, 64.286, 0.5},
	[]float64{50.0, 60.714, 0.5},
	[]float64{50.0, 58.333, 0.5},
}*/

var emaTable = [][]float64{
	[]float64{100.0, 100.0, 0.0},
	[]float64{100.0, 100.0, 0.0},
	[]float64{100.0, 100.0, 0.0},
	[]float64{50, 97.5, 0.0},
	[]float64{50, 95.125, 0.0},
	[]float64{50, 92.869, 0.005},
	[]float64{50, 90.725, 0.005},
	[]float64{50, 88.689, 0.005},
	[]float64{50, 86.755, 0.005},
	[]float64{50, 84.917, 0.005},
	[]float64{50, 83.171, 0.005},
	[]float64{50, 81.512, 0.005},
}

func TestEMA(t *testing.T) {
	assert := require.New(t)

	ema := newEMACalculator(100)

	for _, v := range emaTable {
		avg := ema.Add(v[0])
		assert.InDelta(v[1], avg, v[2])
	}
}
