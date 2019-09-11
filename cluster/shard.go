package cluster

type weightedShard struct {
	id     int
	weight int
	nodeID string
}

// NewShard creates a new shard with a weight
func NewShard(id, weight int) Shard {
	return &weightedShard{
		id:     id,
		weight: weight,
		nodeID: "",
	}
}

func (w *weightedShard) ID() int {
	return w.id
}

func (w *weightedShard) Weight() int {
	return w.weight
}

func (w *weightedShard) NodeID() string {
	return w.nodeID
}

func (w *weightedShard) SetNodeID(nodeID string) {
	w.nodeID = nodeID
}
