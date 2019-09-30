package cluster

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stalehd/clusterfunk/cluster/sharding"
	"github.com/stalehd/clusterfunk/utils"
	"google.golang.org/grpc"
)

// clusterfunkClusterß implements the Cluster interface
type clusterfunkCluster struct {
	serfNode            *SerfNode
	raftNode            *RaftNode
	config              Parameters
	registry            *utils.ZeroconfRegistry
	name                string
	localState          *int32
	localRole           *int32
	mgmtServer          *grpc.Server // gRPC server for management
	leaderServer        *grpc.Server // gRPC server for leader
	eventChannels       []chan Event
	mutex               *sync.RWMutex
	stateChannel        chan fsmEvent
	shardManager        sharding.ShardManager
	reshardingLogIndex  *uint64
	wantedShardLogIndex *uint64
	nodes               map[string]Node
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters, shardManager sharding.ShardManager) Cluster {
	reshardIndex := new(uint64)
	atomic.StoreUint64(reshardIndex, 0)

	state := new(int32)
	role := new(int32)
	wantedIndex := new(uint64)

	atomic.StoreInt32(state, int32(Invalid))
	atomic.StoreInt32(role, int32(Unknown))
	ret := &clusterfunkCluster{
		config:              params,
		name:                params.ClusterName,
		localState:          state,
		localRole:           role,
		eventChannels:       make([]chan Event, 0),
		mutex:               &sync.RWMutex{},
		stateChannel:        make(chan fsmEvent, 5),
		shardManager:        shardManager,
		reshardingLogIndex:  reshardIndex,
		wantedShardLogIndex: wantedIndex,
		nodes:               make(map[string]Node, 0),
	}
	go ret.clusterStateMachine()
	return ret
}

func (c *clusterfunkCluster) Start() error {
	c.config.final()
	if c.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}

	c.setLocalState(Starting)
	c.serfNode = NewSerfNode()

	// Launch node management endpoint
	if err := c.startManagementServices(); err != nil {
		log.Printf("Error starting management endpoint: %v", err)
	}
	if err := c.startLeaderService(); err != nil {
		log.Printf("Error starting leader RPC: %v", err)
	}
	if c.config.ZeroConf {
		c.registry = utils.NewZeroconfRegistry(c.config.ClusterName)

		if !c.config.Raft.Bootstrap && c.config.Serf.JoinAddress == "" {
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
		if err := c.registry.Register(c.config.NodeID, utils.PortOfHostPort(c.config.Serf.Endpoint)); err != nil {
			return err
		}

	}
	c.raftNode = NewRaftNode()

	go c.raftEvents(c.raftNode.Events())

	if err := c.raftNode.Start(c.config.NodeID, c.config.Verbose, c.config.Raft); err != nil {
		return err
	}

	c.serfNode.SetTag(RaftEndpoint, c.raftNode.Endpoint())
	c.serfNode.SetTag(SerfEndpoint, c.config.Serf.Endpoint)

	go c.serfEvents(c.serfNode.Events())

	if err := c.serfNode.Start(c.config.NodeID, c.config.Verbose, c.config.Serf); err != nil {
		return err
	}

	return nil
}

func (c *clusterfunkCluster) raftEvents(ch <-chan RaftEventType) {
	lastTime := time.Now()
	deltaT := func() float64 {
		t := time.Now()
		diff := t.Sub(lastTime)
		lastTime = t
		return float64(diff) / float64(time.Millisecond)
	}

	for e := range ch {
		log.Printf("RAFT: %s (%f ms since last event)", e.String(), deltaT())
		switch e {
		case RaftClusterSizeChanged:
			log.Printf("%d members:  %+v ", c.raftNode.MemberCount(), c.raftNode.Members())

		case RaftLeaderLost:

		case RaftBecameLeader:

		case RaftBecameFollower:

		case RaftReceivedLog:

		default:
			log.Printf("Unknown event received: %+v", e)
		}
	}
}

func (c *clusterfunkCluster) Role() NodeRole {
	return NodeRole(atomic.LoadInt32(c.localRole))
}

func (c *clusterfunkCluster) setRole(newRole NodeRole) {
	atomic.StoreInt32(c.localRole, int32(newRole))
}

func (c *clusterfunkCluster) processReplicatedLog(t LogMessageType, index uint64) {
	msg := c.raftNode.GetReplicatedLogMessage(t)
	c.updateNodes(c.shardManager.NodeList())
	switch msg.MessageType {
	case ProposedShardMap:
		if c.Role() == Leader {
			log.Printf("Already have an updated shard map")
			// Ack to myself
			c.setFSMState(ackReceived, c.raftNode.LocalNodeID())
			return
		}
		if err := c.shardManager.UnmarshalBinary(msg.Data); err != nil {
			panic(fmt.Sprintf("Could not unmarshal shard map from log message: %v", err))
		}
		wantedIndex := atomic.LoadUint64(c.wantedShardLogIndex)
		if wantedIndex > 0 && index != wantedIndex {
			log.Printf("Ignoring shard map with index %d", index)
			return
		}
		atomic.StoreUint64(c.reshardingLogIndex, index)
		c.setFSMState(newShardMapReceived, "")
		c.ackShardMap(msg.AckEndpoint)
	case ShardMapCommitted:
		wantedIndex := atomic.LoadUint64(c.wantedShardLogIndex)
		if wantedIndex > 0 && index < wantedIndex {
			log.Printf("Ignoring commit message with index %d", index)
			return
		}
		log.Printf("Shard map is comitted by leader. Start serving!")
		c.dumpShardMap()

	default:
		log.Printf("Don't know how to process log type %d", t)
	}
}

func (c *clusterfunkCluster) serfEvents(ch <-chan NodeEvent) {
	if c.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			resize := false
			for ev := range ch {
				if ev.Update {
					// Update node list
					c.updateNode(ev.NodeID, ev.Tags)
					continue
				}
				if ev.Joined {
					// add node to list
					if c.addNode(ev.NodeID) {
						c.updateNode(ev.NodeID, ev.Tags)
						resize = resize && true
					}

					if c.config.AutoJoin && c.raftNode.Leader() {
						if err := c.raftNode.AddMember(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
							log.Printf("Error adding member: %v - %+v", err, ev)
						}
					}
					continue
				}
				resize = resize && c.removeNode(ev.NodeID)
				if c.config.AutoJoin && c.raftNode.Leader() {
					if err := c.raftNode.RemoveMember(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
						log.Printf("Error removing member: %v - %+v", err, ev)
					}
				}
			}
			if resize {
				c.setFSMState(clusterSizeChanged, "")
			}
		}(c.serfNode.Events())
	}
}

func (c *clusterfunkCluster) Stop() {
	c.setLocalState(Stopping)

	c.mutex.Lock()
	defer c.mutex.Unlock()
	if c.raftNode != nil {
		c.raftNode.Stop()
		c.raftNode = nil
	}
	if c.serfNode != nil {
		c.serfNode.Stop()
		c.serfNode = nil
	}

	c.setRole(Unknown)
	c.setLocalState(Invalid)
}

func (c *clusterfunkCluster) Name() string {
	return c.name
}

func (c *clusterfunkCluster) Nodes() []Node {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	panic("not implemented")
}

func (c *clusterfunkCluster) LocalNode() Node {
	panic("not implemented")
}

func (c *clusterfunkCluster) LeaderNode() Node {
	panic("not implemented")
}

func (c *clusterfunkCluster) AddLocalEndpoint(name, endpoint string) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.serfNode == nil {
		return
	}
	c.serfNode.SetTag(name, endpoint)
	if err := c.serfNode.PublishTags(); err != nil {
		log.Printf("Error adding endpoint: %v", err)
	}
}

func (c *clusterfunkCluster) Events() <-chan Event {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	ret := make(chan Event)
	c.eventChannels = append(c.eventChannels, ret)
	return ret
}

func (c *clusterfunkCluster) sendEvent(ev Event) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	for _, v := range c.eventChannels {

		select {
		case v <- ev:
			// great success
		case <-time.After(1 * time.Second):
			// drop event
		}
	}
}

func (c *clusterfunkCluster) setLocalState(newState NodeState) {
	currentState := NodeState(atomic.LoadInt32(c.localState))
	if currentState != newState {
		atomic.StoreInt32(c.localState, int32(newState))
		go c.sendEvent(Event{LocalState: newState})
	}
}

func (c *clusterfunkCluster) LocalState() NodeState {
	return NodeState(atomic.LoadInt32(c.localState))
}

// -----------------------------------------------------------------------------
// Node operations -- initiated by Serf events

// updateNodes updates our world view
func (c *clusterfunkCluster) updateNodes(nodes []string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	for k := range c.nodes {
		n := c.nodes[k]
		c.nodes[k] = n
	}
	for _, v := range nodes {
		n, exists := c.nodes[v]
		if !exists {
			n = NewNode(v, Follower)
			c.nodes[v] = n
		}
	}

	for _, v := range c.serfNode.Members() {
		n, ok := c.nodes[v.NodeID]
		if !ok {
			n = NewNode(v.NodeID, NonMember)
		}
		c.updateSerfNode(v, &n)
		c.nodes[v.NodeID] = n
	}
}

// updateNode updates the node with new tags
func (c *clusterfunkCluster) updateNode(nodeID string, tags map[string]string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	existing, ok := c.nodes[nodeID]
	if !ok {
		existing = NewNode(nodeID, NonMember)
		c.nodes[nodeID] = existing
	}
	for _, v := range c.serfNode.Members() {
		if v.NodeID == nodeID {
			c.updateSerfNode(v, &existing)
		}
	}
}

func (c *clusterfunkCluster) updateSerfNode(ni SerfMemberInfo, n *Node) {
	n.Tags[SerfStatusKey] = ni.Status
	for k, v := range ni.Tags {
		n.Tags[k] = v
	}
}

// addNode adds a new node
func (c *clusterfunkCluster) addNode(nodeID string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, ok := c.nodes[nodeID]
	if !ok {
		c.nodes[nodeID] = NewNode(nodeID, NonMember)
		return true
	}
	return false
}

// removeNode removes the node
func (c *clusterfunkCluster) removeNode(nodeID string) bool {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	_, ok := c.nodes[nodeID]
	if !ok {
		return false
	}
	delete(c.nodes, nodeID)
	return true
}

func (c *clusterfunkCluster) updateNodeList() bool {
	list := c.raftNode.Members()
	changed := false
	for _, v := range list {
		changed = changed && c.addNode(v)
	}
	return changed
}

func (c *clusterfunkCluster) getNodes() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	var ret []string
	for _, v := range c.nodes {
		ret = append(ret, v.ID)
	}
	return ret
}
