package cluster

import (
	"errors"
	"log"
	"sync"
	"time"

	"github.com/stalehd/clusterfunk/utils"
	"google.golang.org/grpc"
)

// clusterfunkClusterß implements the Cluster interface
type clusterfunkCluster struct {
	serfNode      *SerfNode
	raftNode      *RaftNode
	config        Parameters
	registry      *utils.ZeroconfRegistry
	name          string
	mgmtServer    *grpc.Server
	nodeState     NodeState
	clusterState  State
	eventChannels []chan Event
	mutex         *sync.RWMutex
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters) Cluster {

	return &clusterfunkCluster{
		config:        params,
		name:          params.ClusterName,
		nodeState:     Initializing,
		clusterState:  Startup,
		eventChannels: make([]chan Event, 0),
		mutex:         &sync.RWMutex{},
	}
}

func (c *clusterfunkCluster) Start() error {
	c.config.final()
	if c.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}
	log.Printf("Launch config: %+v", c.config)

	c.serfNode = NewSerfNode()

	// Launch node management endpoint
	if err := c.startManagementServices(); err != nil {
		log.Printf("Error starting management endpoint: %v", err)
	}

	if c.config.ZeroConf {
		c.registry = utils.NewZeroconfRegistry(c.config.ClusterName)

		if !c.config.Raft.Bootstrap && c.config.Serf.JoinAddress == "" {
			log.Printf("Looking for other Serf instances...")
			var err error
			addrs, err := c.registry.Resolve(1 * time.Second)
			if err != nil {
				return err
			}
			if len(addrs) == 0 {
				return errors.New("no serf instances found")
			}
			c.config.Serf.JoinAddress = addrs[0]
		}
		log.Printf("Registering Serf endpoint (%s) in zeroconf", c.config.Serf.Endpoint)
		if err := c.registry.Register(c.config.NodeID, utils.PortOfHostPort(c.config.Serf.Endpoint)); err != nil {
			return err
		}

	}
	// Set state to none here before it's updated by the
	c.serfNode.SetTag(NodeRaftState, StateNone)

	c.raftNode = NewRaftNode()

	go func(ch <-chan RaftEvent) {
		for e := range ch {
			switch e.Type {
			case RaftNodeAdded:
				log.Printf("EVENT: Node %s added", e.NodeID)
				// TODO: Update shard map
				// There's a new node in the cluster - redistribute shards
				c.startRedistribution()
			case RaftNodeRemoved:
				log.Printf("EVENT: Node %s removed", e.NodeID)
				// TODO: Update shard map
				c.startRedistribution()
			case RaftLeaderLost:
				log.Printf("EVENT: Leader lost")
				// Leader is lost - set cluster to unavailable
				c.setState(Unavailable)
			case RaftBecameLeader:
				log.Printf("EVENT: Became leader")
				// I'm the leader. Start resharding
				c.startRedistribution()
				c.serfNode.SetTag(NodeRaftState, StateLeader)
				c.serfNode.PublishTags()
			case RaftBecameFollower:
				// Wait for the leader to redistribute the shards since it's the new leader
				c.setState(Resharding)
				log.Printf("EVENT: Became follower")
				c.serfNode.SetTag(NodeRaftState, StateFollower)
				c.serfNode.PublishTags()
			}
		}
	}(c.raftNode.Events())

	if err := c.raftNode.Start(c.config.NodeID, c.config.Verbose, c.config.Raft); err != nil {
		return err
	}

	c.serfNode.SetTag(NodeType, c.config.NodeType())
	c.serfNode.SetTag(RaftNodeID, c.config.NodeID)
	c.serfNode.SetTag(RaftEndpoint, c.raftNode.Endpoint())
	c.serfNode.SetTag(SerfEndpoint, c.config.Serf.Endpoint)

	if c.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			for ev := range ch {
				if ev.Joined {
					if c.raftNode.Leader() {
						if err := c.raftNode.AddMember(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
							log.Printf("Error adding member: %v - %+v", err, ev)
						}
					}
					continue
				}
				if c.raftNode.Leader() {
					if err := c.raftNode.RemoveMember(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
						log.Printf("Error removing member: %v - %+v", err, ev)
					}
				}
			}
		}(c.serfNode.Events())

	}

	if err := c.serfNode.Start(c.config.NodeID, c.config.Verbose, c.config.Serf); err != nil {
		return err
	}

	go func() {
		for {
			time.Sleep(5 * time.Second)
			// note: needs a mutex when raft cluster shuts down. If a panic
			// is raised when everything is going down the cluster will be
			// up s**t creek
			if c.raftNode.Leader() {
				start := time.Now()
				if err := c.raftNode.AppendLogEntry(make([]byte, 89999)); err != nil {
					log.Printf("Error writing log entry: %v", err)
					continue
				}
				end := time.Now()
				diff := end.Sub(start)
				log.Printf("Time to apply log: %f ms", float64(diff)/float64(time.Millisecond))
			}
		}
	}()

	c.nodeState = Empty
	return nil
}

func (c *clusterfunkCluster) Stop() {
	c.raftNode.Stop()
	c.serfNode.Stop()
}

func (c *clusterfunkCluster) Name() string {
	return c.name
}

func (c *clusterfunkCluster) Nodes() []Node {

	return nil
}

func (c *clusterfunkCluster) LocalNode() Node {
	return nil
}

func (c *clusterfunkCluster) AddLocalEndpoint(name, endpoint string) {
	c.serfNode.SetTag(name, endpoint)
}

func (c *clusterfunkCluster) Events() <-chan Event {
	ret := make(chan Event)
	c.eventChannels = append(c.eventChannels, ret)
	return ret
}

func (c *clusterfunkCluster) startRedistribution() {
	c.setState(Resharding)
	log.Printf(" **** Starting redistribution of shards ")

	c.setState(Operational)

}

func (c *clusterfunkCluster) setState(state State) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.clusterState = state
}

func (c *clusterfunkCluster) State() State {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.clusterState
}

func (c *clusterfunkCluster) WaitForState(state State, timeout time.Duration) error {
	var timeoutch <-chan time.Time
	if timeout == 0 {
		timeoutch = make(chan time.Time)
	} else {
		timeoutch = time.After(timeout)

	}
	for {
		select {
		case <-timeoutch:
			return errors.New("timed out waiting for cluster state")
		default:
			c.mutex.RLock()
			s := c.clusterState
			c.mutex.RUnlock()
			if s == state {
				return nil
			}
			time.Sleep(1 * time.Millisecond)
		}

	}
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
