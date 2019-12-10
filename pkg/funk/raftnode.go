package funk
//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"errors"
	"fmt"
	"io"

	log "github.com/sirupsen/logrus"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/toolbox"

	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// RaftEventType is the event type for events emitted by the RaftNode type
type RaftEventType int

const (
	//RaftClusterSizeChanged is emitted when a new node is added
	RaftClusterSizeChanged RaftEventType = iota
	// RaftLeaderLost is emitted when the leader is lost, ie the node enters the candidate state
	RaftLeaderLost
	// RaftBecameLeader is emitted when the leader becomes the leader
	RaftBecameLeader
	// RaftBecameFollower is emitted when the node becomes a follower
	RaftBecameFollower
	// RaftReceivedLog is emitted when a log entry is receievd
	RaftReceivedLog
	// RaftUndefinedEvent is the undefined event type
	RaftUndefinedEvent
)

// String is the string representation of the event
func (r RaftEventType) String() string {
	switch r {
	case RaftClusterSizeChanged:
		return "RaftClusterSizeChanged"
	case RaftLeaderLost:
		return "RaftLeaderLost"
	case RaftBecameLeader:
		return "RaftBecameLeader"
	case RaftBecameFollower:
		return "RaftBecameFollower"
	case RaftReceivedLog:
		return "RaftReceivedLog"
	default:
		panic(fmt.Sprintf("Unknown raft event type: %d", r))
	}
}

// RaftNode is a wrapper for the Raft library. The raw events are coalesced into
// higher level events (particularly RaftClusterSizeChanged). Coalesced events
// introduce a small (millisecond) delay on the events but everything on top of
// this library will operate in the millisecond range.
//
// In addition this type keeps track of the active nodes at all times via the
// raft events. There's no guarantee that the list of nodes in the cluster will
// be up to date or correct for the followers. The followers will only
// interact with the leader of the cluster.
type RaftNode struct {
	mutex            *sync.RWMutex // Mutex for the attributes
	fsmMutex         *sync.RWMutex // Mutex for the FSM
	scheduledMutex   *sync.Mutex   // Mutex for scheduled events
	scheduled        map[RaftEventType]time.Time
	localNodeID      string                        // The local node ID
	raftEndpoint     string                        // Raft endpoint
	ra               *raft.Raft                    // Raft instance
	events           chan RaftEventType            // Coalesced events from Raft
	unfilteredEvents chan RaftEventType            // Unfiltered events from Raft
	state            map[LogMessageType]LogMessage // The internal FSM state
	Nodes            toolbox.StringSet
}

// NewRaftNode creates a new RaftNode instance
func NewRaftNode() *RaftNode {
	return &RaftNode{
		Nodes:            toolbox.NewStringSet(),
		localNodeID:      "",
		mutex:            &sync.RWMutex{},
		fsmMutex:         &sync.RWMutex{},
		scheduledMutex:   &sync.Mutex{},
		scheduled:        make(map[RaftEventType]time.Time),
		events:           make(chan RaftEventType),    // tiny buffer here to make multiple events feasable.
		unfilteredEvents: make(chan RaftEventType, 5), // unfiltered events that gets coalesced into one big
		state:            make(map[LogMessageType]LogMessage),
	}
}

// RaftParameters is the configuration for the Raft cluster
type RaftParameters struct {
	RaftEndpoint string `param:"desc=Endpoint for Raft;default="`
	DiskStore    bool   `param:"desc=Disk-based store;default=false"`
	Bootstrap    bool   `param:"desc=Bootstrap a new Raft cluster;default=false"`
	Verbose      bool   `param:"desc=Verbose Raft logging;default=false"`
	DebugLog     bool   `param:"desc=Show debug log messages for Raft;default=false"`
}

// Start launches the node
func (r *RaftNode) Start(nodeID string, cfg RaftParameters) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.ra != nil {
		return errors.New("raft cluster is already started")
	}

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	if cfg.Verbose {
		config.LogLevel = "INFO"
		if cfg.DebugLog {
			config.LogLevel = "DEBUG"
		}
	} else {
		config.LogOutput = newMutedLogger().Writer()
	}

	addr, err := net.ResolveTCPAddr("tcp", cfg.RaftEndpoint)
	if err != nil {
		return err
	}

	/* These are the defaults:
	HeartbeatTimeout:   1000 * time.Millisecond,
	ElectionTimeout:    1000 * time.Millisecond,
	CommitTimeout:      50 * time.Millisecond,
	SnapshotInterval:   120 * time.Second,
	LeaderLeaseTimeout: 500 * time.Millisecond,
	*/

	//These might be too optimistic.
	config.HeartbeatTimeout = 100 * time.Millisecond
	config.ElectionTimeout = 100 * time.Millisecond
	config.CommitTimeout = 5 * time.Millisecond
	config.LeaderLeaseTimeout = 50 * time.Millisecond

	/*
		// Half the defaults
		config.HeartbeatTimeout = 500 * time.Millisecond
		config.ElectionTimeout = 500 * time.Millisecond
		config.CommitTimeout = 25 * time.Millisecond
		config.LeaderLeaseTimeout = 250 * time.Millisecond
	*/

	// The transport logging is separate form the configuration transport. Obviously.
	logger := io.Writer(os.Stderr)
	if !cfg.DebugLog {
		// Will only log the transport log as debug
		logger = newMutedLogger().Writer()
	}
	transport, err := raft.NewTCPTransport(addr.String(), addr, 3, 500*time.Millisecond, logger)
	if err != nil {
		return err
	}
	r.raftEndpoint = string(transport.LocalAddr())

	var logStore raft.LogStore
	var stableStore raft.StableStore
	var snapshotStore raft.SnapshotStore

	if cfg.DiskStore {
		raftdir := fmt.Sprintf("./%s", nodeID)
		log.WithField("dbdir", raftdir).Info("Using boltDB and snapshot store")
		if err := os.MkdirAll(raftdir, os.ModePerm); err != nil {
			log.WithError(err).WithField("dbdir", raftdir).Error("Unable to create store dir")
			return err
		}
		boltDB, err := raftboltdb.NewBoltStore(filepath.Join(raftdir, fmt.Sprintf("%s.db", nodeID)))
		if err != nil {
			log.WithError(err).Error("Unable to create boltDB")
			return err
		}
		logStore = boltDB
		stableStore = boltDB
		snapshotStore, err = raft.NewFileSnapshotStore(raftdir, 3, os.Stderr)
		if err != nil {
			log.WithError(err).WithField("dbdir", raftdir).Error("Unable to create snapshot store")
			return err
		}
	} else {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
		snapshotStore = raft.NewInmemSnapshotStore()
	}
	r.ra, err = raft.NewRaft(config, r, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	if cfg.Bootstrap {
		log.Info("Bootstrapping new cluster")
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		f := r.ra.BootstrapCluster(configuration)
		if f.Error() != nil {
			return f.Error()
		}
	}
	observerChan := make(chan raft.Observation)

	// This node will - surprise - be a member of the cluster
	r.addNode(nodeID)

	go r.observerFunc(observerChan)
	go r.coalesceEvents()
	r.ra.RegisterObserver(raft.NewObserver(observerChan, true, func(*raft.Observation) bool { return true }))
	r.localNodeID = nodeID
	r.sendInternalEvent(RaftBecameFollower)

	return nil
}

func (r *RaftNode) coalesceEvents() {
	for ev := range r.unfilteredEvents {
		timeout := false
		lastEvent := ev
		for !timeout {
			select {
			case ev := <-r.unfilteredEvents:
				if ev == lastEvent {
					continue
				}
				r.events <- lastEvent
				lastEvent = ev
				timeout = true
			case <-time.After(1 * time.Millisecond):
				timeout = true
			}
		}
		select {
		case r.events <- lastEvent:
		case <-time.After(1 * time.Second):
			panic("Event listener is too slow")

		}
	}
	panic("Coalescing function has stopped")
}

func (r *RaftNode) observerFunc(ch chan raft.Observation) {
	for k := range ch {
		switch v := k.Data.(type) {
		case raft.PeerObservation:
			if v.Removed {
				r.removeNode(string(v.Peer.ID))
				continue
			}
			r.addNode(string(v.Peer.ID))

		case raft.LeaderObservation:
			// This can be ignored since we're monitoring the state
			// and are getting the leader info via other channels.

		case raft.RaftState:
			switch v {
			case raft.Candidate:
				r.sendInternalEvent(RaftLeaderLost)

			case raft.Follower:
				r.sendInternalEvent(RaftBecameFollower)

			case raft.Leader:
				r.sendInternalEvent(RaftBecameLeader)
				// This might look a bit weird but the cluster size does not
				// change when there's only a single node becoming a leader
				r.scheduleInternalEvent(RaftClusterSizeChanged, 500*time.Millisecond)
			}

		case raft.RequestVoteRequest:
			// Not using this at the moment

		default:
			log.WithFields(log.Fields{
				"event": k,
				"data":  k.Data,
			}).Error("Unknown Raft event")
		}
	}
}

// Stop stops the node. If the removeWhenStopping flag is set and the server is
// the leader it will remove itself.
func (r *RaftNode) Stop(removeWhenStopping bool) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.ra == nil {
		return errors.New("raft cluster is already stopped")
	}

	if err := r.ra.Shutdown().Error(); err != nil {
		log.WithError(err).Info("Got error on shutdown")
	}
	r.ra = nil
	r.localNodeID = ""
	r.raftEndpoint = ""
	return nil
}

// LocalNodeID returns the local NodeID
func (r *RaftNode) LocalNodeID() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.localNodeID
}

// AddClusterNode adds a new node to the cluster. Must be leader to perform this operation.
func (r *RaftNode) AddClusterNode(nodeID string, endpoint string) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.ra == nil {
		return errors.New("raft cluster is not started")
	}

	if err := r.ra.VerifyLeader().Error(); err != nil {
		// Not the leader so can't add node
		return errors.New("must be leader to add a new member")
	}

	configFuture := r.ra.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(endpoint) {
			// it's already joined
			return nil
		}
	}

	f := r.ra.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(endpoint), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}
	r.addNode(nodeID)
	return nil
}

// RemoveClusterNode removes a node from the cluster. Must be leader to perform this
// operation.
func (r *RaftNode) RemoveClusterNode(nodeID string, endpoint string) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if r.ra == nil {
		return errors.New("raft cluster isn't started")
	}
	if r.ra.VerifyLeader().Error() != nil {
		return errors.New("must be leader to remove ndoe")
	}

	configFuture := r.ra.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}

	r.removeNode(nodeID)
	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(endpoint) {
			return r.ra.RemoveServer(raft.ServerID(nodeID), 0, 0).Error()
		}
	}

	// The server does not exist in the cluster - *technically* an error but
	// it's no longer in the cluster so we're good.
	return nil
}

// Endpoint returns the Raft endpoint (aka bind address)
func (r *RaftNode) Endpoint() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.raftEndpoint
}

// Leader returns true if this node is the leader. This will verify the
// leadership with the Raft library.
func (r *RaftNode) Leader() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return false
	}
	return (r.ra.VerifyLeader().Error() == nil)
}

// AppendLogEntry appends a log entry to the log. The function returns when
// there's a quorum in the cluster
func (r *RaftNode) AppendLogEntry(data []byte) (uint64, error) {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return 0, errors.New("raft node not started")
	}
	f := r.ra.Apply(data, time.Second*2)
	if err := f.Error(); err != nil {
		return 0, err
	}
	return f.Index(), nil
}

// -----------------------------------------------------------------------------
// This is temporary methods that are used in the management code

// LastLogIndex returns the last log index received
func (r *RaftNode) LastLogIndex() uint64 {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return 0
	}
	return r.ra.AppliedIndex()
}

// Events returns the event channel. There is only one
// event channel so use multiple listeners at your own peril.
// You will get NodeAdded, NodeRemoved, LeaderLost and LeaderChanged
// events on this channel.
func (r *RaftNode) Events() <-chan RaftEventType {
	return r.events
}

// GetLogMessages returns the replicated log message with the
// specified type ID
func (r *RaftNode) GetLogMessages(startingIndex uint64) []LogMessage {
	r.fsmMutex.Lock()
	defer r.fsmMutex.Unlock()
	ret := make([]LogMessage, 0)
	for _, v := range r.state {
		if v.Index > startingIndex {
			ret = append(ret, v)
		}
	}
	return ret
}

// StepDown steps down as a leader if this is the leader node
func (r *RaftNode) StepDown() error {
	if !r.Leader() {
		return errors.New("not the leader")
	}
	return r.ra.LeadershipTransfer().Error()
}

func (r *RaftNode) addNode(id string) {
	if r.Nodes.Add(id) {
		r.sendInternalEvent(RaftClusterSizeChanged)
	}
}

func (r *RaftNode) removeNode(id string) {
	if r.Nodes.Remove(id) {
		r.sendInternalEvent(RaftClusterSizeChanged)
	}
}

// RefreshNodes refreshes the node list by reading the Raft configuration.
// If any events are skipped by the Raft library we'll get the appropriate
// events and an updated node list after this call.
func (r *RaftNode) RefreshNodes() {
	cfg := r.ra.GetConfiguration()
	if cfg.Error() != nil {
		log.WithError(cfg.Error()).Warn("Unable to update nodes")
		return
	}
	list := []string{}
	for _, v := range cfg.Configuration().Servers {
		list = append(list, string(v.ID))
	}
	r.Nodes.Sync(list...)
}

func (r *RaftNode) sendInternalEvent(ev RaftEventType) {
	select {
	case r.unfilteredEvents <- ev:
		// Remove aync scheduled events of this type.
		r.scheduledMutex.Lock()
		delete(r.scheduled, ev)
		r.scheduledMutex.Unlock()
	case <-time.After(500 * time.Millisecond):
		panic(fmt.Sprintf("Unable to send internal event %s. Channel full?", ev.String()))
	}
}

func (r *RaftNode) scheduleInternalEvent(ev RaftEventType, timeout time.Duration) {
	r.scheduledMutex.Lock()
	r.scheduled[ev] = time.Now().Add(timeout)
	r.scheduledMutex.Unlock()
	go func() {
		time.Sleep(timeout)
		r.scheduledMutex.Lock()
		_, ok := r.scheduled[ev]
		r.scheduledMutex.Unlock()
		if ok {
			// send the event. Log for now since it happens only in certain
			// circumstances. In most circumstances a change in leadership
			// happens because a node goes down or fails and then a
			// cluster size notification is sent but on rare occasions when
			// a node silently fails it won't trigger a size change event.
			log.WithFields(log.Fields{
				"event":     ev.String(),
				"timeoutMs": timeout / time.Millisecond,
			}).Warn("Scheduled event sent")
			r.sendInternalEvent(ev)
		}
	}()
}

// EnableNode enables a node that has been disabled. The node might be a part of the
// cluster but not available.
func (r *RaftNode) EnableNode(id string) {
	if !r.Leader() {
		return
	}
	r.addNode(id)
}

// DisableNode disables a node (in reality removing it from the local node list
// but it is still a member of the Raft cluster)
func (r *RaftNode) DisableNode(id string) {
	if !r.Leader() {
		return
	}
	r.removeNode(id)
}

// LeaderNodeID returns the Node ID for the leader
func (r *RaftNode) LeaderNodeID() string {
	if r.Leader() {
		return r.LocalNodeID()
	}

	list, err := r.memberList()
	if err != nil {
		// TODO: Handle gracefully (sort of - this will end up as an error later)
		return ""
	}
	for _, v := range list {
		if v.Leader {
			return v.ID
		}
	}
	return ""
}

// The raft.FSM implementation. Right now the implementation looks a lot more
// like a storage layer but technically it's a FSM

// Apply log is invoked once a log entry is committed.
// It returns a value which will be made available in the
// ApplyFuture returned by Raft.Apply method if that
// method was called on the same Raft node as the FSM.
func (r *RaftNode) Apply(l *raft.Log) interface{} {
	msg := LogMessage{}
	if err := msg.UnmarshalBinary(l.Data); err != nil {
		panic(fmt.Sprintf(" ***** Error decoding log message: %v", err))
	}
	r.fsmMutex.Lock()
	defer r.fsmMutex.Unlock()
	msg.Index = l.Index
	r.state[msg.MessageType] = msg
	r.sendInternalEvent(RaftReceivedLog)
	return l.Data
}

// Snapshot is used to support log compaction. This call should
// return an FSMSnapshot which can be used to save a point-in-time
// snapshot of the FSM. Apply and Snapshot are not called in multiple
// threads, but Apply will be called concurrently with Persist. This means
// the FSM should be implemented in a fashion that allows for concurrent
// updates while a snapshot is happening.
func (r *RaftNode) Snapshot() (raft.FSMSnapshot, error) {
	return &raftSnapshot{}, nil
}

// Restore is used to restore an FSM from a snapshot. It is not called
// concurrently with any other command. The FSM must discard all previous
// state.
func (r *RaftNode) Restore(io.ReadCloser) error {
	log.Info("FSMSnapshot Restore")
	return nil
}

// memberList returns a list of nodes in the raft cluster.
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

type raftSnapshot struct {
}

// Persist should dump all necessary state to the WriteCloser 'sink',
// and call sink.Close() when finished or call sink.Cancel() on error.
func (r *raftSnapshot) Persist(sink raft.SnapshotSink) error {
	log.Info("FSMSnapshot Persist")
	sink.Close()
	return nil
}

// Release is invoked when we are finished with the snapshot.
func (r *raftSnapshot) Release() {
	// nothing happens here.
	log.Info("FSMSnapshot Release")
}
