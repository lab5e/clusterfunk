package funk

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"fmt"
	"time"
)

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

	// clusterCreated is the key for the created tag. It's set by the node
	// that bootstraps the cluster. Technically it's writable by all nodes in
	// the cluster but...
	clusterCreated = "cf.created"
)

// Event is the interface for cluster events that are triggered. The events are
// triggered by changes in the node state. The Role field is informational.
// When the cluster is in state Operational the shard map contains the
// current shard mapping. If the State field is different from Operational
// the shard map may contain an invalid or outdated mapping.
type Event struct {
	State NodeState // State is the state of the local cluster node
	Role  NodeRole  // Role is the role (leader, follower...) of the local cluster node
}

// Cluster is a wrapper for the Serf and Raft libraries. It will handle typical
// cluster operations.
type Cluster interface {
	// NodeID is the local cluster node's ID
	NodeID() string

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
	State() NodeState

	// Events returns an event channel for the cluster. The channel will
	// be closed when the cluster is stopped. Events are for information only
	Events() <-chan Event

	// SetEndpoint registers an endpoint on the local node
	SetEndpoint(name string, endpoint string)

	// Leader returns the node ID of the leader in the cluster. If no leader
	// is currently elected it will return a blank string.
	Leader() string

	// Nodes returns a list of the node IDs of each member
	Nodes() []string

	// GetEndpoint returns the endpoint for a particular node. If the node
	// or endpoint isn't found it will return a blank. The endpoint is
	// retrieved from the Serf cluster. Note that there's no guarantee
	// that the node will be responding on that endpoint.
	GetEndpoint(nodeID string, endpointName string) string

	// NewObserver returns a new endpoint observer for endpoints in and around
	// the cluster
	NewObserver() EndpointObserver

	// Created returns the time the cluster was created. This is set when the
	// cluster is bootstrapped.
	Created() time.Time
}

// The following are internal tags and values for nodes
const (
	EndpointPrefix = "ep."
)

const (
	// ZeroconfSerfKind is the type used to register serf endpoints in zeroconf.
	ZeroconfSerfKind = "serf"
	// ZeroconfManagementKind is the type used to register management endpoints
	// in zeroconf.
	ZeroconfManagementKind = "mgmt"

	// ZeroconfServiceKind is the type used to register (non-cluster) services
	ZeroconfServiceKind = "svc"
)

// The following is a list of well-known endpoints on nodes
const (
	//These are
	SerfEndpoint           = "ep.serf"
	RaftEndpoint           = "ep.raft"
	ManagementEndpoint     = "ep.clusterfunk.management" //  gRPC endpoint for management
	LivenessEndpoint       = "ep.liveness"
	SerfServiceName        = "meta.serviceName"
	MonitoringEndpointName = "ep.monitoring" // Typically Prometheus endpoint with /metrics and /healthz
)
