package cluster

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/stalehd/clusterfunk/utils"
	"google.golang.org/grpc"

	"github.com/hashicorp/raft"
)

// clusterfunkClusterß implements the Cluster interface
type clusterfunkCluster struct {
	serfNode     *SerfNode
	ra           *raft.Raft
	config       Parameters
	raftEndpoint string
	tags         map[string]string
	mutex        *sync.RWMutex
	registry     *ZeroconfRegistry
	name         string
	mgmtServer   *grpc.Server
	state        NodeState
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters) Cluster {
	return &clusterfunkCluster{
		config: params,
		tags:   make(map[string]string),
		mutex:  &sync.RWMutex{},
		name:   params.ClusterName,
		state:  Initializing,
	}
}

func (cf *clusterfunkCluster) Start() error {
	cf.config.final()
	if cf.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}
	log.Printf("Launch config: %+v", cf.config)

	cf.serfNode = NewSerfNode()

	// Launch node management endpoint
	if err := cf.startManagementServices(); err != nil {
		log.Printf("Error starting management endpoint: %v", err)
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

	cf.state = ReadyToJoin

	cf.serfNode.SetTag(NodeType, cf.config.NodeType())
	cf.serfNode.SetTag(RaftNodeID, cf.config.NodeID)
	cf.serfNode.SetTag(RaftEndpoint, cf.raftEndpoint)
	cf.serfNode.SetTag(SerfEndpoint, cf.config.SerfEndpoint)

	if cf.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			for ev := range ch {
				if ev.Joined {
					if err := cf.addNodeToRaftCluster(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
						log.Printf("Error adding member: %+v", ev)
					}
					continue
				}
				cf.removeNodeFromRaftCluster(ev.NodeID, ev.Tags[RaftEndpoint])
				log.Printf("Removing member: %+v", ev)
			}
		}(cf.serfNode.Events())

	}

	if err := cf.serfNode.Start(cf.config.NodeID, cf.config.Verbose, cf.config.SerfEndpoint, cf.config.Bootstrap, cf.config.Join); err != nil {
		return err
	}

	go func() {
		for {
			time.Sleep(5 * time.Second)
			// note: needs a mutex when raft cluster shuts down. If a panic
			// is raised when everything is going down the cluster will be
			// up s**t creek
			if cf.ra.VerifyLeader().Error() == nil {
				start := time.Now()
				if err := cf.ra.Apply(make([]byte, 98999), time.Second*5).Error(); err != nil {
					log.Printf("Error writing log entry: %v", err)
					continue
				}
				end := time.Now()
				diff := end.Sub(start)
				log.Printf("Time to apply log: %f ms", float64(diff)/float64(time.Millisecond))
			}
		}
	}()
	return nil
}

func (cf *clusterfunkCluster) Stop() {
	cf.shutdownRaft()
	cf.serfNode.Stop()
}

func (cf *clusterfunkCluster) Name() string {
	return cf.name
}

func (cf *clusterfunkCluster) Nodes() []Node {

	return nil
}

func (cf *clusterfunkCluster) LocalNode() Node {
	return nil
}

func (cf *clusterfunkCluster) AddLocalEndpoint(name, endpoint string) {
	cf.serfNode.SetTag(name, endpoint)
}
