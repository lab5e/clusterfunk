package funk

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk/managepb"
	"github.com/lab5e/clusterfunk/pkg/funk/metrics"
	"github.com/lab5e/gotoolbox/grpcutil"
	"github.com/lab5e/gotoolbox/netutils"
	"github.com/sirupsen/logrus"

	"github.com/lab5e/clusterfunk/pkg/funk/clusterpb"
	"google.golang.org/protobuf/proto"

	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"google.golang.org/grpc"
)

// clusterfunkClusterß implements the Cluster interface
type clusterfunkCluster struct {
	name                 string                    // Name of cluster
	shardManager         sharding.ShardMap         // Shard map
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
	cfmetrics            metrics.Sink
	leaderClientMutex    *sync.Mutex // Mutex for the gRPC client pointing to the leader node
	leaderClientConn     *grpc.ClientConn
	leaderClient         managepb.ClusterManagementClient
	created              *int64
	writeBootstrap       *int64
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters, shardManager sharding.ShardMap) (Cluster, error) {
	params.Final()
	if params.Name == "" {
		return nil, errors.New("cluster name not specified")
	}
	ret := &clusterfunkCluster{
		name:                 params.Name,
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
		livenessChecker:      NewLivenessChecker(params.LivenessRetries),
		leaderClientMutex:    &sync.Mutex{},
		created:              new(int64),
		writeBootstrap:       new(int64),
	}
	ret.cfmetrics = metrics.NewSinkFromString(params.Metrics, ret)
	atomic.StoreInt64(ret.created, time.Now().UnixNano())
	atomic.StoreInt64(ret.writeBootstrap, 0)
	return ret, nil
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

func (c *clusterfunkCluster) SetEndpoint(name, listenAddress string) {
	// Sanity check the endpoint. If it's the loopback adapter it might not be
	// what you want.
	if netutils.IsLoopbackAddress(listenAddress) {
		logrus.WithFields(logrus.Fields{
			"listenAddress": listenAddress,
			"name":          name,
		}).Warning("Registering endpoint with loopback adapter. The endpoint won't be reachable for anyone but local clients.")
	}
	c.serfNode.SetTag(name, listenAddress)
	if err := c.serfNode.PublishTags(); err != nil {
		logrus.WithError(err).Error("Error adding endpoint")
	}
}

func (c *clusterfunkCluster) GetEndpoint(nodeID string, endpointName string) string {
	return c.serfNode.Node(nodeID).Tags[endpointName]
}

func (c *clusterfunkCluster) NewObserver() EndpointObserver {
	return NewEndpointObserver(c.NodeID(), c.serfNode.Events(), c.serfNode.Endpoints())
}

func (c *clusterfunkCluster) Stop() {
	c.setState(Stopping)

	if c.raftNode.Leader() {
		if err := c.raftNode.StepDown(); err != nil {
			logrus.WithError(err).Warning("Unable to step down as leader before leaving")
		}
	}
	if err := c.raftNode.Stop(c.config.AutoJoin); err != nil {
		logrus.WithError(err).Warning("Error stopping Raft node. Will stop anyways")
	}
	if err := c.serfNode.Stop(); err != nil {
		logrus.WithError(err).Warning("Error stopping Serf node. Will stop anyways.")
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
	c.setState(Starting)

	// Launch node management endpoint
	if err := c.startManagementServices(); err != nil {
		logrus.WithError(err).Error("Unable to start management endpoint")
	}
	if err := c.startLeaderService(); err != nil {
		logrus.WithError(err).Error("Error starting leader gRPC interface")
	}
	if c.config.ZeroConf {
		// TOOD: Merge with service init code
		c.registry = toolbox.NewZeroconfRegistry(c.config.Name)

		var err error
		addrs, err := c.registry.Resolve(ZeroconfSerfKind, 1*time.Second)
		if err != nil {
			return err
		}

		if len(c.config.Serf.JoinAddress) == 0 {
			if len(addrs) > 0 {
				c.config.Serf.JoinAddress = addrs
			}
			if len(addrs) == 0 {
				logrus.Debug("No Serf nodes found, bootstrapping cluster")
				//c.config.Raft.Bootstrap = true
			}
		}
		if c.config.Raft.Bootstrap && len(addrs) > 0 {
			return errors.New("there's already a cluster with that name")
		}

		if err := c.registry.Register(ZeroconfSerfKind, c.config.NodeID, netutils.PortOfHostPort(c.config.Serf.Endpoint)); err != nil {
			return err
		}
		if err := c.registry.Register(ZeroconfManagementKind, c.config.NodeID, netutils.PortOfHostPort(c.config.Management.Endpoint)); err != nil {
			return err
		}

		// If we're using zeroconf + serf to automatically expand and shrink the cluster
		// leaders might linger in the Raft cluster even if they've shut down so
		// purge old nodes that have left the Serf cluster regularly
		go func() {
			logrus.Debug("Starting stale node check for ZeroConf cluster")
			for {
				time.Sleep(10 * time.Second)
				if !c.raftNode.Leader() {
					continue
				}
				raftList, err := c.raftNode.memberList()
				if err != nil {
					logrus.WithError(err).Warning("Unable to check member list in Raft cluster")
					continue
				}
				for _, rn := range raftList {
					sn := c.serfNode.Node(rn.ID)
					if sn.State != SerfAlive {
						// purge the node from the Raft cluster
						logrus.WithField("node", rn.ID).Info("Removing stale Raft node")
						if err := c.raftNode.RemoveClusterNode(rn.ID, sn.Tags[RaftEndpoint]); err != nil {
							logrus.WithError(err).Warning("Unable to remove stale Raft node")
						}
					}
				}
			}
		}()
	}
	if c.livenessClient == nil {
		c.livenessClient = NewLivenessClient(c.config.LivenessEndpoint)
		go c.livenessEventLoop()
	}
	go c.raftEventLoop(c.raftNode.Events())

	bootstrap, err := c.raftNode.Start(c.config.NodeID, c.config.Raft)
	if err != nil {
		return err
	}
	if bootstrap {
		t := time.Now()
		logrus.Infof("Bootstrapping cluster at %v. Will write bootstrap event when I'm the leader", t)
		atomic.StoreInt64(c.writeBootstrap, 1)
		// Write event when the node becomes the leader
	}
	// TODO: Start Serf node when Raft node has joined the cluster, not before
	// if the autojoin parameter is set to false. Raft nodes that haven't joined
	// will show up as members of the cluster (for the ctrlc tool) but they
	// aren't members until they're added to the cluster.
	c.serfNode.SetTag(RaftEndpoint, c.raftNode.Endpoint())
	c.serfNode.SetTag(SerfEndpoint, c.config.Serf.Endpoint)
	c.serfNode.SetTag(LivenessEndpoint, c.config.LivenessEndpoint)

	go c.serfEventLoop(c.serfNode.Events())

	return c.serfNode.Start(c.config.NodeID, c.name, c.config.Serf)
}

func (c *clusterfunkCluster) raftEventLoop(ch <-chan RaftEventType) {
	for e := range ch {
		switch e {
		case RaftClusterSizeChanged:
			c.clearLeaderManagementClient()
			toolbox.TimeCall(func() { c.handleClusterSizeChanged(c.raftNode.Nodes.List()) }, "ClusterSizeChanged")
			c.cfmetrics.SetClusterSize(c.raftNode.Nodes.Size())

		case RaftLeaderLost:
			c.livenessChecker.Clear()
			c.clearLeaderManagementClient()
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
			logrus.WithField("eventType", e).Error("Unknown event received")
		}
	}
}

func (c *clusterfunkCluster) handleLeaderEvent() {
	c.setRole(Leader)
	c.setCurrentShardMapIndex(0)
	// Write bootstrap log entry if this node has bootstrapped the cluster
	if atomic.SwapInt64(c.writeBootstrap, 0) == 1 {
		logrus.Debug("Writing bootstrap event log")
		m := NewBootstrapMessage(time.Now().UnixNano())
		buf, err := m.MarshalBinary()
		if err != nil {
			logrus.WithError(err).Warning("Could not marshal the bootstrap message for the replicated log")
		} else {
			if _, err := c.raftNode.AppendLogEntry(buf); err != nil {
				logrus.WithError(err).Warning("Could not write bootstrap message to the replicated log. Creation time might be incorrect.")
			}
		}
	}
	// When assuming leadership the node list built by events won't be up to date
	// with the list.  This forces an update of the node list.
	c.raftNode.RefreshNodes()

	// Ensure we've picked up all nodes in the cluster by checking the Serf list
	for _, node := range c.serfNode.Nodes() {
		if node.Tags[RaftEndpoint] != "" {
			// this is a Raft member
			if !c.raftNode.Nodes.Contains(node.NodeID) {
				logrus.WithField("nodeid", node.NodeID).Error("Raft configuration does not contain node")
			}
		}
	}
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
			logrus.WithField("nodes", unackNodes).Warning("Ack timeout for nodes but I'm no longer the leader")
			return
		}

		// TODO: rewrite a bit. There's a lot of shuffling around with types here.
		logrus.WithField("nodes", unackNodes).Warning("Ack timed out for one or more nodes. Repeating sharding.")
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
				logrus.Warning("No nodes have acked. Stepping down as leader.")
				if err := c.raftNode.StepDown(); err != nil {
					logrus.Warning("Unable to step down as leader")
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
	// reshard cluster, distribute via replicated logrus.

	c.shardManager.UpdateNodes(nodeList...)
	if len(nodeList) == 0 {
		logrus.Error("Cluster does not contain any nodes. Nothing to do when size changes")
		return
	}
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
		logrus.WithError(err).Error("Could not write log entry for new shard map")
	}
	// Reset the list of acked nodes and
	c.unacknowledged = newAckCollection()
	c.unacknowledged.StartAck(nodeList, index, c.config.AckTimeout)

	go c.checkAckStatus()
	c.cfmetrics.SetShardCount(c.shardManager.ShardCountForNode(c.raftNode.LocalNodeID()))
	c.cfmetrics.SetLogIndex(index)

	logrus.WithFields(logrus.Fields{"index": index}).Debugf("Shard map index")

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

		case Bootstrap:
			// Update bootstrap time
			atomic.StoreInt64(c.created, GetBootstrapTime(msg))
			logrus.WithFields(logrus.Fields{
				"index": msg.Index,
				"time":  atomic.LoadInt64(c.created),
			}).Info("Found bootstrap message")

		default:
			logrus.WithField("logType", msg.MessageType).Error("Unknown log type in replication log")
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

	c.cfmetrics.SetShardCount(c.shardManager.ShardCountForNode(c.raftNode.LocalNodeID()))
	c.cfmetrics.SetLogIndex(uint64(msg.Index))
}

// processShardMapCommitMessage processes the commit message from the leader.
// if this matches the previously acked shard map index.
func (c *clusterfunkCluster) processShardMapCommitMessage(msg *LogMessage) {
	if c.raftNode.Leader() {
		return
	}
	commitMsg := &clusterpb.CommitShardMapMessage{}
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
	clientParam := grpcutil.GRPCClientParam{
		ServerEndpoint: endpoint,
		TLS:            false,
		CAFile:         "",
	}
	opts, err := grpcutil.GetDialOpts(clientParam)
	if err != nil {
		//panic(fmt.Sprintf("Unable to acknowledge gRPC client parameters: %v", err))
		logrus.WithError(err).Error("Unable to acknowledge gRPC client parameters")
		return
	}
	conn, err := grpc.Dial(clientParam.ServerEndpoint, opts...)
	if err != nil {
		logrus.WithError(err).Error("Unable to dial server when acking shard map")
		return
	}
	defer conn.Close()
	client := clusterpb.NewClusterLeaderServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	resp, err := client.ConfirmShardMap(ctx, &clusterpb.ConfirmShardMapRequest{
		NodeId:   c.raftNode.LocalNodeID(),
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
			logrus.WithFields(logrus.Fields{
				"index":       index,
				"leaderIndex": resp.CurrentIndex,
			}).Error("Leader rejected ack")
		}
		return
	}
	c.setState(Resharding)
}

func (c *clusterfunkCluster) sendCommitMessage(index uint64) {
	commitMsg := clusterpb.CommitShardMapMessage{
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
		logrus.WithError(err).Error("Could not write log entry for new shard map")
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
						logrus.WithField("node", ev.Node.NodeID).Debug("Adding serf node")
						if err := c.raftNode.AddClusterNode(ev.Node.NodeID, ev.Node.Tags[RaftEndpoint]); err != nil {
							logrus.WithError(err).WithField("event", ev).Error("Error adding member")
						} else {
							c.livenessChecker.Add(ev.Node.NodeID, ev.Node.Tags[LivenessEndpoint])
						}
					}
				case SerfNodeLeft:
					known := knownNodes.Remove(ev.Node.NodeID)

					if c.config.AutoJoin && c.Role() == Leader {
						logrus.WithField("node", ev.Node.NodeID).Debug("Removing serf node")
						if err := c.raftNode.RemoveClusterNode(ev.Node.NodeID, ev.Node.Tags[RaftEndpoint]); err != nil {
							if known {
								logrus.WithError(err).WithField("event", ev).Error("Error removing member")
							}
						}
						c.livenessChecker.Remove(ev.Node.NodeID)
					}

				default:
					logrus.WithField("event", ev.Event).Warn("Unknown SerfNode event type")
				}
			}
		}(c.serfNode.Events())
	}
}

func (c *clusterfunkCluster) livenessEventLoop() {
	for {
		select {
		case id := <-c.livenessChecker.AliveEvents():
			logrus.WithField("id", id).Info("Node is presumed alive")
			// TODO(stalehd): Enable node here
			// c.raftNode.EnableNode(id)

		case id := <-c.livenessChecker.DeadEvents():
			logrus.WithField("id", id).Info("Node is presumed dead")
			// TODO(stalehd): Disable node here
			// c.raftNode.DisableNode(id)
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

func (c *clusterfunkCluster) Created() time.Time {
	return time.Unix(0, atomic.LoadInt64(c.created))
}
