package cluster

import (
	"io"
	"log"
	"sync"
	"time"

	"github.com/hashicorp/raft"
)

type fsmLogEvent struct {
	Index   uint64
	LogType byte
}

// The Raft FSM is used by the clients to access the replicated log. Typically
// this will hold the (high level) data structures for the replaced log.
// Comments on the methods are lifted directly from the godoc.
//
// The Events channel is a single listener that only the clustering library
// will listen on. Expect weird behaviour if more than one client is ingesting
// the messages.
type raftFSM struct {
	Events chan fsmLogEvent
	state  map[byte][]byte
	mutex  *sync.Mutex
}

// newStateMaching creates a new client-side FSM
func newStateMachine() *raftFSM {
	return &raftFSM{
		Events: make(chan fsmLogEvent),
		state:  make(map[byte][]byte),
		mutex:  &sync.Mutex{},
	}
}

func (f *raftFSM) logEvent(idx uint64, logType byte) {
	ev := fsmLogEvent{
		Index:   idx,
		LogType: logType,
	}
	// Why not use select...case...default? Well - it turns out the default clause
	// is *really* picky so even a 10 us delay will discard the message. Nice to
	// know.
	select {
	case f.Events <- ev:
	case <-time.After(1 * time.Millisecond):
		// drop the event. Panic is a bit strict but nice for debugging.
		panic("dropped event from FSM")
	}
}

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.
func (f *raftFSM) Apply(l *raft.Log) interface{} {
	//log.Printf("FSM: Apply, index = %d, term = %d", l.Index, l.Term)
	f.logEvent(l.Index, l.Data[0])
	if len(l.Data) > 1 {
		f.mutex.Lock()
		defer f.mutex.Unlock()
		id := l.Data[0]
		buf := make([]byte, len(l.Data)-1)
		copy(buf, l.Data[1:])
		f.state[id] = buf
	}
	return nil
}

// Snapshot is used to support log compaction. This call should
// return an FSMSnapshot which can be used to save a point-in-time
// snapshot of the FSM. Apply and Snapshot are not called in multiple
// threads, but Apply will be called concurrently with Persist. This means
// the FSM should be implemented in a fashion that allows for concurrent
// updates while a snapshot is happening.
func (f *raftFSM) Snapshot() (raft.FSMSnapshot, error) {
	return &raftSnapshot{}, nil
}

// Entry returns the latest log message with that particular ID
func (f *raftFSM) Entry(id byte) []byte {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	buf, exists := f.state[id]
	if !exists {
		return nil
	}
	ret := make([]byte, len(buf))
	copy(ret, buf)
	return ret
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
func (f *raftFSM) Restore(io.ReadCloser) error {
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
	// nothing happens here.
}
