package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

var (
	// clusterSize reports the cluster size as seen by the node.
	clusterSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "cf",
			Subsystem: "cluster",
			Name:      "clusterSize",
			Help:      "Cluster size",
		},
		[]string{"node"})

	// shardCount reports the number of shards assigned to the node.
	shardCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "cf",
			Subsystem: "cluster",
			Name:      "shardCount",
			Help:      "Number of shards handled by the local node",
		},
		[]string{"node"})

	// shardIndex is the current version of the shard index
	shardIndex = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "cf",
			Subsystem: "cluster",
			Name:      "shardIndex",
			Help:      "Current shard map version",
		},
		[]string{"node"})

	// requests show the number of requests handled by the gRPC interceptor.
	requests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "cf",
			Subsystem: "cluster",
			Name:      "requests",
			Help:      "Requests handled by node",
		},
		[]string{"node", "destination", "method"})
)

func init() {
	prometheus.MustRegister(clusterSize)
	prometheus.MustRegister(shardCount)
	prometheus.MustRegister(shardIndex)
	prometheus.MustRegister(requests)
}

// NewPrometheusSink creates a metrics sink for Prometheus. All sinks created
// by this function will write to the same sinks.
func NewPrometheusSink() Sink {
	return &prometheusSink{}
}

type prometheusSink struct {
}

func (p *prometheusSink) SetClusterSize(nodeid string, size int) {
	clusterSize.With(
		prometheus.Labels{
			"node": nodeid,
		}).Set(float64(size))
}

func (p *prometheusSink) SetShardCount(nodeid string, shards int) {
	shardCount.With(
		prometheus.Labels{
			"node": nodeid,
		}).Set(float64(shards))
}

func (p *prometheusSink) SetShardIndex(nodeid string, index uint64) {
	shardIndex.With(prometheus.Labels{
		"node": nodeid,
	}).Set(float64(index))
}

func (p *prometheusSink) LogRequest(nodeid, destination, method string) {
	requests.With(prometheus.Labels{
		"node":        nodeid,
		"destination": destination,
		"method":      method,
	}).Inc()
}
