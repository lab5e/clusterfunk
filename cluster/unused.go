package cluster

import "errors"

// This is for unused types and declarations
// -----------------------------------------------------------------------------

// RedistributeFunc is the callback function to redistribute the
// shards across nodes. The cluster client must implement this.
type xRedistributeFunc func(addedNodes []Node, removedNodes []Node) []byte

type xNodeState int

const (
	// Initializing is the initial state of nodes. At this point the nodes
	// are starting up. Possible states after this: Ready or Terminating
	Initializing xNodeState = iota
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

// ... or those that *should* be unused.
// -----------------------------------------------------------------------------

// --- temp methods below

// RaftNodeTemp is ... a temp struct
type RaftNodeTemp struct {
	ID     string
	State  string
	Leader bool
}

// MemberList returns a list of nodes in the raft cluster
func (r *RaftNode) MemberList() ([]RaftNodeTemp, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return nil, errors.New("raft cluster is not started")
	}
	config := r.ra.GetConfiguration()
	if err := config.Error(); err != nil {
		return nil, err
	}
	leader := r.ra.Leader()

	members := config.Configuration().Servers
	ret := make([]RaftNodeTemp, len(members))
	for i, v := range members {
		ret[i] = RaftNodeTemp{
			ID:     string(v.ID),
			State:  v.Suffrage.String(),
			Leader: (v.Address == leader),
		}
	}
	return ret, nil
}
