package sharding

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
import "encoding"

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

	// SetNodeID sets the node ID for the shard
	SetNodeID(nodeID string)
}

// ShardMap is a type that manages shards. The number of shards are immutable, ie
// no new shards will be added for the lifetime. (shards can be added or removed
// between invocations of the leader)
type ShardMap interface {
	// Init reinitializes the (shard) manager. This can be called one and only
	// once. Performance critical since this is part of the node
	// onboarding process. The weights parameter may be set to nil. In that
	// case the shards gets a weight of 1. If the weights parameter is specfied
	// the lenght of the weights parameter must match the maxShards parameter.
	// Shard IDs are assigned from 0...maxShards-1
	Init(maxShards int, weights []int) error

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

	// TotalWeight is the total weight of all shards. Not performance critical
	// directly but it will be used when calculating the distribution of shards
	// so the manager should cache this value and update when a shard changes
	// its weight.
	TotalWeight() int

	// ShardCount returns the number of shards
	ShardCount() int

	// NodeList returns a list of nodes in the shard map
	NodeList() []string

	// ShardCountForNode returns the number of shards allocated to a particular node.
	ShardCountForNode(nodeid string) int

	// WorkerID returns the worker ID for the node
	WorkerID(nodeID string) int

	// The marshaling methods are used to save and restore the shard manager
	// from the Raft logrus.

	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler

	// TBD: Methods to update shard weights and redistributing shards.

}
