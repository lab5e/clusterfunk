package cluster

import (
	"encoding"
	"time"
)

// EventType is the event type for cluster events
type EventType int

const (
	// NodeAdded events are triggered when one or more nodes are added to the
	// cluster
	NodeAdded EventType = iota
	// NodeRemoved events are triggered when one or more nodes are removed from
	// the cluster
	NodeRemoved
	// NodeRetired events are trieggered when one or more nodes are retired from
	// the cluster
	NodeRetired
	// LocalNodeStopped is triggered right after the cluster is stopped. Event
	// channels will be closed and the current node will shut down.
	LocalNodeStopped
	// LeaderLost is triggered when the current leader goes away
	LeaderLost
	// LeaderChanged is triggered when a new leader is elected
	LeaderChanged
)

// Event is the interface for cluster events that are triggered
type Event interface {
	// Type is the type of cluster event
	Type() EventType
	// Nodes returns the nodes affected by the event.
	Nodes() []Node
}

// RedistributeFunc is the callback function to redistribute the
// shards across nodes. The cluster client must implement this.
type RedistributeFunc func(addedNodes []Node, removedNodes []Node) []byte

// State is the current state of the cluster.
type State int

const (
	// Operational is the ordinary state for the cluster. Nodes are serving and
	// requests are (probably) processed as they should.
	Operational State = iota
	// Startup is the initial state of the cluster. This is the default
	// state for new cluster instances before the Raft cluster is boostrapped
	// and/or joined.
	Startup
	// Unavailable is a state the cluster is in while there's a leader election
	// ongoing. The cluster might stay in this state for a very long time if
	// there's no quorum.
	Unavailable
	// Resharding is a state where the leader is currently resharding and
	// distributing the results to the nodes.
	Resharding
)

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

	// State is the current cluster state
	State() State

	// WaitForState blocks until the cluster reaches the desired state. If the
	// timeout is set to 0 the call wil block forever. If the desired state isn't
	// reached within the timeout an error is returned.
	WaitForState(state State, timeout time.Duration) error

	// Nodes return a list of the active nodes in the cluster
	Nodes() []Node

	// Events returns an event channel for the cluster. The channel will
	// be closed when the cluster is stopped. Events are for information only
	Events() <-chan Event

	// AddLocalEndpoint registers an endpoint on the local node
	// TOOD(stalehd) add LocalNode() + AddEndpoint method to node
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

const (
	// StateLeader is the state reported in the Serf cluster tags
	StateLeader = "leader"
	// StateFollower is the state reported when the node is in the follower state
	StateFollower = "follower"
	// StateNone is the state reported when the node is in an unknown (raft) state
	StateNone = "none"
)

// ShardMapper is the cluster's view of the Shard Manager type. It only concerns itself with
// adding and removing nodes plus
type ShardMapper interface {

	// AddNode adds a new bucket. The returned shard operations are required
	// to balance the shards across the buckets in the cluster. If the bucket
	// already exists nil is returned. Performance critical since this is
	// used when nodes join or leave the cluster.
	AddNode(nodeID string) error

	// RemoveNode removes a bucket from the cluster. The returned shard operations
	// are required to balance the shards across the buckets in the cluster.
	// Performance critical since this is used when nodes join or leave the cluster.
	RemoveNode(nodeID string) error

	encoding.BinaryMarshaler
}
