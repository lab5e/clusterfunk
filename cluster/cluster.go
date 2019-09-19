package cluster

// Cluster is a wrapper for the Serf and Raft libraries. It will handle typical
// cluster operations.
type Cluster interface {

	// Name returns the cluster's name
	Name() string

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

const (
	// Initializing is the initial state of nodes. At this point the nodes
	// are starting up. Possible states after this: Ready or Terminating
	Initializing NodeState = iota
	// Ready is the ready states for nodes when they have finished initializing
	// and is ready to join the cluster. Posslbe states after this: Empty or Terminating
	Ready
	// Empty is the initial state for nodes in the cluster when they join. At
	// this point the nodes have joined the cluster but have no shards allocated
	// to them. Possible states after this: Reorganizing or Terminating
	Empty

	// Reorganizing is the state the nodes enter when they are receiving shard
	// allocations from the leader node. Requests are halted until they have
	// received a new set of shards from the leader and acknowledged the shards
	// Possible states after this: Allocated or Terminating
	Reorganizing

	// Allocated state is the state following Reorganizing when the nodes have
	// allocated shards, acknowledged and is waiting for the go-head signal from
	// the leader. Possible states: Reorganizing, Terminating or Serving
	Allocated

	// Serving state is the ordinary operational state for nodes. They have a set
	// of shards they handle and are serving requests. Possible states after this
	// is Reorganizing, Draining
	Serving

	// Draining state is when the node will stop serving requests. The Draining state
	// will be set when the leader is reorganizing the shards. Once the cluster
	// has finished distributing shards the drainging node will send the remaining
	// requests to the other members of the cluster and then enter the Terminating state.
	Draining

	// Terminating is the final state for nodes. After the Draining phase
	Terminating
)

// NodeKind is the different kinds of nodes
type NodeKind int

const (
	// Leader is the leader of the cluster and the winner of the last leader election.
	Leader NodeKind = iota
	// Voter is the regular voting node in the cluster. It can participate in leader
	// elections and may even be a *leader* one day.
	Voter
	// Nonvoter is a cluster member that won't participate in leader elections.
	Nonvoter
	// Nonmember is a node that isn't a part of the cluster itself, just the swarm.
	Nonmember
)

// Node is one of the processes in the cluster. Note that this might be
// a process.
type Node interface {
	// ID returns the node ID. This is an unique string in the cluster
	ID() string

	// Shards returns the shards the node is currently owning. Note that the
	// ownership might change at any time.
	Shards() []Shard

	// Voter returns true if this is a voting member of the cluster. If the
	// node isn't a member of the Raft cluster the string is empty
	Voter() bool

	// Leader returns true if the node is the leader of the cluster
	Leader() bool

	// Endpoints returns a list of endpoints
	Endpoints() []string

	// GetEndpoint returns the named endpoint for the node
	GetEndpoint(name string) (string, error)

	// State returns the state of the node
	State() NodeState
}

// The following are internal tags and values for nodes
const (
	clusterEndpointPrefix = "ep."
	RaftNodeID            = "raft.nodeid"
	NodeType              = "kind"
	VoterKind             = "member"
	NonvoterKind          = "nonvoter"
	NodeRaftState         = "raft.state"
)

// The following is a list of well-known endpoints on nodes
const (
	//These are
	//MetricsEndpoint    = "ep.metrics"    // MetricsEndpoint is the metrics endpoint
	//HTTPEndpoint       = "ep.http"       // HTTPEndpoint is the HTTP endpoint
	SerfEndpoint       = "ep.serf"
	RaftEndpoint       = "ep.raft"
	ManagementEndpoint = "ep.management" // ManagementEndpoint is gRPC endpoint for management
)
