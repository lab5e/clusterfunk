package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

var ()

var oneTimeRegister sync.Once

type prometheusSink struct {
	clusterSize *prometheus.GaugeVec
	shardCount  *prometheus.GaugeVec
	logIndex    *prometheus.GaugeVec
	requests    *prometheus.CounterVec
}

var promMetrics *prometheusSink

// NewPrometheusSink creates a metrics sink for Prometheus. All sinks created
// by this function will write to the same sinks.
func NewPrometheusSink(nodeid string) Sink {
	// This registers the metrics for the first time but not for subsequent
	// calls. Since this is a one-time operation it will also work for unit
	// tests but the registration might be stale or incorrect.
	// Registering via a simple init() function also works but it pollutes
	// the package namespace with symbols.
	oneTimeRegister.Do(func() {
		promMetrics = &prometheusSink{
			// clusterSize reports the cluster size as seen by the node.
			clusterSize: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: "cf",
					Subsystem: "cluster",
					Name:      "clusterSize",
					Help:      "Cluster size",
					ConstLabels: prometheus.Labels{
						"node": nodeid,
					},
				},
				[]string{}),
			// shardCount reports the number of shards assigned to the node.
			shardCount: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: "cf",
					Subsystem: "cluster",
					Name:      "shardCount",
					Help:      "Number of shards handled by the local node",
					ConstLabels: prometheus.Labels{
						"node": nodeid,
					},
				},
				[]string{}),
			// shardIndex is the current version of the shard index
			logIndex: prometheus.NewGaugeVec(
				prometheus.GaugeOpts{
					Namespace: "cf",
					Subsystem: "cluster",
					Name:      "logIndex",
					Help:      "Replicated log index",
					ConstLabels: prometheus.Labels{
						"node": nodeid,
					},
				},
				[]string{}),
			// requests show the number of requests handled by the gRPC interceptor.

			requests: prometheus.NewCounterVec(
				prometheus.CounterOpts{
					Namespace: "cf",
					Subsystem: "cluster",
					Name:      "requests",
					Help:      "Requests handled by node",
					ConstLabels: prometheus.Labels{
						"node": nodeid,
					},
				},
				[]string{"destination", "method"}),
		}
		prometheus.MustRegister(promMetrics.clusterSize)
		prometheus.MustRegister(promMetrics.shardCount)
		prometheus.MustRegister(promMetrics.logIndex)
		prometheus.MustRegister(promMetrics.requests)
	})
	return promMetrics
}

func (p *prometheusSink) SetClusterSize(size int) {
	p.clusterSize.With(prometheus.Labels{}).Set(float64(size))
}

func (p *prometheusSink) SetShardCount(shards int) {
	p.shardCount.With(prometheus.Labels{}).Set(float64(shards))
}

func (p *prometheusSink) SetLogIndex(index uint64) {
	p.logIndex.With(prometheus.Labels{}).Set(float64(index))
}

func (p *prometheusSink) LogRequest(destination, method string) {
	p.requests.With(prometheus.Labels{
		"destination": destination,
		"method":      method,
	}).Inc()
}
