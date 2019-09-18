package cluster

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

func (cf *clusterfunkCluster) createRaft() error {
	cf.mutex.Lock()
	defer cf.mutex.Unlock()

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(cf.config.NodeID)

	if cf.config.Verbose {
		config.LogLevel = "DEBUG"
	} else {
		config.Logger = hclog.NewNullLogger()
	}

	addr, err := net.ResolveTCPAddr("tcp", cf.config.RaftEndpoint)
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
	cf.raftEndpoint = string(transport.LocalAddr())
	if err != nil {
		return err
	}

	var logStore raft.LogStore
	var stableStore raft.StableStore
	var snapshotStore raft.SnapshotStore

	if cf.config.DiskStore {
		raftdir := fmt.Sprintf("./%s", cf.config.NodeID)
		log.Printf("Using boltDB and snapshot store in %s", raftdir)
		if err := os.MkdirAll(raftdir, os.ModePerm); err != nil {
			log.Printf("Unable to create store dir: %v", err)
			return err
		}
		boltDB, err := raftboltdb.NewBoltStore(filepath.Join(raftdir, fmt.Sprintf("%s.db", cf.config.NodeID)))
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

	cf.ra, err = raft.NewRaft(config, newStateMachine(), logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	if cf.config.Bootstrap {
		log.Printf("Bootstrapping new cluster")
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		f := cf.ra.BootstrapCluster(configuration)
		if f.Error() != nil {
			return f.Error()
		}
	}
	observerChan := make(chan raft.Observation)
	go func(ch chan raft.Observation) {
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
				log.Printf("**** Leader observation: %+v. Last index = %d", v, cf.ra.LastIndex())
			case raft.RaftState:
				switch v {
				case raft.Candidate:
					candidateTime = time.Now()
				case raft.Follower:
					printTime(candidateTime, time.Now())
				case raft.Leader:
					printTime(candidateTime, time.Now())
				}
				log.Printf("Raft state: %s", v.String())
				cf.serfNode.SetTag(NodeRaftState, v.String())
				cf.serfNode.PublishTags()
			case *raft.RequestVoteRequest:
				log.Printf("Request vote: %+v", *v)
			}
		}
	}(observerChan)

	cf.ra.RegisterObserver(raft.NewObserver(observerChan, true, func(*raft.Observation) bool { return true }))
	log.Printf("Created Raft instance, binding to %s", cf.raftEndpoint)
	return nil
}

// Note: This is not thread safe
func (cf *clusterfunkCluster) shutdownRaft() {
	cf.mutex.Lock()
	defer cf.mutex.Unlock()
	if cf.ra == nil {
		return
	}
	if cf.ra.VerifyLeader().Error() == nil {
		log.Printf("I'm the leader. Must transfer")
		if cf.ra.RemoveServer(raft.ServerID(cf.config.NodeID), 0, 0).Error() != nil {
			log.Printf("Error removing myself!")
		}
	}
	if err := cf.ra.Shutdown().Error(); err != nil {
		log.Printf("Error leaving Raft cluster: %v", err)
	}
	cf.ra = nil
}

func (cf *clusterfunkCluster) addNodeToRaftCluster(nodeID, bindAddress string) error {

	if cf.ra == nil {
		return errors.New("raft cluster is nil")
	}
	if err := cf.ra.VerifyLeader().Error(); err != nil {
		// Not the leader so can't add node
		return nil
	}
	log.Printf("Joining server: %s", bindAddress)
	configFuture := cf.ra.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(bindAddress) {
			// it's already joined
			return nil
		}
	}

	f := cf.ra.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(bindAddress), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}
	log.Printf("%s joined cluster with node ID %s", bindAddress, nodeID)
	return nil
}

func (cf *clusterfunkCluster) removeNodeFromRaftCluster(nodeID, bindAddress string) {

	if cf.ra == nil {
		return
	}
	if cf.ra.VerifyLeader().Error() != nil {
		return
	}
	configFuture := cf.ra.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Printf("Error reading config: %v", err)
		return
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(bindAddress) {
			cf.ra.RemoveServer(raft.ServerID(nodeID), 0, 0)
			return
		}
	}
}
