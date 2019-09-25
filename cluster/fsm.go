package cluster

import (
	"io"
	"log"
	"sync"

	"github.com/hashicorp/raft"
)

// These are the events emitted by the log. They are handled by the
// cluster library.
type fsmLogEventType byte

const (
	shardMapReceived fsmLogEventType = iota
	commitEntryReceived
	fsmReadyEvent
)

type fsmLogEvent struct {
	EventType fsmLogEventType
	Index     uint64
	Data      []byte
}

// The Raft FSM is used by the clients to access the replicated log. Typically
// this will hold the (high level) data structures for the replaced log.
// Comments on the methods are lifted directly from the godoc.
//
// The Events channel is a single listener that only the clustering library
// will listen on. Expect weird behaviour if more than one client is ingesting
// the messages.
type raftFsm struct {
	Events chan fsmLogEvent
	mutex  *sync.Mutex
}

// newStateMaching creates a new client-side FSM
func newStateMachine() *raftFsm {
	return &raftFsm{Events: make(chan fsmLogEvent), mutex: &sync.Mutex{}}
}
func (r *raftFsm) SignalReady(lastIndex uint64) {
	r.logEvent(fsmReadyEvent, lastIndex, []byte{})
}
func (r *raftFsm) logEvent(et fsmLogEventType, idx uint64, data []byte) {
	ev := fsmLogEvent{
		EventType: et,
		Index:     idx,
		Data:      data,
	}
	select {
	case r.Events <- ev:
	default:
		// drop the event.
	}
}

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.
func (r *raftFsm) Apply(l *raft.Log) interface{} {
	//log.Printf("FSM: Apply, index = %d, term = %d", l.Index, l.Term)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	logMsg := NewLogMessage(0, nil)
	if err := logMsg.UnmarshalBinary(l.Data); err != nil {
		log.Printf("Couldn't unmarshal log message: %v (id=%d, len=%d)", err, l.Data[0], len(l.Data))
	}
	switch logMsg.MessageType {
	case ProposedShardMap:
		r.logEvent(shardMapReceived, l.Index, l.Data)
	case ShardMapCommitted:
		r.logEvent(commitEntryReceived, l.Index, l.Data)
	default:
		log.Printf("Unknown log message type received: id=%d, len=%d", logMsg.MessageType, len(logMsg.Data))

	}
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
