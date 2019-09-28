package cluster

// TODO: Node and NodeCollection are completely different beasts. Rename and make private.

// NodeCollection is a collection of nodes. The events from raft might
// include duplicates but Raft does not support clusters much larger than
// 9-11 nodes so
type NodeCollection struct {
	Nodes []string
}

func newNodeCollection() NodeCollection {
	return NodeCollection{
		Nodes: make([]string, 0),
	}
}

// Sync synchronizes the collection with the IDs in the array and
// returns true if there's a change.
func (n *NodeCollection) Sync(nodes ...string) bool {
	if len(n.Nodes) != len(nodes) {
		n.Nodes = append([]string{}, nodes...)
		return true
	}
	// Make sure all nodes in n.Nodes are in n.nodes
	for i := range n.Nodes {
		found := false
		for j := range nodes {
			if n.Nodes[i] == nodes[j] {
				found = true
				break
			}
		}
		if !found {
			n.Nodes = append([]string{}, nodes...)
			return true
		}
	}
	return false
}

// Size returns the size of the node collection
func (n *NodeCollection) Size() int {
	return len(n.Nodes)
}
