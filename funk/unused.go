package funk

import "errors"

// This is for unused types and declarations
// -----------------------------------------------------------------------------

// ... or those that *should* be unused.
// -----------------------------------------------------------------------------

// --- temp methods below

// nodeList is ... a temp struct
type nodeItem struct {
	ID     string
	State  string
	Leader bool
}

// memberList returns a list of nodes in the raft cluster. This might be a
// keeper. It's used to find the leader but it's quite inefficient
func (r *RaftNode) memberList() ([]nodeItem, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return nil, errors.New("raft cluster is not started")
	}
	config := r.ra.GetConfiguration()
	if err := config.Error(); err != nil {
		return nil, err
	}
	leader := r.ra.Leader()

	members := config.Configuration().Servers
	ret := make([]nodeItem, len(members))
	for i, v := range members {
		ret[i] = nodeItem{
			ID:     string(v.ID),
			State:  v.Suffrage.String(),
			Leader: (v.Address == leader),
		}
	}
	return ret, nil
}
