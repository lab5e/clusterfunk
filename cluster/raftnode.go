package cluster

import (
	"errors"
	"fmt"
	"io"
	"log"
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
	mutex          *sync.RWMutex
	nodeMutex      *sync.RWMutex
	localNodeID    string
	raftEndpoint   string
	ra             *raft.Raft
	events         chan RaftEventType
	fsm            *raftFSM
	nodes          map[string]bool
	internalEvents chan RaftEventType
}

// NewRaftNode creates a new RaftNode instance
func NewRaftNode() *RaftNode {
	return &RaftNode{
		localNodeID:    "",
		mutex:          &sync.RWMutex{},
		nodeMutex:      &sync.RWMutex{},
		events:         make(chan RaftEventType, 10), // tiny buffer here to make multiple events feasable.
		internalEvents: make(chan RaftEventType, 10),
		nodes:          make(map[string]bool, 0), // Active nodes in the cluster
	}
}

// RaftParameters is the configuration for the Raft cluster
type RaftParameters struct {
	RaftEndpoint string `param:"desc=Endpoint for Raft;default="`
	DiskStore    bool   `param:"desc=Disk-based store;default=false"`
	Bootstrap    bool   `param:"desc=Bootstrap a new Raft cluster;default=false"`
}

// Start launches the node
func (r *RaftNode) Start(nodeID string, verboseLog bool, cfg RaftParameters) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.ra != nil {
		return errors.New("raft cluster is already started")
	}

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(nodeID)

	if verboseLog {
		config.LogLevel = "DEBUG"
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
	if !verboseLog {
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
		log.Printf("Using boltDB and snapshot store in %s", raftdir)
		if err := os.MkdirAll(raftdir, os.ModePerm); err != nil {
			log.Printf("Unable to create store dir: %v", err)
			return err
		}
		boltDB, err := raftboltdb.NewBoltStore(filepath.Join(raftdir, fmt.Sprintf("%s.db", nodeID)))
		if err != nil {
			log.Printf("Unable to create boltDB: %v", err)
			return err
		}
		logStore = boltDB
		stableStore = boltDB
		snapshotStore, err = raft.NewFileSnapshotStore(raftdir, 3, os.Stderr)
		if err != nil {
			log.Printf("Unable to create snapshot store: %v", err)
			return err
		}
	} else {
		logStore = raft.NewInmemStore()
		stableStore = raft.NewInmemStore()
		snapshotStore = raft.NewInmemSnapshotStore()
	}
	r.fsm = newStateMachine()
	go r.logObserver(r.fsm.Events)

	r.ra, err = raft.NewRaft(config, r.fsm, logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	if cfg.Bootstrap {
		log.Printf("Bootstrapping new cluster")
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
	go r.coalescingEvents()
	go r.observerFunc(observerChan)
	r.ra.RegisterObserver(raft.NewObserver(observerChan, true, func(*raft.Observation) bool { return true }))
	r.localNodeID = nodeID
	r.sendInternalEvent(RaftBecameFollower)

	return nil
}

func (r *RaftNode) logObserver(ch chan fsmLogEvent) {
	for range ch {
		r.sendInternalEvent(RaftReceivedLog)
	}
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
				// Cluster size changed events will not be triggered when the
				// node is the only member in the cluster. This is just to
				// make sure the event is triggered for the first time.
				r.sendInternalEvent(RaftClusterSizeChanged)
			}
		case raft.PeerLiveness:
			lt, ok := k.Data.(raft.PeerLiveness)
			if ok {
				if !lt.Heartbeat {
					r.removeNode(string(lt.ID))
				}
				if lt.Heartbeat {
					r.addNode(string(lt.ID))
				}
			}
		case raft.RequestVoteRequest:
			// Not using this at the moment

		default:
			log.Printf("Unknown Raft event: %+v (%T)", k, k.Data)
		}
	}
}

// Stop stops the node
func (r *RaftNode) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.ra == nil {
		return errors.New("raft cluster is already stopped")
	}

	if r.ra.VerifyLeader().Error() == nil {
		// I'm the leader. Transfer leadership away before stopping
		if err := r.ra.RemoveServer(raft.ServerID(r.localNodeID), 0, 0).Error(); err != nil {
			return err
		}
	}
	if err := r.ra.Shutdown().Error(); err != nil {
		return err
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

// AddMember adds a new node to the cluster
func (r *RaftNode) AddMember(nodeID string, endpoint string) error {
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

// RemoveMember removes a node from the cluster
func (r *RaftNode) RemoveMember(nodeID string, endpoint string) error {
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
	// TODO: Check if this is really necessary. Apply might do the job
	if err := r.ra.Barrier(0).Error(); err != nil {
		return 0, err
	}
	return f.Index(), nil
}

// Members returns a list of the node IDs that are a member of the cluster
func (r *RaftNode) Members() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	var ret []string
	for k := range r.nodes {
		ret = append(ret, k)
	}
	return ret
}

// -----------------------------------------------------------------------------
// This is temporary methods that are used in the management code

// MemberCount returns the number of members in the Raft cluster
func (r *RaftNode) MemberCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return len(r.nodes)
}

// LastIndex returns the last log index received
func (r *RaftNode) LastIndex() uint64 {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return 0
	}
	return r.ra.LastIndex()
}

// Events returns the event channel. There is only one
// event channel so use multiple listeners at your own peril.
// You will get NodeAdded, NodeRemoved, LeaderLost and LeaderChanged
// events on this channel.
func (r *RaftNode) Events() <-chan RaftEventType {
	return r.events
}

// GetReplicatedLogMessage returns the replicated log message with the
// specified type ID
func (r *RaftNode) GetReplicatedLogMessage(id LogMessageType) LogMessage {
	return r.fsm.Entry(id)
}

func (r *RaftNode) addNode(id string) {
	r.nodeMutex.Lock()
	defer r.nodeMutex.Unlock()

	_, exists := r.nodes[id]
	if exists {
		return
	}
	r.nodes[id] = true
	r.sendInternalEvent(RaftClusterSizeChanged)
}

func (r *RaftNode) removeNode(id string) {
	r.nodeMutex.Lock()
	defer r.nodeMutex.Unlock()

	_, exists := r.nodes[id]
	if exists {
		delete(r.nodes, id)
		r.sendInternalEvent(RaftClusterSizeChanged)
	}
}

func (r *RaftNode) sendInternalEvent(ev RaftEventType) {
	select {
	case r.internalEvents <- ev:
	case <-time.After(10 * time.Millisecond):
		panic("Unable to send internal event. Channel full?")
	}
}

func (r *RaftNode) coalescingEvents() {
	eventsToGenerate := make(map[RaftEventType]int)
	for {
		timedOut := false
		select {
		case ev := <-r.internalEvents:
			eventsToGenerate[ev]++
			// got event
		case <-time.After(1 * time.Millisecond):
			timedOut = true
		}
		if timedOut {
			for k := range eventsToGenerate {
				// generate appropriate event
				r.events <- k
				delete(eventsToGenerate, k)
			}

		}
	}
}
