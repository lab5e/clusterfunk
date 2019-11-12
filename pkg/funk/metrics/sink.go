package metrics

// Sink is the metrics sink for the cluster. Implement this interface to write
// to other kinds of systems.
type Sink interface {
	SetClusterSize(nodeid string, size int)
	SetShardCount(nodeid string, shards int)
	SetShardIndex(nodeid string, index uint64)
	LogRequest(nodeid, destination, method string)
}

// NewSinkFromString returns a named sink
func NewSinkFromString(name string) Sink {
	switch name {
	case "prometheus":
		return NewPrometheusSink()
	default:
		return NewBlackHoleSink()
	}
}
