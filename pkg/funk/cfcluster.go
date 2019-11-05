package funk

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/golang/protobuf/proto"
	"github.com/stalehd/clusterfunk/pkg/funk/clusterproto"

	"github.com/stalehd/clusterfunk/pkg/funk/sharding"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
	"google.golang.org/grpc"
)

// clusterfunkClusterß implements the Cluster interface
type clusterfunkCluster struct {
	name                 string                    // Name of cluster
	shardManager         sharding.ShardManager     // Shard map
	config               Parameters                // Configuration
	serfNode             *SerfNode                 // Serf
	raftNode             *RaftNode                 // Raft
	eventChannels        []chan Event              // Event channels for subscribers
	mgmtServer           *grpc.Server              // gRPC server for management
	leaderServer         *grpc.Server              // gRPC server for leader
	mutex                *sync.RWMutex             // Mutex for events
	stateMutex           *sync.RWMutex             // Mutex for state and role
	currentShardMapIndex uint64                    // The current acked, wanted or sent shard map index
	processedIndex       uint64                    // The last processed index in the replicated log
	state                NodeState                 // Current cluster state for this node. Set based on events from Raft
	role                 NodeRole                  // Current cluster role for this node. Set based on events from Raft-
	unacknowledged       ackCollection             // The list of unacknowledged nodes. Only used by the leader.
	registry             *toolbox.ZeroconfRegistry // Zeroconf lookups. Used when joining
	livenessChecker      LivenessChecker           // Check for node liveness. The built-in heartbeats in Raft isn't exposed
	livenessClient       LocalLivenessEndpoint
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters, shardManager sharding.ShardManager) Cluster {
	ret := &clusterfunkCluster{
		name:                 params.ClusterName,
		shardManager:         shardManager,
		config:               params,
		serfNode:             NewSerfNode(),
		raftNode:             NewRaftNode(),
		eventChannels:        make([]chan Event, 0),
		mutex:                &sync.RWMutex{},
		state:                Invalid,
		role:                 NonVoter,
		stateMutex:           &sync.RWMutex{},
		currentShardMapIndex: 0,
		unacknowledged:       newAckCollection(),
		livenessChecker:      NewLivenessChecker(params.LivenessInterval, params.LivenessRetries),
	}
	return ret
}

func (c *clusterfunkCluster) Nodes() []string {
	return c.raftNode.Nodes.List()
}

func (c *clusterfunkCluster) Leader() string {
	return c.raftNode.LeaderNodeID()
}

func (c *clusterfunkCluster) NodeID() string {
	return c.config.NodeID
}

func (c *clusterfunkCluster) SetEndpoint(name, endpoint string) {
	c.serfNode.SetTag(name, endpoint)
	if err := c.serfNode.PublishTags(); err != nil {
		log.WithError(err).Error("Error adding endpoint")
	}
}

func (c *clusterfunkCluster) GetEndpoint(nodeID string, endpointName string) string {
	return c.serfNode.Node(nodeID).Tags[endpointName]
}

func (c *clusterfunkCluster) Stop() {
	c.setState(Stopping)

	if err := c.raftNode.Stop(); err != nil {
		log.WithError(err).Warning("Error stopping Raft node. Will stop anyways")
	}
	if err := c.serfNode.Stop(); err != nil {
		log.WithError(err).Warning("Error stopping Serf node. Will stop anyways.")
	}

	c.setRole(Unknown)
	c.setState(Invalid)

	// TODO: stop gRPC services

	for _, v := range c.eventChannels {
		close(v)
	}
}

func (c *clusterfunkCluster) Name() string {
	return c.name
}

func (c *clusterfunkCluster) Events() <-chan Event {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	ret := make(chan Event)
	c.eventChannels = append(c.eventChannels, ret)
	return ret
}

func (c *clusterfunkCluster) Start() error {
	c.config.Final()
	if c.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}

	c.setState(Starting)

	// Launch node management endpoint
	if err := c.startManagementServices(); err != nil {
		log.WithError(err).Error("Unable to start management endpoint")
	}
	if err := c.startLeaderService(); err != nil {
		log.WithError(err).Error("Error starting leader gRPC interface")
	}
	if c.config.ZeroConf {
		c.registry = toolbox.NewZeroconfRegistry(c.config.ClusterName)

		var err error
		addrs, err := c.registry.Resolve(ZeroconfSerfKind, 1*time.Second)
		if err != nil {
			return err
		}

		if c.config.Serf.JoinAddress == "" {
			if len(addrs) > 0 {
				c.config.Serf.JoinAddress = addrs[0]
			}
			if len(addrs) == 0 {
				log.Debug("No Serf nodes found, bootstrapping cluster")
				c.config.Raft.Bootstrap = true
			}
		}
		if c.config.Raft.Bootstrap && len(addrs) > 0 {
			return errors.New("there's already a cluster with that name")
		}

		if err := c.registry.Register(ZeroconfSerfKind, c.config.NodeID, toolbox.PortOfHostPort(c.config.Serf.Endpoint)); err != nil {
			return err
		}
		if err := c.registry.Register(ZeroconfManagementKind, c.config.NodeID, toolbox.PortOfHostPort(c.config.Management.Endpoint)); err != nil {
			return err
		}
	}
	if c.livenessClient == nil {
		c.livenessClient = NewLivenessClient(c.config.LivenessEndpoint)
		go c.livenessEventLoop()
	}
	go c.raftEventLoop(c.raftNode.Events())

	if err := c.raftNode.Start(c.config.NodeID, c.config.Raft); err != nil {
		return err
	}

	c.serfNode.SetTag(RaftEndpoint, c.raftNode.Endpoint())
	c.serfNode.SetTag(SerfEndpoint, c.config.Serf.Endpoint)
	c.serfNode.SetTag(LivenessEndpoint, c.config.LivenessEndpoint)

	go c.serfEventLoop(c.serfNode.Events())

	if err := c.serfNode.Start(c.config.NodeID, c.config.Serf); err != nil {
		return err
	}

	return nil
}

func (c *clusterfunkCluster) raftEventLoop(ch <-chan RaftEventType) {
	for e := range ch {
		switch e {
		case RaftClusterSizeChanged:
			toolbox.TimeCall(func() { c.handleClusterSizeChanged(c.raftNode.Nodes.List()) }, "ClusterSizeChanged")

		case RaftLeaderLost:
			c.livenessChecker.Clear()
			toolbox.TimeCall(func() { c.handleLeaderLost() }, "LeaderLost")

		case RaftBecameLeader:
			c.updateLivenessChecks()
			toolbox.TimeCall(func() { c.handleLeaderEvent() }, "BecameLeader")

		case RaftBecameFollower:
			c.livenessChecker.Clear()
			toolbox.TimeCall(func() { c.handleFollowerEvent() }, "BecameFollower")

		case RaftReceivedLog:
			toolbox.TimeCall(func() { c.handleReceiveLog() }, "ReceivedLog")

		default:
			log.WithField("eventType", e).Error("Unknown event received")
		}
	}
}

func (c *clusterfunkCluster) handleLeaderEvent() {
	c.setRole(Leader)
	c.setCurrentShardMapIndex(0)
	// When assuming leadership the node list built by events won't be up to date
	// with the list.  This forces an update of the node list.
	c.raftNode.RefreshNodes()
}

func (c *clusterfunkCluster) handleLeaderLost() {
	c.setState(Voting)
	c.setCurrentShardMapIndex(0)
}

func (c *clusterfunkCluster) handleFollowerEvent() {
	c.setRole(Follower)
	c.setCurrentShardMapIndex(0)
}

func (c *clusterfunkCluster) checkAckStatus() {
	select {
	case <-c.unacknowledged.Completed():
		// ack is completed. Enable the new shard map for the cluster by
		// sending a commit log message. No further processing is required
		// here.
		c.sendCommitMessage(c.unacknowledged.ShardIndex())
		c.setState(Operational)
		c.unacknowledged.Done()

	case unackNodes := <-c.unacknowledged.MissingAck():
		c.unacknowledged.Done()

		if !c.raftNode.Leader() {
			log.WithField("nodes", unackNodes).Warning("Ack timeout for nodes but I'm no longer the leader")
			return
		}

		// TODO: rewrite a bit. There's a lot of shuffling around with types here.
		log.WithField("nodes", unackNodes).Warning("Ack timed out for one or more nodes. Repeating sharding.")
		if c.raftNode.Leader() {
			// remove nodes from list, start new size change
			list := c.raftNode.Nodes.List()
			box := toolbox.NewStringSet()
			box.Sync(list...)
			for _, v := range unackNodes {
				box.Remove(v)
			}

			if box.Size() == 0 {
				// No nodes have responded to me. Step down as leader.
				log.Warning("No nodes have acked. Stepping down as leader.")
				if err := c.raftNode.StepDown(); err != nil {
					log.Warning("Unable to step down as leader")
				}
				return
			}
			c.handleClusterSizeChanged(box.List())
		}
	}
}
func (c *clusterfunkCluster) handleClusterSizeChanged(nodeList []string) {
	if c.Role() != Leader {
		// I'm not the leader. Won't do anything.
		return
	}

	c.setState(Resharding)
	// reshard cluster, distribute via replicated log.

	c.shardManager.UpdateNodes(nodeList...)
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
	index, err := c.raftNode.AppendLogEntry(buf)
	if err != nil {
		if c.raftNode.Leader() {
			panic("I'm the leader but I could not write the log")
		}
		// otherwise -- just log it and continue
		log.WithError(err).Error("Could not write log entry for new shard map")
	}
	// Reset the list of acked nodes and
	c.unacknowledged = newAckCollection()
	c.unacknowledged.StartAck(nodeList, index, c.config.AckTimeout)

	go c.checkAckStatus()

	log.WithFields(log.Fields{"index": index}).Debugf("Shard map index")

	// Next messages will be ackReceived when the changes has replicated
	// out to the other nodes.
	// No new state here - wait for a series of ackReceived states
	// from the nodes.
}

func (c *clusterfunkCluster) handleAckReceived(nodeID string, shardIndex uint64) bool {
	// when a new ack message is received the ack is noted for the node and
	// until all nodes have acked the state will stay the same.
	return c.unacknowledged.Ack(nodeID, shardIndex)
}

func (c *clusterfunkCluster) handleReceiveLog() {
	messages := c.raftNode.GetLogMessages(c.ProcessedIndex())
	for _, msg := range messages {
		c.setProcessedIndex(msg.Index)
		switch msg.MessageType {
		case ProposedShardMap:
			if c.Role() == Leader {
				// Ack to myself - this skips the whole gRPC call
				c.handleAckReceived(c.raftNode.LocalNodeID(), c.unacknowledged.ShardIndex())
				continue
			}
			c.processProposedShardMap(&msg)

		case ShardMapCommitted:
			if c.Role() == Leader {
				// I'm the leader so the map is already committed
				continue
			}
			wantedIndex := c.CurrentShardMapIndex()
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
	wantedIndex := c.CurrentShardMapIndex()
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
	if c.CurrentShardMapIndex() == 0 {
		return
	}
	if uint64(commitMsg.ShardMapLogIndex) != c.CurrentShardMapIndex() {
		return
	}
	c.raftNode.Nodes.Sync(commitMsg.Nodes...)
	// Reset state here
	c.setCurrentShardMapIndex(0)
	c.setState(Operational)
}

func (c *clusterfunkCluster) ackShardMap(index uint64, endpoint string) {
	// Step 1 Leader ID
	// Confirm the shard map
	clientParam := toolbox.GRPCClientParam{
		ServerEndpoint: endpoint,
		TLS:            false,
		CAFile:         "",
	}
	opts, err := toolbox.GetGRPCDialOpts(clientParam)
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
	c.setCurrentShardMapIndex(uint64(resp.CurrentIndex))

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
	c.setState(Resharding)
}

func (c *clusterfunkCluster) sendCommitMessage(index uint64) {
	commitMsg := clusterproto.CommitShardMapMessage{
		ShardMapLogIndex: int64(index),
	}
	commitMsg.Nodes = append(commitMsg.Nodes, c.raftNode.Nodes.List()...)
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
	knownNodes := toolbox.NewStringSet()
	if c.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			for ev := range ch {
				if ev.Node.Tags[RaftEndpoint] == "" {
					// ignore this node
					continue
				}
				switch ev.Event {
				case SerfNodeJoined, SerfNodeUpdated:
					if knownNodes.Add(ev.Node.NodeID) && c.config.AutoJoin && c.Role() == Leader {
						log.Warnf("Adding serf node %s to cluster", ev.Node.NodeID)
						if err := c.raftNode.AddClusterNode(ev.Node.NodeID, ev.Node.Tags[RaftEndpoint]); err != nil {
							log.WithError(err).WithField("event", ev).Error("Error adding member")
						} else {
							c.livenessChecker.Add(ev.Node.NodeID, ev.Node.Tags[LivenessEndpoint])
						}
					}
				case SerfNodeLeft:
					if knownNodes.Remove(ev.Node.NodeID) && c.config.AutoJoin && c.Role() == Leader {
						log.Warnf("Removing serf node %s from cluster", ev.Node.NodeID)
						if err := c.raftNode.RemoveClusterNode(ev.Node.NodeID, ev.Node.Tags[RaftEndpoint]); err != nil {
							log.WithError(err).WithField("event", ev).Error("Error removing member")
						}
						c.livenessChecker.Remove(ev.Node.NodeID)
					}

				default:
					log.WithField("event", ev.Event).Warn("Unknown SerfNode event type")
				}
			}
		}(c.serfNode.Events())
	}
}

func (c *clusterfunkCluster) livenessEventLoop() {
	for {
		select {
		case id := <-c.livenessChecker.AliveEvents():
			// if node is a member of the cluster change the cluster size
			c.raftNode.EnableNode(id)

		case id := <-c.livenessChecker.DeadEvents():
			// if node is a member of the cluster change the cluster size
			c.raftNode.DisableNode(id)
		}
	}
}

func (c *clusterfunkCluster) updateLivenessChecks() {
	for _, v := range c.raftNode.Nodes.List() {
		ep := c.serfNode.Node(v).Tags[LivenessEndpoint]
		if ep != "" {
			c.livenessChecker.Add(v, ep)
		}
	}
}
