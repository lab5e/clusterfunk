package cluster

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/golang/protobuf/proto"
	"github.com/stalehd/clusterfunk/cluster/clusterproto"

	"github.com/stalehd/clusterfunk/cluster/sharding"
	"github.com/stalehd/clusterfunk/utils"
	"google.golang.org/grpc"
)

// This is the internal state protected by a single mutex
type internalState struct {
	mutex                *sync.RWMutex
	currentShardMapIndex uint64
	processedIndex       uint64 // The last processed index in the replicated log
	state                NodeState
	role                 NodeRole
	Unacknowledged       StringSet
}

func (i *internalState) logStateChange() {
	log.WithFields(log.Fields{
		"state": i.state.String(),
		"role":  i.role.String()}).Debug("State changed")
}
func (i *internalState) SetState(newState NodeState) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.state = newState
	i.logStateChange()
}

func (i *internalState) State() NodeState {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.state
}

func (i *internalState) Role() NodeRole {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.role
}

func (i *internalState) SetRole(newRole NodeRole) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.role = newRole
	i.logStateChange()
}

func (i *internalState) ProcessedIndex() uint64 {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.processedIndex
}

// SetProcessedIndex sets the last processed index. The returned value is
// the current value of the processed index.
func (i *internalState) SetProcessedIndex(index uint64) uint64 {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	if index > i.processedIndex {
		i.processedIndex = index
	}
	return i.processedIndex
}

func (i *internalState) CurrentShardMapIndex() uint64 {
	i.mutex.RLock()
	defer i.mutex.RUnlock()
	return i.currentShardMapIndex
}

func (i *internalState) SetCurrentShardMapIndex(index uint64) {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.currentShardMapIndex = index
}

// clusterfunkClusterß implements the Cluster interface
type clusterfunkCluster struct {
	serfNode      *SerfNode
	raftNode      *RaftNode
	config        Parameters
	registry      *utils.ZeroconfRegistry
	name          string
	mgmtServer    *grpc.Server // gRPC server for management
	leaderServer  *grpc.Server // gRPC server for leader
	eventChannels []chan Event
	mutex         *sync.RWMutex
	shardManager  sharding.ShardManager
	state         internalState
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters, shardManager sharding.ShardManager) Cluster {
	ret := &clusterfunkCluster{
		config:        params,
		name:          params.ClusterName,
		eventChannels: make([]chan Event, 0),
		mutex:         &sync.RWMutex{},
		shardManager:  shardManager,
		state: internalState{
			state:                Invalid,
			role:                 NonVoter,
			mutex:                &sync.RWMutex{},
			currentShardMapIndex: 0,
			Unacknowledged:       NewStringSet(),
		},
	}
	return ret
}

func (c *clusterfunkCluster) Role() NodeRole {
	return c.state.Role()
}

func (c *clusterfunkCluster) LocalState() NodeState {
	return c.state.State()
}
func (c *clusterfunkCluster) Start() error {
	c.config.Final()
	if c.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}

	c.state.SetState(Starting)
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
		switch e {
		case RaftClusterSizeChanged:
			timeCall(func() { c.handleClusterSizeChanged() }, "ClusterSizeChanged")

		case RaftLeaderLost:
			timeCall(func() { c.handleLeaderLost() }, "LeaderLost")

		case RaftBecameLeader:
			timeCall(func() { c.handleLeaderEvent() }, "BecameLeader")

		case RaftBecameFollower:
			timeCall(func() { c.handleFollowerEvent() }, "BecameFollower")

		case RaftReceivedLog:
			timeCall(func() { c.handleReceiveLog() }, "ReceivedLog")

		default:
			log.WithField("eventType", e).Error("Unknown event received")
		}
	}
}

func (c *clusterfunkCluster) handleLeaderEvent() {
	c.state.SetRole(Leader)
	// A clustersizechanged-event is emitted by the Raft library when
	// leader is announced. This will force a reshard
}

func (c *clusterfunkCluster) handleLeaderLost() {
	c.state.SetState(Voting)
	c.state.SetCurrentShardMapIndex(0)
}

func (c *clusterfunkCluster) handleFollowerEvent() {
	c.state.SetRole(Follower)
}

func (c *clusterfunkCluster) handleClusterSizeChanged() {
	if c.state.Role() != Leader {
		// I'm not the leader. Won't do anything.
		return
	}

	// Reset the list of acked nodes.
	list := c.raftNode.Nodes.List()
	c.state.Unacknowledged.Sync(list...)

	c.state.SetState(Resharding)
	// reshard cluster, distribute via replicated log.

	c.shardManager.UpdateNodes(list...)
	proposedShardMap, err := c.shardManager.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Can't marshal the shard map: %v", err))
	}

	mapMessage := NewLogMessage(ProposedShardMap, c.config.LeaderEndpoint, proposedShardMap)
	// Replicate proposed shard map via log
	buf, err := mapMessage.MarshalBinary()
	if err != nil {
		panic(fmt.Sprintf("Unable to marshal the log message containing shard map: %v", err))
	}
	// TODO: Clients might have acked at this point.
	index, err := c.raftNode.AppendLogEntry(buf)
	if err != nil {
		if c.raftNode.Leader() {
			panic("I'm the leader but I could not write the log")
		}
		// otherwise -- just log it and continue
		log.WithError(err).Error("Could not write log entry for new shard map")
	}
	c.state.SetCurrentShardMapIndex(index)
	log.WithFields(log.Fields{"index": index}).Debugf("Shard map index")

	// Next messages will be ackReceived when the changes has replicated
	// out to the other nodes.
	// No new state here - wait for a series of ackReceived states
	// from the nodes.
}

func (c *clusterfunkCluster) handleAckReceived(nodeID string, shardIndex uint64) bool {
	// when a new ack message is received the ack is noted for the node and
	// until all nodes have acked the state will stay the same.
	if !c.state.Unacknowledged.Remove(nodeID) {
		return false
	}
	// Timeouts are handled when calling the other nodes via gRPC

	if c.state.Unacknowledged.Size() == 0 {
		// ack is completed. Enable the new shard map for the cluster by
		// sending a commit log message. No further processing is required
		// here.
		c.sendCommitMessage(shardIndex)
		c.state.SetState(Operational)

	}
	return true

}
func (c *clusterfunkCluster) handleReceiveLog() {
	messages := c.raftNode.GetLogMessages(c.state.ProcessedIndex())
	for _, msg := range messages {
		c.state.SetProcessedIndex(msg.Index)
		switch msg.MessageType {
		case ProposedShardMap:
			if c.state.Role() == Leader {
				// Ack to myself - this skips the whole gRPC call
				c.handleAckReceived(c.raftNode.LocalNodeID(), c.state.CurrentShardMapIndex())
				continue
			}
			c.processProposedShardMap(&msg)

		case ShardMapCommitted:
			if c.state.Role() == Leader {
				// I'm the leader so the map is already committed
				continue
			}
			wantedIndex := c.state.CurrentShardMapIndex()
			if wantedIndex > 0 && msg.Index < wantedIndex {
				continue
			}
			// Update internal node list to reflect the list of nodes. Do a
			// quick sanity check on the node list to make sure this node
			// is a member (we really should)
			c.processShardMapCommitMessage(&msg)

		default:
			log.WithField("logType", msg.MessageType).Error("Unknown log type in replication log")
		}
	}
}

func (c *clusterfunkCluster) processProposedShardMap(msg *LogMessage) {
	// Check if this is the index we're looking for. If this isn't the index
	// we want or the index is bigger than the one we expected we can ignore it.
	wantedIndex := c.state.CurrentShardMapIndex()
	if wantedIndex > 0 && msg.Index < wantedIndex {
		return
	}

	// Unmarshal the shard map. If this doesn't work we're in trouble.
	if err := c.shardManager.UnmarshalBinary(msg.Data); err != nil {
		panic(fmt.Sprintf("Could not unmarshal shard map from log message: %v", err))
	}
	// If this is a map without the local node we can ignore it completely.
	found := false
	localNode := c.raftNode.LocalNodeID()
	for _, v := range c.shardManager.NodeList() {
		if v == localNode {
			found = true
			break
		}
	}
	if !found {
		return
	}

	// Ack it. If the leader doesn't recognize it we can ignore the response.
	c.ackShardMap(uint64(msg.Index), msg.AckEndpoint)
}

// processShardMapCommitMessage processes the commit message from the leader.
// if this matches the previously acked shard map index.
func (c *clusterfunkCluster) processShardMapCommitMessage(msg *LogMessage) {
	if c.raftNode.Leader() {
		return
	}
	commitMsg := &clusterproto.CommitShardMapMessage{}
	if err := proto.Unmarshal(msg.Data, commitMsg); err != nil {
		panic(fmt.Sprintf("Unable to unmarshal commit message: %v", err))
	}
	if c.state.CurrentShardMapIndex() == 0 {
		return
	}
	if uint64(commitMsg.ShardMapLogIndex) != c.state.CurrentShardMapIndex() {
		return
	}
	c.raftNode.Nodes.Sync(commitMsg.Nodes...)
	// Reset state here
	c.state.SetCurrentShardMapIndex(0)
	c.state.SetState(Operational)
}

func (c *clusterfunkCluster) ackShardMap(index uint64, endpoint string) {
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
	client := clusterproto.NewClusterLeaderServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	resp, err := client.ConfirmShardMap(ctx, &clusterproto.ConfirmShardMapRequest{
		NodeID:   c.raftNode.LocalNodeID(),
		LogIndex: int64(index),
	})
	if err != nil {
		return
	}
	// Set the currently acked and expected map index
	c.state.SetCurrentShardMapIndex(uint64(resp.CurrentIndex))

	if !resp.Success {
		if uint64(resp.CurrentIndex) == index {
			// It's only an error if the index is the same
			log.WithFields(log.Fields{
				"index":       index,
				"leaderIndex": resp.CurrentIndex,
			}).Error("Leader rejected ack")
		}
		return
	}
	c.state.SetState(Resharding)
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
					if c.config.AutoJoin && c.state.Role() == Leader {
						if err := c.raftNode.AddClusterNode(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
							log.WithError(err).WithField("event", ev).Error("Error adding member")
						}
					}
					continue
				}
				if c.config.AutoJoin && c.state.Role() == Leader {
					if err := c.raftNode.RemoveClusterNode(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
						log.WithError(err).WithField("event", ev).Error("Error removing member")
					}
				}
			}
		}(c.serfNode.Events())
	}
}

func (c *clusterfunkCluster) Stop() {
	c.state.SetState(Stopping)

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

	c.state.SetRole(Unknown)
	c.state.SetState(Invalid)
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
