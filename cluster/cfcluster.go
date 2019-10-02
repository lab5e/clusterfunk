package cluster

import (
	"context"
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
	shardManager        sharding.ShardManager
	reshardingLogIndex  *uint64
	wantedShardLogIndex *uint64
	lastProcessedIndex  uint64
	unacknowledgedNodes StringSet
	shardMapLogIndex    uint64
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
		shardManager:        shardManager,
		reshardingLogIndex:  reshardIndex,
		wantedShardLogIndex: wantedIndex,
		unacknowledgedNodes: NewStringSet(),
	}
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

	go c.raftEventLoop(c.raftNode.Events())

	if err := c.raftNode.Start(c.config.NodeID, c.config.Verbose, c.config.Raft); err != nil {
		return err
	}

	c.serfNode.SetTag(RaftEndpoint, c.raftNode.Endpoint())
	c.serfNode.SetTag(SerfEndpoint, c.config.Serf.Endpoint)

	go c.serfEventLoop(c.serfNode.Events())

	if err := c.serfNode.Start(c.config.NodeID, c.config.Verbose, c.config.Serf); err != nil {
		return err
	}

	return nil
}

func (c *clusterfunkCluster) raftEventLoop(ch <-chan RaftEventType) {
	for e := range ch {
		log.WithFields(log.Fields{
			"eventType": "raft",
			"state":     e.String(),
		}).Debugf("Raft event: %s", e)
		switch e {
		case RaftClusterSizeChanged:
			c.handleClusterSizeChanged()

		case RaftLeaderLost:
			c.handleLeaderLost()

		case RaftBecameLeader:
			c.handleLeaderEvent()

		case RaftBecameFollower:
			c.handleFollowerEvent()

		case RaftReceivedLog:
			c.handleReceiveLog()

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
func (c *clusterfunkCluster) handleLeaderEvent() {
	c.setRole(Leader)
	// A clustersizechanged-event is emitted by the Raft library when
	// leader is announced. This will force a reshard
}
func (c *clusterfunkCluster) handleLeaderLost() {
	c.setLocalState(Voting)
}
func (c *clusterfunkCluster) handleFollowerEvent() {
	c.setRole(Follower)
}
func (c *clusterfunkCluster) handleClusterSizeChanged() {
	if c.Role() != Leader {
		// I'm not the leader. Won't do anything.
		log.Debugf("Ignoring cluster size change when I'm not the leader")
		return
	}
	log.WithFields(log.Fields{
		"count":   c.raftNode.Nodes.Size(),
		"members": c.raftNode.Nodes.List(),
	}).Error("Cluster size changed 1")

	c.setLocalState(Resharding)
	// reshard cluster, distribute via replicated log.

	// Reset the list of acked nodes.
	list := c.raftNode.Nodes.List()

	c.shardManager.UpdateNodes(list...)
	proposedShardMap, err := c.shardManager.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Can't marshal the shard map: %v", err))
	}
	mapMessage := NewLogMessage(ProposedShardMap, c.config.LeaderEndpoint, proposedShardMap)
	c.unacknowledgedNodes.Sync(list...)
	log.Debugf("Unacknowledged nodes: %+v", c.unacknowledgedNodes)
	// Replicate proposed shard map via log
	buf, err := mapMessage.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Unable to marshal the log message containing shard map: %v", err))
	}
	index, err := c.raftNode.AppendLogEntry(buf)
	if err != nil {
		if c.raftNode.Leader() {
			panic("I'm the leader but I could not write the log")
		}
		// otherwise -- just log it and continue
		log.WithError(err).Error("Could not write log entry for new shard map")
	}
	atomic.StoreUint64(c.reshardingLogIndex, index)
	// This is the index we want commits for.
	c.shardMapLogIndex = c.raftNode.LastLogIndex()
	log.WithFields(log.Fields{"index": c.shardMapLogIndex}).Debugf("Shard map index")
	//c.updateNodes(c.shardManager.NodeList())
	// Next messages will be ackReceived when the changes has replicated
	// out to the other nodes.
	// No new state here - wait for a series of ackReceived states
	// from the nodes.

	log.WithFields(log.Fields{
		"count":   c.raftNode.Nodes.Size(),
		"members": c.raftNode.Nodes.List(),
	}).Error("Cluster size changed 2")

}

func (c *clusterfunkCluster) handleAckReceived(nodeID string) {
	// when a new ack message is received the ack is noted for the node and
	// until all nodes have acked the state will stay the same.
	log.Infof("1 Unacknowledged nodes = %+v.", c.unacknowledgedNodes.Strings)
	c.unacknowledgedNodes.Remove(nodeID)
	log.Infof("2 Unacknowledged nodes = %+v.", c.unacknowledgedNodes.Strings)
	log.WithFields(log.Fields{"nodeid": nodeID, "remaining": c.unacknowledgedNodes.Size()}).Debug("STATE: Ack received")
	// Timeouts are handled when calling the other nodes via gRPC

	if c.unacknowledgedNodes.Size() == 0 {
		log.Infof("Unacknowledged nodes = %+v. Operational next", c.unacknowledgedNodes.Strings)
		// ack is completed. Enable the new shard map for the cluster by
		// sending a commit log message. No further processing is required
		// here.
		c.sendCommitMessage(c.shardMapLogIndex)
		c.setLocalState(Operational)

	}
	return // continue

}
func (c *clusterfunkCluster) handleReceiveLog() {
	messages := c.raftNode.GetLogMessages(c.lastProcessedIndex)
	for _, msg := range messages {
		if msg.Index > c.lastProcessedIndex {
			c.lastProcessedIndex = msg.Index
		}
		switch msg.MessageType {
		case ProposedShardMap:
			if c.Role() == Leader {
				// Ack to myself - this skips the whole gRPC call
				log.Infof("Acking myself (%s)", c.raftNode.LocalNodeID())
				c.handleAckReceived(c.raftNode.LocalNodeID())
				continue
			}
			c.processProposedShardMap(&msg)
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
			}).Debug("Node list updated from commit message")
			c.setLocalState(Operational)

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

func (c *clusterfunkCluster) ackShardMap(endpoint string) {

	// Step 1 Leader ID
	// Confirm the shard map
	clientParam := utils.GRPCClientParam{
		ServerEndpoint: endpoint,
		TLS:            false,
		CAFile:         "",
	}
	opts, err := utils.GetGRPCDialOpts(clientParam)
	if err != nil {
		//panic(fmt.Sprintf("Unable to acknowledge gRPC client parameters: %v", err))
		log.WithError(err).Error("Unable to acknowledge gRPC client parameters")
		return
	}
	conn, err := grpc.Dial(clientParam.ServerEndpoint, opts...)
	if err != nil {
		log.WithError(err).Error("Unable to dial server when acking shard map")
		return
	}
	defer conn.Close()
	logIndex := atomic.LoadUint64(c.reshardingLogIndex)

	client := clusterproto.NewClusterLeaderServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	resp, err := client.ConfirmShardMap(ctx, &clusterproto.ConfirmShardMapRequest{
		NodeID:   c.raftNode.LocalNodeID(),
		LogIndex: int64(logIndex),
	})
	if err != nil {
		log.WithError(err).Error("Unable to confirm shard map")
		return
	}
	if !resp.Success {
		if uint64(resp.CurrentIndex) == logIndex {
			// It's only an error if the index is the same
			log.WithFields(log.Fields{
				"index":       logIndex,
				"leaderIndex": resp.CurrentIndex,
			}).Error("Leader rejected ack")
		}
		atomic.StoreUint64(c.wantedShardLogIndex, uint64(resp.CurrentIndex))
		return
	}
	log.WithFields(log.Fields{
		"leaderEndpoint": endpoint,
		"logIndex":       logIndex,
	}).Debug("Shard map ack successfully sent to leader")
	atomic.StoreUint64(c.wantedShardLogIndex, 0)
}

func (c *clusterfunkCluster) sendCommitMessage(index uint64) {
	commitMsg := clusterproto.CommitShardMapMessage{
		ShardMapLogIndex: int64(index),
	}
	for _, v := range c.raftNode.Nodes.List() {
		commitMsg.Nodes = append(commitMsg.Nodes, v)
	}
	commitBuf, err := proto.Marshal(&commitMsg)
	if err != nil {
		panic(fmt.Sprintf("Unable to marshal commit message buffer: %v", err))
	}
	commitMessage := NewLogMessage(ShardMapCommitted, c.raftNode.LocalNodeID(), commitBuf)
	buf, err := commitMessage.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Unable to marshal commit message: %v", err))
	}
	if _, err := c.raftNode.AppendLogEntry(buf); err != nil {
		// We might have lost the leadership here. Panic if we're still
		// the leader
		if err := c.raftNode.ra.VerifyLeader().Error(); err == nil {
			panic("I'm the leader but I could not write the log")
		}
		// otherwise -- just log it and continue
		log.WithError(err).Error("Could not write log entry for new shard map")
	}
}

func (c *clusterfunkCluster) serfEventLoop(ch <-chan NodeEvent) {
	if c.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			for ev := range ch {
				if ev.Joined {
					if c.config.AutoJoin && c.Role() == Leader {
						if err := c.raftNode.AddClusterNode(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
							log.WithError(err).WithField("event", ev).Error("Error adding member")
						}
					}
					continue
				}
				if c.config.AutoJoin && c.Role() == Leader {
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
		log.WithField("state", newState.String()).Debug("State changed")
		go c.sendEvent(Event{LocalState: newState})
		log.WithField("state", newState.String()).Debug("Event emitted")
	} else {
		log.Debugf("Won't change state from %s to %s", currentState, newState)
	}
}

func (c *clusterfunkCluster) LocalState() NodeState {
	return NodeState(atomic.LoadInt32(c.localState))
}
