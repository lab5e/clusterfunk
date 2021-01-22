package sharding

import "encoding"

// Shard represents a partition that a single node is responsible for.
type Shard interface {
	// ID is the shard ID. The shard ID is calculated through a shard function
	// which will map an identifier to a particular shard.
	ID() int

	// NodeID returns the node responsible for the shard.
	NodeID() string

	// SetNodeID sets the node ID for the shard
	SetNodeID(nodeID string)
}

// ShardMap is a type that manages shards. The number of shards are immutable, ie
// no new shards will be added for the lifetime. (shards can be added or removed
// between invocations of the leader)
type ShardMap interface {
	// Init reinitializes the (shard) manager. This can be called one and only
	// once. Performance critical since this is part of the node
	// onboarding process. Shard IDs are assigned from 0...maxShards-1
	Init(maxShards int) error

	// UpdateNodes syncs the nodes internally in the cluster and reshards if
	// necessary.
	UpdateNodes(nodeID ...string)

	// GetNode returns the node (ID) responsible for the shards. Performance
	// critical since this will be used in every single call to determine
	// the home location for mutations.
	// TBD: Panic if the shard ID is > maxShards?
	MapToNode(shardID int) Shard

	// Shards returns a copy of all of the shards. Not performance critical. This
	// is typically used for diagnostics.
	Shards() []Shard

	// ShardCount returns the number of shards
	ShardCount() int

	// NodeList returns a list of nodes in the shard map
	NodeList() []string

	// ShardCountForNode returns the number of shards allocated to a particular node.
	ShardCountForNode(nodeid string) int

	// WorkerID returns the worker ID for the node
	WorkerID(nodeID string) int

	// NewShards lists the new shards in this map compared to the old one. This
	// node is now responsible for these shards.
	NewShards(nodeID string, oldMap ShardMap) []Shard

	// DeletedShards lists the deleted shards in this map compared to the old
	// one. These shards can be dropped by the node when there's a reshard.
	DeletedShards(nodeID string, oldMap ShardMap) []Shard

	// The marshaling methods are used to save and restore the shard manager
	// from the Raft logrus.

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	// TBD: Methods to update shard weights and redistributing shards.

}
