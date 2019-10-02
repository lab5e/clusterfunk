package cluster

import "fmt"

// TODO: get rid of this (user supplied)
const numberOfShards = 10000

// NodeState is the enumeration of different states a node can be in.
type NodeState int32

// These are the (local) states the cluster node can be in
const (
	Invalid     NodeState = iota // Invalid or unknown state
	Joining                      // Joining the cluster
	Operational                  // Operational, normal operation
	Voting                       // Leader election in progress
	Resharding                   // Leader is elected, resharding in progress
	Starting                     // Starting the node
	Stopping                     // Stopping the node
)

func (n NodeState) String() string {
	switch n {
	case Invalid:
		return "Invalid"
	case Joining:
		return "Joining"
	case Operational:
		return "Operational"
	case Voting:
		return "Voting"
	case Resharding:
		return "Resharding"
	case Starting:
		return "Starting"
	case Stopping:
		return "Stopping"
	default:
		panic(fmt.Sprintf("Unknown state: %d", n))
	}
}

// NodeRole is the roles the node can have in the cluster
type NodeRole int32

// These are the roles the node might have in the cluster
const (
	Unknown   NodeRole = iota // Uknown state
	Follower                  // A follower in a cluster
	Leader                    // The current leader node
	NonVoter                  // Non voting role in cluster
	NonMember                 // NonMember nodes are part of the Serf cluster but not the Raft cluste
)

func (n NodeRole) String() string {
	switch n {
	case Unknown:
		return "Unknown"
	case Follower:
		return "Follower"
	case Leader:
		return "Leader"
	case NonMember:
		return "NonMember"
	case NonVoter:
		return "NonVoter"
	default:
		panic(fmt.Sprintf("Unknown role: %d", n))
	}
}

const (
	// SerfStatusKey is the key for the serf status
	SerfStatusKey = "serf.status"
)

// Event is the interface for cluster events that are triggered
type Event struct {
	// State is the current cluster state
	LocalState NodeState
}

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

	// Role is the current role of the node
	Role() NodeRole

	// State is the current cluster state
	LocalState() NodeState

	// Events returns an event channel for the cluster. The channel will
	// be closed when the cluster is stopped. Events are for information only
	Events() <-chan Event
}

// The following are internal tags and values for nodes
const (
	clusterEndpointPrefix = "ep."
)

// The following is a list of well-known endpoints on nodes
const (
	//These are
	//MetricsEndpoint    = "ep.metrics"    // MetricsEndpoint is the metrics endpoint
	//HTTPEndpoint       = "ep.http"       // HTTPEndpoint is the HTTP endpoint
	SerfEndpoint       = "ep.serf"
	RaftEndpoint       = "ep.raft"
	LeaderEndpoint     = "ep.leader"
	ManagementEndpoint = "ep.management" //  gRPC endpoint for management
)
