package cluster

type weightedShardManager struct {
}

// NewShardManager creates a new ShardManager instance.
func NewShardManager() ShardManager {
	return &weightedShardManager{}
}

func (sm *weightedShardManager) Init(shards []Shard) error {
	panic("not implemented")
}

func (sm *weightedShardManager) AddNode(nodeID string) []ShardTransfer {
	panic("not implemented")
}

func (sm *weightedShardManager) RemoveNode(nodeID string) []ShardTransfer {
	panic("not implemented")
}

func (sm *weightedShardManager) MapToNode(shardID int) string {
	panic("not implemented")
}

func (sm *weightedShardManager) Shards() []Shard {
	panic("not implemented")
}

func (sm *weightedShardManager) TotalWeight() int {
	panic("not implemented")
}
