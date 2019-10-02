package cluster

import "sync"

// StringSet is a collection of nodes.
type StringSet struct {
	Strings []string
	Mutex   *sync.RWMutex
}

// NewStringSet creates a new string set instance
func NewStringSet() StringSet {
	return StringSet{
		Strings: make([]string, 0),
		Mutex:   &sync.RWMutex{},
	}
}

// Sync synchronizes the collection with the IDs in the array and
// returns true if there's a change.
func (s *StringSet) Sync(nodes ...string) bool {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	if len(s.Strings) != len(nodes) {
		s.Strings = append([]string{}, nodes...)
		return true
	}
	// Make sure all nodes in n.Nodes are in n.nodes
	for i := range s.Strings {
		found := false
		for j := range nodes {
			if s.Strings[i] == nodes[j] {
				found = true
				break
			}
		}
		if !found {
			s.Strings = append([]string{}, nodes...)
			return true
		}
	}
	return false
}

// Add adds a new node to the collection. It returns true if the node is added
func (s *StringSet) Add(node string) bool {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	for _, v := range s.Strings {
		if v == node {
			return false
		}
	}
	s.Strings = append(s.Strings, node)
	return true
}

// Remove removes a node from the collection. It returns true if a node is removed
func (s *StringSet) Remove(node string) bool {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	for i, v := range s.Strings {
		if v == node {
			s.Strings = append(s.Strings[:i], s.Strings[i+1:]...)
			return true
		}
	}
	return false
}

// Size returns the size of the node collection
func (s *StringSet) Size() int {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return len(s.Strings)
}

// List returns a list of the nodes in the collection
func (s *StringSet) List() []string {
	s.Mutex.RLock()
	defer s.Mutex.RUnlock()
	return s.Strings[:]
}

// Clear empties the string set
func (s *StringSet) Clear() {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()

	s.Strings = []string{}
}
