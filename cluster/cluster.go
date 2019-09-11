package cluster

// Cluster is a wrapper for the Serf and Raft libraries. It will handle typical
// cluster operations.
type Cluster interface {
	// Start launches the cluster, ie joins a Serf cluster and announces its
	// presence
	Start() error

	// Stop stops the cluster
	Stop()

	// Nodes return a list of the active nodes in the cluster
	Nodes() []Node

	// LocalNode returns the local node
	LocalNode() Node

	// AddEndpoint adds a local endpoint. The endpoints will be distributed
	// to the other nodes in the cluster.
	AddLocalEndpoint(name, endpoint string)
}

// NodeState is the enumeration of different states a node can be in.
type NodeState int

// This the node states. The
// initial state is ReadyToJoin, ie the node is ready to join the Raft cluster
// and is waiting for the leader to add it to the cluster. When joined its state
// is set to Empty once it has built the shard map locally, ie it can proxy
// requests to other nodes but have no shards on their own. Nodes that are
// non-voters will will stay in the Empty state.
// the other nodes allocated shards. When the node is in the Allocated state
// it has allocated a set of shards but isn't serving. When in Serving state
// it is serving requests for its allocated shards. When the node is in the
// Retiring state it is still serving requests but is preparing to be drained
// and stopped once the leader removes the shards from it. When in Draining mode
// it will no longer accept any new requests and serve the remaining requests
// before stopping. The leader will remove the node from the Raft cluster and
// the node will remove all of its endpoint information before it goes into
// Draining mode. When it has finished draining it may shut down whenever
// ready.
//
//            +-------+
//            | Start |
//            +---|---+
//                |
//                |
//                |
//                |
//       +-----------------+           +----------------+
//       |   ReadyToJoin   -------------    Empty       |
//       +-----------------+           +--------|-------+
//                                              |
//                                              |
//                                              |
//       +-----------------+           +--------|-------+
//       |  Serving        -------------  Allocated     |
//       +--------|--------+           +----------------+
//                |
//                |
//                |
//       +--------|--------+
//       |  Retiring       |
//       +--------|--------+
//                |
//                |
//                |
//       +--------|--------+
//       | Draining        |
//       +--------|--------+
//                |
//                |
//                |
//            +---|--+
//            | End  |
//            +------+
const (
	ReadyToJoin NodeState = iota
	Empty
	Allocated
	Serving
	Retiring
	Draining
)

// Node is one of the processes in the cluster. Note that this might be
// a process.
type Node interface {
	// ID returns the node ID. This is an unique string in the cluster
	ID() string

	// Shards returns the shards the node is currently owning. Note that the
	// ownership might change at any time.
	Shards() []Shard

	// SerfEndpoint returns the node's Serf endpoint
	SerfEndpoint() string

	// RaftEndpoint returns the node's Raft endpoint. If the node isn't a
	// member of the Raft cluster the string is empty
	RaftEndpoint() string

	// Voter returns true if this is a voting member of the cluster. If the
	// node isn't a member of the Raft cluster the string is empty
	Voter() bool

	// Leader returns true if the node is the leader of the cluster
	Leader() bool

	// GetEndpoint returns the named endpoint for the node
	GetEndpoint(name string) (string, error)

	// State returns the state of the node
	State() NodeState
}

// The following are internal tags and values for nodes
const (
	raftEndpoint    = "ep.raft"
	raftNodeID      = "raft.nodeid"
	nodeType        = "kind"
	voterKind       = "member"
	nonvoterKind    = "nonvoter"
	cheerleaderKind = "cheerleader" // Cheerleaders are special kind of voters that exists solely to keep the
)

// The following is a list of well-known endpoints on nodes
const (
	MetricsEndpoint    = "ep.metrics"    // MetricsEndpoint is the metrics endpoint
	HTTPEndpoint       = "ep.http"       // HTTPEndpoint is the HTTP endpoint
	ManagementEndpoint = "ep.management" // ManagementEndpoint is gRPC endpoint for management
)

const (
	// To simplify local development clusters we use mDNS to auto-discover clusters.
	// If the join parameter is set that will be used. Multiple development clusters
	// on the same subnet won't be possible but that's OK.
	mDNSBootstrap = "clattering.local"
)
