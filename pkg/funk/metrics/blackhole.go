package metrics

// NewBlackHoleSink creates a metrics sink that discards all metrics
func NewBlackHoleSink() Sink {
	return &blackHoleSink{}
}

type blackHoleSink struct {
}

func (b *blackHoleSink) SetClusterSize(size int) {
	// do nothing
}

func (b *blackHoleSink) SetShardCount(shards int) {
	// do nothing
}

func (b *blackHoleSink) SetLogIndex(index uint64) {
	// do nothing
}

func (b *blackHoleSink) LogRequest(destination, method string) {
	// do nothing
}
