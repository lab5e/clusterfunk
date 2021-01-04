package funk

// emaCalculator calculates the exponential moving average (ema) for a series
// of values. It's relatively simple to implement so there's no need for
// an additional dependency just for this. The type is not thread safe.
type emaCalculator struct {
	m     float64
	ema   float64
	count int
}

// newEMACalculator creates a new EMA calculator with the initial value
// set to the specified value. A value of 0 can be used but it might need a
// few samples to converge into an usable value.
func newEMACalculator(initial float64) *emaCalculator {
	return &emaCalculator{
		m:     0.05,
		ema:   initial,
		count: 0,
	}
}

// Add adds a new sample to the calculator and calculates a new moving average
// for the number series. The new moving average is returned.
func (e *emaCalculator) Add(x float64) float64 {

	// Note for future implementations. The EMA can be sluggish to react to
	// abrupt and permanent changes, ie if the latency goes from 10ms to 100ms
	// and stays there so a fixed weight (m) can be used instead to calculate
	// the average. A value of f.e. 0.1 or 0.15 will converge much quicker
	// to the actual average and not remember historical values. This is
	// usually not an issue since the latency is rarely changing for the nodes
	// but if f.e. a slower switch is introduced between nodes or the latency
	// changes for another reason it might not behave as desired.
	//
	// Uncomment the following lines to get a classic EMA behaviour:
	//
	//e.count++
	//e.m = 2.0 / (float64(e.count) + 1.0)

	e.ema = (x-e.ema)*e.m + e.ema
	return e.ema
}

func (e *emaCalculator) Average() float64 {
	return e.ema
}
