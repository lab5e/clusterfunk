package cluster

import "sync"

// NodeCollection is a collection of nodes.
type NodeCollection struct {
	Nodes []string
	Mutex *sync.RWMutex
}

func newNodeCollection() NodeCollection {
	return NodeCollection{
		Nodes: make([]string, 0),
		Mutex: &sync.RWMutex{},
	}
}

// Sync synchronizes the collection with the IDs in the array and
// returns true if there's a change.
func (n *NodeCollection) Sync(nodes ...string) bool {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
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

// Add adds a new node to the collection. It returns true if the node is added
func (n *NodeCollection) Add(node string) bool {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	for _, v := range n.Nodes {
		if v == node {
			return false
		}
	}
	n.Nodes = append(n.Nodes, node)
	return true
}

// Remove removes a node from the collection. It returns true if a node is removed
func (n *NodeCollection) Remove(node string) bool {
	n.Mutex.Lock()
	defer n.Mutex.Unlock()
	for i, v := range n.Nodes {
		if v == node {
			n.Nodes = append(n.Nodes[:i], n.Nodes[i+1:]...)
			return true
		}
	}
	return false
}

// Size returns the size of the node collection
func (n *NodeCollection) Size() int {
	n.Mutex.RLock()
	defer n.Mutex.RUnlock()
	return len(n.Nodes)
}

// List returns a list of the nodes in the collection
func (n *NodeCollection) List() []string {
	n.Mutex.RLock()
	defer n.Mutex.RUnlock()
	return n.Nodes[:]
}
