package funk

// ServiceParameters is configuration parameters for service nodes, ie nodes that
// aren't part of the cluster but provides discoverable services to the cluster
// (and cluster clients)
type ServiceParameters struct {
	Name     string         `kong:"help='Cluster name',default='clusterfunk'"`
	Verbose  bool           `kong:"help='Verbose logging for Serf and Raft'"`
	ZeroConf bool           `kong:"help='Zero-conf startup',default='true'"`
	Serf     SerfParameters `kong:"embed,prefix='serf-'"`
}
