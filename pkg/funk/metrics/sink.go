package metrics

import "time"

// ClusterInfo is the interface to expose cluster info
type ClusterInfo interface {
	Created() time.Time
	NodeID() string
}

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
func NewSinkFromString(name string, cluster ClusterInfo) Sink {
	switch name {
	case "prometheus":
		return NewPrometheusSink(cluster)
	default:
		return NewBlackHoleSink()
	}
}
