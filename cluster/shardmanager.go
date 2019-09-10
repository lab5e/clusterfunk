package cluster

// Shard represents a partition that a single node is responsible for.
type Shard interface {
	// ID is the shard ID. The shard ID is calculated through a shard function
	// which will map an identifier to a particular shard.
	ID() int

	// Weight represents the relative work for the shard. Some shards might
	// require more work than others, depending on the work distribution.
	// Initially this can be set to 1 for all shards but if you have hotspots
	// with higher resource requirements (like more CPU or memory) you can
	// increase the weight of a shard to balance the load across the cluster.
	Weight() int

	// NodeID returns the node responsible for the shard.
	NodeID() string
}

// ShardTransfer is an transfer operation on a shard, ie move from A to B.
type ShardTransfer struct {
	Shard             Shard
	SourceNodeID      string
	DestinationNodeID string
}

// ShardManager is a type that manages shards. The number of shards are immutable, ie
// no new shards will be added for the lifetime. (shards can be added or removed
// between invocations of the leader)
type ShardManager interface {
	// Init reinitializes the (shard) manager. This can be called one and only
	// once. Performance critical since this is part of the node
	// onboarding process.
	Init(shards []Shard) error

	// AddBucket adds a new bucket. The returned shard operations are required
	// to balance the shards across the buckets in the cluster. If the bucket
	// already exists nil is returned. Performance critical since this is
	// used when nodes join or leave the cluster.
	AddNode(nodeID string) []ShardTransfer

	// RemoveBucket removes a bucket from the cluster. The returned shard operations
	// are required to balance the shards across the buckets in the cluster.
	// Performance critical since this is used when nodes join or leave the cluster.
	RemoveNode(nodeID string) []ShardTransfer

	// GetNode returns the node (ID) responsible for the shards. Performance
	// critical since this will be used in every single call to determine
	// the home location for mutations.
	MapToNode(shard int) string

	// Shards returns a copy of all of the shards. Not performance critical. This
	// is typically used for diagnostics.
	Shards() []Shard

	// TotalWeight is the total weight of all shards. Not performance critical
	// directly but it will be used when calculating the distribution of shards
	// so the manager should cache this value and update when a shard changes
	// its weight.
	TotalWeight() int

	// TBD: Methods to update shard weights and redistributing shards.
}
