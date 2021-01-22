package sharding

type simpleShard struct {
	id     int
	nodeID string
}

// NewShard creates a new shard with a weight
func NewShard(id int) Shard {
	return &simpleShard{
		id:     id,
		nodeID: "",
	}
}

func (w *simpleShard) ID() int {
	return w.id
}

func (w *simpleShard) NodeID() string {
	return w.nodeID
}

func (w *simpleShard) SetNodeID(nodeID string) {
	w.nodeID = nodeID
}
