package cluster

// Node is one of the participants in the cluster or in the swarm. A node
// might be a full voting member, a non-voting member or just a member in the
// swarm.
type Node interface {
	// ID returns the node ID. This is an unique string in the cluster
	ID() string

	// Kind returns the type of node
	Role() NodeRole

	// Tags returns the node's tags
	Tags() map[string]string
}

// NewNode creates a new node
func NewNode(nodeID string, tags map[string]string, role NodeRole) Node {
	ret := &clusterNode{id: nodeID, tags: map[string]string{}, role: role}
	for k, v := range tags {
		ret.tags[k] = v
	}
	return ret
}

type clusterNode struct {
	id   string
	tags map[string]string
	role NodeRole
}

func (n *clusterNode) ID() string {
	return n.id
}

func (n *clusterNode) Role() NodeRole {
	return n.role
}

func (n *clusterNode) Tags() map[string]string {
	return n.tags
}

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

// AddNode adds a new node to the collection. Returns true if the
// list is modified.
func (n *NodeCollection) AddNode(nodeID string) bool {
	for i := range n.Nodes {
		if n.Nodes[i] == nodeID {
			return false
		}
	}
	n.Nodes = append(n.Nodes, nodeID)
	return true
}

// RemoveNode removes a node from the collection. Returns true
// if the list is modified
func (n *NodeCollection) RemoveNode(nodeID string) bool {
	for i := range n.Nodes {
		if n.Nodes[i] == nodeID {
			n.Nodes = append(n.Nodes[:i], n.Nodes[i+1:]...)
			return true
		}
	}
	return false
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
