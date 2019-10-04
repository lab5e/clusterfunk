package funk

import "errors"

// This is for unused types and declarations
// -----------------------------------------------------------------------------

// ... or those that *should* be unused.
// -----------------------------------------------------------------------------

// --- temp methods below

// RaftNodeTemp is ... a temp struct
type RaftNodeTemp struct {
	ID     string
	State  string
	Leader bool
}

// MemberList returns a list of nodes in the raft cluster
func (r *RaftNode) MemberList() ([]RaftNodeTemp, error) {
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
	ret := make([]RaftNodeTemp, len(members))
	for i, v := range members {
		ret[i] = RaftNodeTemp{
			ID:     string(v.ID),
			State:  v.Suffrage.String(),
			Leader: (v.Address == leader),
		}
	}
	return ret, nil
}
