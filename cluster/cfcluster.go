package cluster

import (
	"errors"
	"log"
	"time"

	"github.com/stalehd/clusterfunk/utils"
	"google.golang.org/grpc"
)

// clusterfunkClusterß implements the Cluster interface
type clusterfunkCluster struct {
	serfNode   *SerfNode
	raftNode   *RaftNode
	config     Parameters
	registry   *ZeroconfRegistry
	name       string
	mgmtServer *grpc.Server
	state      NodeState
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters) Cluster {
	return &clusterfunkCluster{
		config: params,
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

		if !cf.config.Raft.Bootstrap && cf.config.Serf.JoinAddress == "" {
			log.Printf("Looking for other Serf instances...")
			var err error
			addrs, err := cf.registry.Resolve(1 * time.Second)
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				return errors.New("no serf instances found")
			}
			cf.config.Serf.JoinAddress = addrs[0]
		}
		log.Printf("Registering Serf endpoint (%s) in zeroconf", cf.config.Serf.Endpoint)
		if err := cf.registry.Register(cf.config.NodeID, utils.PortOfHostPort(cf.config.Serf.Endpoint)); err != nil {
			return err
		}

	}

	cf.raftNode = NewRaftNode(cf.serfNode)
	if err := cf.raftNode.Start(cf.config.NodeID, cf.config.Verbose, cf.config.Raft); err != nil {
		return err
	}

	cf.serfNode.SetTag(NodeType, cf.config.NodeType())
	cf.serfNode.SetTag(RaftNodeID, cf.config.NodeID)
	cf.serfNode.SetTag(RaftEndpoint, cf.raftNode.Endpoint())
	cf.serfNode.SetTag(SerfEndpoint, cf.config.Serf.Endpoint)

	if cf.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			for ev := range ch {
				if ev.Joined {
					if err := cf.raftNode.AddMember(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
						log.Printf("Error adding member: %+v", ev)
					}
					continue
				}
				cf.raftNode.RemoveMember(ev.NodeID, ev.Tags[RaftEndpoint])
				log.Printf("Removing member: %+v", ev)
			}
		}(cf.serfNode.Events())

	}

	if err := cf.serfNode.Start(cf.config.NodeID, cf.config.Verbose, cf.config.Serf); err != nil {
		return err
	}

	go func() {
		for {
			time.Sleep(5 * time.Second)
			// note: needs a mutex when raft cluster shuts down. If a panic
			// is raised when everything is going down the cluster will be
			// up s**t creek
			if cf.raftNode.Leader() {
				start := time.Now()
				if err := cf.raftNode.AppendLogEntry(make([]byte, 89999)); err != nil {
					log.Printf("Error writing log entry: %v", err)
					continue
				}
				end := time.Now()
				diff := end.Sub(start)
				log.Printf("Time to apply log: %f ms", float64(diff)/float64(time.Millisecond))
			}
		}
	}()

	cf.state = Empty
	return nil
}

func (cf *clusterfunkCluster) Stop() {
	cf.raftNode.Stop()
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

/*
type clusterNode struct {
}

func newClusterNode() Node {
	return &clusterNode{}
}

func (cn *clusterNode) ID() string {
	return cn.nodeID
}

func (cn *clusterNode) Shards() []Shard {
	return cn.shards
}

func (cn *clusterNode) Voter() bool {
	return cn.voter
}

func (cn *clusterNode) Leader() bool {
	return cn.leader
}

func (cn *clusterNode) Endpoints() []string {

}

func (cn *clusterNode) GetEndpoint(name string) (string, error) {

}

func (cn *clusterNode) State() NodeState {

}
*/
