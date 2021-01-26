package metrics

// Sink is the metrics sink for the cluster. Implement this interface to write
// to other kinds of systems.
type Sink interface {
	SetClusterSize(size int)
	SetShardCount(shards int)
	SetLogIndex(index uint64)
	LogRequest(destination, method string)
}

// The list of supported metrics
const (
	PrometheusSink = "prometheus"
	NoSink         = "none"
)

// NewSinkFromString returns a named sink
func NewSinkFromString(name string, nodeid string) Sink {
	switch name {
	case "prometheus":
		return NewPrometheusSink(nodeid)
	default:
		return NewBlackHoleSink()
	}
}
