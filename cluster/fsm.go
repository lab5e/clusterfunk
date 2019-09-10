package cluster

import (
	"io"
	"log"

	"github.com/hashicorp/raft"
)

// The Raft FSM is used by the clients to access the replicated log. Typically
// this will hold the (high level) data structures for the replaced log.
// Comments on the methods are lifted directly from the godoc.
type raftFsm struct {
}

// newStateMaching creates a new client-side FSM
func newStateMachine() *raftFsm {
	return &raftFsm{}
}

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.

func (r *raftFsm) Apply(l *raft.Log) interface{} {
	log.Printf("FSM: Apply, index = %d, term = %d", l.Index, l.Term)
	return nil
}

// Snapshot is used to support log compaction. This call should
// return an FSMSnapshot which can be used to save a point-in-time
// snapshot of the FSM. Apply and Snapshot are not called in multiple
// threads, but Apply will be called concurrently with Persist. This means
// the FSM should be implemented in a fashion that allows for concurrent
// updates while a snapshot is happening.
func (r *raftFsm) Snapshot() (raft.FSMSnapshot, error) {
	log.Printf("FSM: Snapshot")
	return &raftSnapshot{}, nil
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
func (r *raftFsm) Restore(io.ReadCloser) error {
	log.Printf("FSM: Restore")
	return nil
}

type raftSnapshot struct {
}

// Persist should dump all necessary state to the WriteCloser 'sink',
// and call sink.Close() when finished or call sink.Cancel() on error.
func (r *raftSnapshot) Persist(sink raft.SnapshotSink) error {
	log.Printf("FSMSnapshot: Persist")
	sink.Close()
	return nil
}

// Release is invoked when we are finished with the snapshot.
func (r *raftSnapshot) Release() {
	log.Printf("FSMSnapshot: Release")
}
