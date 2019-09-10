package cluster

import (
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/go-hclog"
	raftboltdb "github.com/hashicorp/raft-boltdb"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// hordeCluster implements the Cluster interface
type clatteringCluster struct {
	se           *serf.Serf
	ra           *raft.Raft
	config       Parameters
	raftEndpoint string
	tags         map[string]string
	mutex        *sync.Mutex
}

// New returns a new cluster (client)
func New(params Parameters) Cluster {
	return &clatteringCluster{
		config: params,
		tags:   make(map[string]string),
		mutex:  &sync.Mutex{},
	}
}

func (hc *clatteringCluster) Start() error {
	hc.config.final()
	if hc.config.ZeroConf {
		log.Printf("Registering Serf in mDNS")
		if err := zeroconfRegister(hc.config.SerfEndpoint); err != nil {
			return err
		}
		if !hc.config.Bootstrap && hc.config.Join == "" {
			log.Printf("Looking for other Serf instances...")
			var err error
			hc.config.Join, err = zeroconfLookup(hc.config.SerfEndpoint)
			if err != nil {
				return err
			}
		}
	}

	if err := hc.createRaft(); err != nil {
		return err
	}

	if err := hc.createSerf(); err != nil {
		log.Printf("Error creating Serf client: %v", err)
		return err
	}

	if hc.config.Join != "" {
		if err := hc.joinSerfCluster(); err != nil {
			return err
		}
	}
	return nil
}

// Note: This is not thread safe
func (hc *clatteringCluster) shutdownRaft() {
	if hc.ra == nil {
		return
	}
	if hc.ra.VerifyLeader().Error() == nil {
		log.Printf("I'm the leader. Must transfer")
		if hc.ra.RemoveServer(raft.ServerID(hc.config.NodeID), 0, 0).Error() != nil {
			log.Printf("Error removing myself!")
		}
	}
	if err := hc.ra.Shutdown().Error(); err != nil {
		log.Printf("Error leaving Raft cluster: %v", err)
	}
	hc.ra = nil
}

// Note: This is not thread safe
func (hc *clatteringCluster) shutdownSerf() {
	if hc.se == nil {
		return
	}
	if err := hc.se.Leave(); err != nil {
		log.Printf("Error leaving Serf cluster: %v", err)
	}
	hc.se = nil
}

func (hc *clatteringCluster) Stop() {
	zeroconfShutdown()
	hc.shutdownRaft()
	hc.shutdownSerf()
}

func (hc *clatteringCluster) Nodes() []Node {
	return nil
}

func (hc *clatteringCluster) LocalNode() Node {
	return nil
}

func (hc *clatteringCluster) setTag(name, value string) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	hc.tags[name] = value
}

func (hc *clatteringCluster) AddLocalEndpoint(name, endpoint string) {
	hc.setTag(name, endpoint)
	hc.mutex.Lock()
	defer hc.mutex.Unlock()
	if hc.se != nil {
		hc.se.SetTags(hc.tags)
	}
}

func (hc *clatteringCluster) createRaft() error {

	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(hc.config.NodeID)

	if hc.config.Verbose {
		config.LogLevel = "DEBUG"
	} else {
		config.Logger = hclog.NewNullLogger()
	}

	addr, err := net.ResolveTCPAddr("tcp", hc.config.RaftEndpoint)
	if err != nil {
		return err
	}
	//TODO(stalehd): Check timeouts
	transport, err := raft.NewTCPTransport(addr.String(), addr, 3, 5*time.Second, os.Stderr)
	hc.raftEndpoint = string(transport.LocalAddr())
	if err != nil {
		return err
	}

	var logStore raft.LogStore
	var stableStore raft.StableStore
	var snapshotStore raft.SnapshotStore

	if hc.config.DiskStore {
		raftdir := fmt.Sprintf("./%s", hc.config.NodeID)
		log.Printf("Using boltDB and snapshot store in %s", raftdir)
		if err := os.MkdirAll(raftdir, os.ModePerm); err != nil {
			log.Printf("Unable to create store dir: %v", err)
			return err
		}
		boltDB, err := raftboltdb.NewBoltStore(filepath.Join(raftdir, fmt.Sprintf("%s.db", hc.config.NodeID)))
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

	hc.ra, err = raft.NewRaft(config, newStateMachine(), logStore, stableStore, snapshotStore, transport)
	if err != nil {
		return err
	}

	if hc.config.Bootstrap {
		log.Printf("Bootstrapping new cluster")
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      config.LocalID,
					Address: transport.LocalAddr(),
				},
			},
		}
		f := hc.ra.BootstrapCluster(configuration)
		if f.Error() != nil {
			return f.Error()
		}
	}
	observerChan := make(chan raft.Observation)
	go func(ch chan raft.Observation) {
		for k := range ch {
			switch v := k.Data.(type) {
			case raft.PeerObservation:
				log.Printf("Peer observation: Removed: %t Peer: %s", v.Removed, v.Peer.ID)
			case raft.LeaderObservation:
				log.Printf("**** Leader observation: %+v. Last index = %d", v, hc.ra.LastIndex())
			case raft.RaftState:
				log.Printf("Raft state: %s", v.String())
			case *raft.RequestVoteRequest:
				log.Printf("Request vote: %+v", *v)
			}
		}
	}(observerChan)

	hc.ra.RegisterObserver(raft.NewObserver(observerChan, true, func(*raft.Observation) bool { return true }))
	log.Printf("Created Raft instance, binding to %s", hc.raftEndpoint)
	return nil
}

func (hc *clatteringCluster) joinRaftCluster(bindAddress, nodeID string) error {
	if err := hc.ra.VerifyLeader().Error(); err != nil {
		// Not the leader so can't add node
		return nil
	}
	log.Printf("Joining server: %s", bindAddress)
	configFuture := hc.ra.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		return err
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(bindAddress) {
			// it's already joined
			return nil
		}
	}

	f := hc.ra.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(bindAddress), 0, 0)
	if f.Error() != nil {
		return f.Error()
	}
	log.Printf("%s joined cluster with node ID %s", bindAddress, nodeID)
	return nil
}

func (hc *clatteringCluster) leaveRaftCluster(bindAddress, nodeID string) {
	if hc.ra == nil {
		return
	}
	if hc.ra.VerifyLeader().Error() != nil {
		return
	}
	configFuture := hc.ra.GetConfiguration()
	if err := configFuture.Error(); err != nil {
		log.Printf("Error reading config: %v", err)
		return
	}

	for _, srv := range configFuture.Configuration().Servers {
		if srv.ID == raft.ServerID(nodeID) && srv.Address == raft.ServerAddress(bindAddress) {
			hc.ra.RemoveServer(raft.ServerID(nodeID), 0, 0)
			return
		}
	}
}

func (hc *clatteringCluster) serfEventHandler(events chan serf.Event) {
	for ev := range events {
		switch ev.EventType() {
		case serf.EventMemberJoin:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				if err := hc.joinRaftCluster(v.Tags[raftEndpoint], v.Tags[raftNodeID]); err != nil {
					log.Printf("Error adding member: %+v", v)
				}
			}
		case serf.EventMemberLeave:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				hc.leaveRaftCluster(v.Tags[raftEndpoint], v.Tags[raftNodeID])
			}
		case serf.EventMemberReap:
		case serf.EventMemberUpdate:
		case serf.EventUser:
		case serf.EventQuery:

		default:
			log.Printf("Unknown event: %+v", ev)
		}
	}
}

func (hc *clatteringCluster) createSerf() error {
	log.Printf("Binding Serf client to %s", hc.config.SerfEndpoint)

	config := serf.DefaultConfig()
	config.NodeName = hc.config.NodeID
	host, portStr, err := net.SplitHostPort(hc.config.SerfEndpoint)
	if err != nil {
		return err
	}
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return err
	}
	config.MemberlistConfig.BindAddr = host
	config.MemberlistConfig.BindPort = int(port)

	// The advertise address is the public-reachable address. Since we're running on
	// a LAN (or a LAN-like) infrastructure this is the public IP address of the host.
	config.MemberlistConfig.AdvertiseAddr = host
	config.MemberlistConfig.AdvertisePort = int(port)

	config.SnapshotPath = "" // empty since we're using dynamic clusters.

	config.Init()
	eventCh := make(chan serf.Event)
	config.EventCh = eventCh

	if hc.config.Verbose {
		config.Logger = log.New(os.Stderr, "serf", log.LstdFlags)
	} else {
		log.Printf("Muting Serf events")
		serfLogger := newClusterLogger("serf")
		config.Logger = serfLogger.Logger
		config.MemberlistConfig.Logger = serfLogger.Logger
	}

	// Assign tags and default endpoint
	hc.setTag(nodeType, voterKind)
	hc.setTag(raftNodeID, hc.config.NodeID)
	hc.setTag(raftEndpoint, hc.raftEndpoint)
	hc.mutex.Lock()
	config.Tags = hc.tags
	hc.mutex.Unlock()

	go hc.serfEventHandler(eventCh)

	if hc.se, err = serf.Create(config); err != nil {
		return err
	}
	return nil
}

func (hc *clatteringCluster) joinSerfCluster() error {
	joined, err := hc.se.Join([]string{hc.config.Join}, true)
	if err != nil {
		return err
	}
	log.Printf("Joined %d nodes\n", joined)
	return nil
}
