package cluster

// Node is one of the participants in the cluster or in the swarm. A node
// might be a full voting member, a non-voting member or just a member in the
// swarm.
type Node struct {
	// ID returns the node ID. This is an unique string in the cluster
	ID string

	// Kind returns the type of node
	Role NodeRole

	// Tags returns the node's tags
	Tags map[string]string
}

// NewNode creates a new node
func NewNode(nodeID string, role NodeRole) Node {
	ret := Node{ID: nodeID, Tags: map[string]string{}, Role: role}
	return ret
}

// SetTags copies and sets the tags on the node
func (n *Node) SetTags(tags map[string]string) {
	for k, v := range tags {
		n.Tags[k] = v
	}
}
