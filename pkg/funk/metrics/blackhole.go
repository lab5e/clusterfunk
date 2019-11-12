package metrics

// NewBlackHoleSink creates a metrics sink that discards all metrics
func NewBlackHoleSink() Sink {
	return &blackHoleSink{}
}

type blackHoleSink struct {
}

func (b *blackHoleSink) SetClusterSize(nodeid string, size int) {
	// do nothing
}

func (b *blackHoleSink) SetShardCount(nodeid string, shards int) {
	// do nothing
}

func (b *blackHoleSink) SetShardIndex(nodeid string, index uint64) {
	// do nothing
}

func (b *blackHoleSink) LogRequest(nodeid, destination, method string) {
	// do nothing
}
