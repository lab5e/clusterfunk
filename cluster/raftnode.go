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
	//RaftNodeAdded is emitted when a new node is added
	RaftNodeAdded RaftEventType = iota
	// RaftNodeRemoved is emitted when a node is removed
	RaftNodeRemoved
	// RaftLeaderLost is emitted when the leader is lost, ie the node enters the candidate state
	RaftLeaderLost
	// RaftBecameLeader is emitted when the leader becomes the leader
	RaftBecameLeader
	// RaftBecameFollower is emitted when the node becomes a follower
	RaftBecameFollower
	// RaftReceivedLog is emitted when a log entry is receievd
	RaftReceivedLog
)

// RaftEvent is an event emitted by the RaftNode type
type RaftEvent struct {
	Type    RaftEventType // Type is the event type
	NodeID  string        // NodeID is the node ID of the source
	Index   uint64        // Index is a log entry index (if relevant)
	LogType byte          // Log type identifier from log
}

// RaftNode is a wrapper for the Raft library
type RaftNode struct {
	mutex        *sync.RWMutex
	localNodeID  string
	raftEndpoint string
	ra           *raft.Raft
	events       chan RaftEvent
	fsm          *raftFSM
}

// NewRaftNode creates a new RaftNode instance
func NewRaftNode() *RaftNode {
	return &RaftNode{
		localNodeID: "",
		mutex:       &sync.RWMutex{},
		events:      make(chan RaftEvent, 2), // tiny buffer here to make multiple events feasable.
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

	go r.observerFunc(observerChan)

	r.ra.RegisterObserver(raft.NewObserver(observerChan, true, func(*raft.Observation) bool { return true }))
	r.localNodeID = nodeID

	return nil
}

func (r *RaftNode) sendEvent(e RaftEvent) {
	select {
	case r.events <- e:
	default:
		log.Printf("**** Nobody's listening to me! ev = %+v", e)
	}
}

func (r *RaftNode) logObserver(ch chan fsmLogEvent) {
	for ev := range ch {
		r.sendEvent(RaftEvent{
			Type:    RaftReceivedLog,
			NodeID:  string(r.localNodeID),
			Index:   ev.Index,
			LogType: ev.LogType,
		})
	}
}
func (r *RaftNode) observerFunc(ch chan raft.Observation) {
	for k := range ch {
		switch v := k.Data.(type) {
		case raft.PeerObservation:
			if v.Removed {
				r.sendEvent(RaftEvent{
					Type:   RaftNodeRemoved,
					NodeID: string(v.Peer.ID),
				})
				continue
			}
			r.sendEvent(RaftEvent{
				Type:   RaftNodeAdded,
				NodeID: string(v.Peer.ID),
			})
		case raft.LeaderObservation:
			// This can be ignored since we're monitoring the state
			// and are getting the leader info via other channels.
		case raft.RaftState:
			switch v {
			case raft.Candidate:
				r.sendEvent(RaftEvent{
					Type:   RaftLeaderLost,
					NodeID: "",
				})
			case raft.Follower:
				r.sendEvent(RaftEvent{
					Type:   RaftBecameFollower,
					NodeID: r.localNodeID,
				})
			case raft.Leader:
				r.sendEvent(RaftEvent{
					Type:   RaftBecameLeader,
					NodeID: r.localNodeID,
				})
			}
		case raft.PeerLiveness:
			lt, ok := k.Data.(raft.PeerLiveness)
			if ok {
				if !lt.Heartbeat {
					log.Printf("Peer %s has gone away", lt.ID)
				}
				if lt.Heartbeat {
					log.Printf("Peer %s is back up", lt.ID)
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

	log.Printf("Joining server: %s", endpoint)
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
	log.Printf("%s joined cluster with node ID %s", endpoint, nodeID)
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

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(endpoint) {
			return r.ra.RemoveServer(raft.ServerID(nodeID), 0, 0).Error()
		}
	}

	return errors.New("unknown member node")
}

// Endpoint returns the Raft endpoint (aka bind address)
func (r *RaftNode) Endpoint() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.raftEndpoint
}

// Leader returns true if this node is the leader
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
func (r *RaftNode) AppendLogEntry(data []byte) error {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return errors.New("raft node not started")
	}
	return r.ra.Apply(data, time.Second*2).Error()
}

// Members returns a list of the node IDs that are a member of the cluster
func (r *RaftNode) Members() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return []string{}
	}
	config := r.ra.GetConfiguration()
	if err := config.Error(); err != nil {
		panic(fmt.Sprintf("Unable to read member list from Raft: %v", err))
	}

	members := config.Configuration().Servers
	ret := make([]string, len(members))
	for i, v := range members {
		ret[i] = string(v.ID)
	}
	return ret
}

// -----------------------------------------------------------------------------
// This is temporary methods that are used in the management code

// MemberCount returns the number of members in the Raft cluster
func (r *RaftNode) MemberCount() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return 0
	}
	cfg := r.ra.GetConfiguration()
	if cfg.Error() != nil {
		return 0
	}
	return len(cfg.Configuration().Servers)
}

// State returns the Raft server's state
func (r *RaftNode) State() string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if r.ra == nil {
		return ""
	}
	return r.ra.State().String()
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
func (r *RaftNode) Events() <-chan RaftEvent {
	return r.events
}

// GetReplicatedLogMessage returns the replicated log message with the
// specified type ID
func (r *RaftNode) GetReplicatedLogMessage(id byte) []byte {
	return r.fsm.Entry(id)
}
