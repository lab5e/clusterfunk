package cluster

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/golang/protobuf/proto"
	"github.com/stalehd/clusterfunk/cluster/clusterproto"

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
	lastProcessedIndex  uint64
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
	}
	go ret.clusterStateMachine()
	return ret
}

func (c *clusterfunkCluster) Start() error {
	c.config.Final()
	if c.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}

	c.setLocalState(Starting)
	c.serfNode = NewSerfNode()

	// Launch node management endpoint
	if err := c.startManagementServices(); err != nil {
		log.WithError(err).Error("Unable to start management endpoint")
	}
	if err := c.startLeaderService(); err != nil {
		log.WithError(err).Error("Error starting leader gRPC interface")
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
		log.WithFields(log.Fields{
			"eventType":      "raft",
			"state":          e.String(),
			"deltaSinceLast": deltaT(),
		}).Debug("Event received")
		switch e {
		case RaftClusterSizeChanged:
			log.WithFields(log.Fields{
				"count":   c.raftNode.Nodes.Size(),
				"members": c.raftNode.Nodes.List(),
			}).Debug("Member list")
			c.setFSMState(clusterSizeChanged, "")
		case RaftLeaderLost:
			c.setFSMState(leaderLost, "")
		case RaftBecameLeader:
			c.setFSMState(assumeLeadership, "")
		case RaftBecameFollower:
			c.setFSMState(assumeFollower, "")
		case RaftReceivedLog:
			c.processReplicatedLog()
		default:
			log.WithField("eventType", e).Error("Unknown event received")
		}
	}
}

func (c *clusterfunkCluster) Role() NodeRole {
	return NodeRole(atomic.LoadInt32(c.localRole))
}

func (c *clusterfunkCluster) setRole(newRole NodeRole) {
	atomic.StoreInt32(c.localRole, int32(newRole))
}

func (c *clusterfunkCluster) processReplicatedLog() {
	messages := c.raftNode.GetLogMessages(c.lastProcessedIndex)
	for _, msg := range messages {
		if msg.Index > c.lastProcessedIndex {
			c.lastProcessedIndex = msg.Index
		}
		switch msg.MessageType {
		case ProposedShardMap:
			if c.Role() == Leader {
				// Ack to myself - this skips the whole gRPC call
				c.setFSMState(ackReceived, c.raftNode.LocalNodeID())
				continue
			}
			c.processProposedShardMap(&msg)
			c.setFSMState(newShardMapReceived, "")
			c.ackShardMap(msg.AckEndpoint)

		case ShardMapCommitted:
			if c.Role() == Leader {
				// I'm the leader so the map is already committed
				continue
			}
			wantedIndex := atomic.LoadUint64(c.wantedShardLogIndex)
			if wantedIndex > 0 && msg.Index < wantedIndex {
				continue
			}
			// Update internal node list to reflect the list of nodes. Do a
			// quick sanity check on the node list to make sure this node
			// is a member (we really should)
			c.processShardMapCommitMessage(&msg)
			log.WithFields(log.Fields{
				"size":    c.raftNode.Nodes.Size(),
				"members": c.raftNode.Nodes.List(),
			}).Debug("Node list updated from shard map")
			c.setLocalState(Operational)
			c.dumpShardMap()

		default:
			log.WithField("logType", msg.MessageType).Error("Unknown log type in replication log")
		}
	}

}

func (c *clusterfunkCluster) processProposedShardMap(msg *LogMessage) {
	if err := c.shardManager.UnmarshalBinary(msg.Data); err != nil {
		panic(fmt.Sprintf("Could not unmarshal shard map from log message: %v", err))
	}
	wantedIndex := atomic.LoadUint64(c.wantedShardLogIndex)
	if wantedIndex > 0 && msg.Index != wantedIndex {
		return
	}
	atomic.StoreUint64(c.reshardingLogIndex, msg.Index)
}
func (c *clusterfunkCluster) processShardMapCommitMessage(msg *LogMessage) {
	if c.raftNode.Leader() {
		return
	}
	commitMsg := &clusterproto.CommitShardMapMessage{}
	if err := proto.Unmarshal(msg.Data, commitMsg); err != nil {
		panic(fmt.Sprintf("Unable to unmarshal commit message: %v", err))
	}

	c.raftNode.Nodes.Sync(commitMsg.Nodes...)
}

func (c *clusterfunkCluster) serfEvents(ch <-chan NodeEvent) {
	if c.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			for ev := range ch {
				if ev.Joined {
					if c.config.AutoJoin && c.raftNode.Leader() {
						if err := c.raftNode.AddClusterNode(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
							log.WithError(err).WithField("event", ev).Error("Error adding member")
						}
					}
					continue
				}
				if c.config.AutoJoin && c.raftNode.Leader() {
					if err := c.raftNode.RemoveClusterNode(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
						log.WithError(err).WithField("event", ev).Error("Error removing member")
					}
				}
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

func (c *clusterfunkCluster) AddLocalEndpoint(name, endpoint string) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	if c.serfNode == nil {
		return
	}
	c.serfNode.SetTag(name, endpoint)
	if err := c.serfNode.PublishTags(); err != nil {
		log.WithError(err).Error("Error adding endpoint")
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
