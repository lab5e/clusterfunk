package funk

// nodeItem is a temporary struct used by the SerfNode and RaftNode types
// for their internal memberList methods.
// TODO: make separate structs for SerfNode/RaftNode?
type nodeItem struct {
	ID     string
	State  string
	Leader bool
}
