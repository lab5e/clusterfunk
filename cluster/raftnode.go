package cluster

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

// TODO(stalehd): Dependency on SerfNode isn't kosher (and probably useless)

// RaftNode is a wrapper for the Raft library
type RaftNode struct {
	mutex         *sync.RWMutex
	localNodeID   string
	localSerfNode *SerfNode
	raftEndpoint  string
	ra            *raft.Raft
}

// NewRaftNode creates a new RaftNode instance
func NewRaftNode(localSerfNode *SerfNode) *RaftNode {
	return &RaftNode{
		localNodeID:   "",
		mutex:         &sync.RWMutex{},
		localSerfNode: localSerfNode,
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
		config.Logger = hclog.NewNullLogger()
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
	config.HeartbeatTimeout = 100 * time.Millisecond
	config.ElectionTimeout = 100 * time.Millisecond
	config.LeaderLeaseTimeout = 50 * time.Millisecond

	transport, err := raft.NewTCPTransport(addr.String(), addr, 3, 5*time.Second, os.Stderr)
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

	r.ra, err = raft.NewRaft(config, newStateMachine(), logStore, stableStore, snapshotStore, transport)
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
	log.Printf("Created Raft instance, binding to %s", transport.LocalAddr())
	r.localNodeID = nodeID
	return nil
}

func (r *RaftNode) observerFunc(ch chan raft.Observation) {
	printTime := func(start time.Time, end time.Time) {
		d := float64(end.Sub(start)) / float64(time.Millisecond)
		log.Printf("%f milliseconds for election", d)
	}
	var candidateTime = time.Now()
	for k := range ch {
		switch v := k.Data.(type) {
		case raft.PeerObservation:
			log.Printf("Peer observation: Removed: %t Peer: %s", v.Removed, v.Peer.ID)
		case raft.LeaderObservation:
			log.Printf("**** Leader observation: %+v", v)
		case raft.RaftState:
			switch v {
			case raft.Candidate:
				candidateTime = time.Now()
			case raft.Follower:
				printTime(candidateTime, time.Now())
				log.Printf("Raft state: %s", v.String())
				r.localSerfNode.SetTag(NodeRaftState, v.String())
				r.localSerfNode.PublishTags()
			case raft.Leader:
				printTime(candidateTime, time.Now())
				log.Printf("Raft state: %s", v.String())
				r.localSerfNode.SetTag(NodeRaftState, v.String())
				r.localSerfNode.PublishTags()
			}
		case *raft.RequestVoteRequest:
			log.Printf("Request vote: %+v", *v)
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
