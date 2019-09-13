package cluster

import (
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/stalehd/clusterfunk/utils"
	"google.golang.org/grpc"

	"github.com/hashicorp/go-hclog"
	raftboltdb "github.com/hashicorp/raft-boltdb"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// hordeCluster implements the Cluster interface
type clusterfunkCluster struct {
	se           *serf.Serf
	ra           *raft.Raft
	config       Parameters
	raftEndpoint string
	tags         map[string]string
	mutex        *sync.Mutex
	registry     *ZeroconfRegistry
	mgmtServer   *grpc.Server
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters) Cluster {
	return &clusterfunkCluster{
		config: params,
		tags:   make(map[string]string),
		mutex:  &sync.Mutex{},
	}
}

func (cf *clusterfunkCluster) Start() error {
	cf.config.final()
	if cf.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}
	if cf.config.ZeroConf {
		cf.registry = NewZeroconfRegistry(cf.config.ClusterName)

		if !cf.config.Bootstrap && cf.config.Join == "" {
			log.Printf("Looking for other Serf instances...")
			var err error
			addrs, err := cf.registry.Resolve(1 * time.Second)
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				return errors.New("no serf instances found")
			}
			cf.config.Join = addrs[0]
		}
		log.Printf("Registering Serf endpoint (%s) in zeroconf", cf.config.SerfEndpoint)
		if err := cf.registry.Register(cf.config.NodeID, utils.PortOfHostPort(cf.config.SerfEndpoint)); err != nil {
			return err
		}

	}

	if err := cf.createRaft(); err != nil {
		return err
	}

	if err := cf.createSerf(); err != nil {
		log.Printf("Error creating Serf client: %v", err)
		return err
	}

	if cf.config.Join != "" {
		if err := cf.joinSerfCluster(); err != nil {
			return err
		}
	}

	// Launch node management endpoint
	cf.startManagementServices()
	// Launch leader management endpoint
	return nil
}

// Note: This is not thread safe
func (cf *clusterfunkCluster) shutdownRaft() {
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

// Note: This is not thread safe
func (cf *clusterfunkCluster) shutdownSerf() {
	if cf.se == nil {
		return
	}
	if err := cf.se.Leave(); err != nil {
		log.Printf("Error leaving Serf cluster: %v", err)
	}
	cf.se = nil
}

func (cf *clusterfunkCluster) Stop() {
	cf.shutdownRaft()
	cf.shutdownSerf()
}

func (cf *clusterfunkCluster) Nodes() []Node {
	return nil
}

func (cf *clusterfunkCluster) LocalNode() Node {
	return nil
}

func (cf *clusterfunkCluster) setTag(name, value string) {
	cf.mutex.Lock()
	defer cf.mutex.Unlock()
	cf.tags[name] = value
}

func (cf *clusterfunkCluster) AddLocalEndpoint(name, endpoint string) {
	cf.setTag(name, endpoint)
	cf.mutex.Lock()
	defer cf.mutex.Unlock()
	if cf.se != nil {
		cf.se.SetTags(cf.tags)
	}
}

func (cf *clusterfunkCluster) createRaft() error {

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
	//TODO(stalehd): Check timeouts
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
		for k := range ch {
			switch v := k.Data.(type) {
			case raft.PeerObservation:
				log.Printf("Peer observation: Removed: %t Peer: %s", v.Removed, v.Peer.ID)
			case raft.LeaderObservation:
				log.Printf("**** Leader observation: %+v. Last index = %d", v, cf.ra.LastIndex())
			case raft.RaftState:
				log.Printf("Raft state: %s", v.String())
			case *raft.RequestVoteRequest:
				log.Printf("Request vote: %+v", *v)
			}
		}
	}(observerChan)

	cf.ra.RegisterObserver(raft.NewObserver(observerChan, true, func(*raft.Observation) bool { return true }))
	log.Printf("Created Raft instance, binding to %s", cf.raftEndpoint)
	return nil
}

func (cf *clusterfunkCluster) joinRaftCluster(bindAddress, nodeID string) error {
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

func (cf *clusterfunkCluster) leaveRaftCluster(bindAddress, nodeID string) {
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

func (cf *clusterfunkCluster) serfEventHandler(events chan serf.Event) {
	for ev := range events {
		switch ev.EventType() {
		case serf.EventMemberJoin:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				if err := cf.joinRaftCluster(v.Tags[raftEndpoint], v.Tags[raftNodeID]); err != nil {
					log.Printf("Error adding member: %+v", v)
				}
			}
		case serf.EventMemberLeave:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				cf.leaveRaftCluster(v.Tags[raftEndpoint], v.Tags[raftNodeID])
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

func (cf *clusterfunkCluster) createSerf() error {
	log.Printf("Binding Serf client to %s", cf.config.SerfEndpoint)

	config := serf.DefaultConfig()
	config.NodeName = cf.config.NodeID
	host, portStr, err := net.SplitHostPort(cf.config.SerfEndpoint)
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

	if cf.config.Verbose {
		config.Logger = log.New(os.Stderr, "serf", log.LstdFlags)
	} else {
		log.Printf("Muting Serf events")
		serfLogger := newClusterLogger("serf")
		config.Logger = serfLogger.Logger
		config.MemberlistConfig.Logger = serfLogger.Logger
	}

	// Assign tags and default endpoint
	cf.setTag(nodeType, cf.config.NodeType())
	cf.setTag(raftNodeID, cf.config.NodeID)
	cf.setTag(raftEndpoint, cf.raftEndpoint)
	cf.mutex.Lock()
	config.Tags = cf.tags
	cf.mutex.Unlock()

	go cf.serfEventHandler(eventCh)

	if cf.se, err = serf.Create(config); err != nil {
		return err
	}
	return nil
}

func (cf *clusterfunkCluster) joinSerfCluster() error {
	joined, err := cf.se.Join([]string{cf.config.Join}, true)
	if err != nil {
		return err
	}
	log.Printf("Joined %d nodes\n", joined)
	return nil
}
