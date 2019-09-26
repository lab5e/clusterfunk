package cluster

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/stalehd/clusterfunk/cluster/sharding"
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
	localState    NodeState
	localRole     NodeRole
	mgmtServer    *grpc.Server
	eventChannels []chan Event
	mutex         *sync.RWMutex
	stateChannel  chan fsmEvent
	shardManager  sharding.ShardManager
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters, shardManager sharding.ShardManager) Cluster {

	ret := &clusterfunkCluster{
		config:        params,
		name:          params.ClusterName,
		localState:    Invalid,
		localRole:     Unknown,
		eventChannels: make([]chan Event, 0),
		mutex:         &sync.RWMutex{},
		stateChannel:  make(chan fsmEvent, 5),
		shardManager:  shardManager,
	}
	go ret.clusterStateMachine()
	return ret
}

func (c *clusterfunkCluster) Start() error {
	c.config.final()
	if c.config.ClusterName == "" {
		return errors.New("cluster name not specified")
	}

	c.setLocalState(Starting, true)
	c.serfNode = NewSerfNode()

	// Launch node management endpoint
	if err := c.startManagementServices(); err != nil {
		log.Printf("Error starting management endpoint: %v", err)
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
	// Set state to none here before it's updated by the
	c.serfNode.SetTag(NodeRaftState, StateNone)

	c.raftNode = NewRaftNode()

	go c.raftEvents(c.raftNode.Events())

	if err := c.raftNode.Start(c.config.NodeID, c.config.Verbose, c.config.Raft); err != nil {
		return err
	}

	c.serfNode.SetTag(NodeType, c.config.NodeType())
	c.serfNode.SetTag(RaftNodeID, c.config.NodeID)
	c.serfNode.SetTag(RaftEndpoint, c.raftNode.Endpoint())
	c.serfNode.SetTag(SerfEndpoint, c.config.Serf.Endpoint)

	go c.serfEvents(c.serfNode.Events())

	if err := c.serfNode.Start(c.config.NodeID, c.config.Verbose, c.config.Serf); err != nil {
		return err
	}

	return nil
}

func (c *clusterfunkCluster) raftEvents(ch <-chan RaftEvent) {
	for e := range ch {
		switch e.Type {
		case RaftNodeAdded:
			//log.Printf("EVENT: Node %s added", e.NodeID)
			c.setFSMState(clusterSizeChanged, e.NodeID)

		case RaftNodeRemoved:
			//log.Printf("EVENT: Node %s removed", e.NodeID)
			c.setFSMState(clusterSizeChanged, e.NodeID)

		case RaftLeaderLost:
			//log.Printf("EVENT: Leader lost")
			c.setFSMState(leaderLost, "")

		case RaftBecameLeader:
			//log.Printf("EVENT: Became leader")
			c.setFSMState(assumeLeadership, "")
			go func() {
				// I'm the leader. Start resharding
				c.serfNode.SetTag(NodeRaftState, StateLeader)
				c.serfNode.PublishTags()
			}()

		case RaftBecameFollower:
			// Wait for the leader to redistribute the shards since it's the new leader
			log.Printf("EVENT: Became follower")
			c.setFSMState(assumeFollower, "")
			go func() {
				c.serfNode.SetTag(NodeRaftState, StateFollower)
				c.serfNode.PublishTags()
			}()

		case RaftReceivedLog:
			log.Printf("EVENT: Log (idx=%d, type=%d) received", e.Index, e.LogType)
			c.processReplicatedLog(e.LogType, e.Index)

		default:
			log.Printf("Unknown event received: %+v", e)
		}
	}
}

func (c *clusterfunkCluster) Role() NodeRole {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.localRole
}

func (c *clusterfunkCluster) setRole(newRole NodeRole) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.localRole = newRole
}

func (c *clusterfunkCluster) processReplicatedLog(t byte, index uint64) {
	buf := c.raftNode.GetReplicatedLogMessage(t)
	if buf == nil {
		panic(fmt.Sprintf("Could not find a message with type %d", t))
	}
	dumpMap := func() {
		log.Println("================== shard map ======================")
		nodes := make(map[string]int)
		for _, v := range c.shardManager.Shards() {
			n := nodes[v.NodeID()]
			n++
			nodes[v.NodeID()] = n
		}
		for k, v := range nodes {
			log.Printf("%-20s: %d shards", k, v)
		}
		log.Println("===================================================")
	}
	switch LogMessageType(t) {
	case ProposedShardMap:
		if c.Role() == Leader {
			log.Printf("Already have an updated shard map")
			dumpMap()
			return
		}
		if err := c.shardManager.UnmarshalBinary(buf); err != nil {
			panic(fmt.Sprintf("Could not unmarshal shard map from log message: %v", err))
		}
		dumpMap()
		c.setFSMState(newShardMapReceived, c.raftNode.LocalNodeID())
	case ShardMapCommitted:
		log.Printf("Shard map is comitted by leader. Start serving!")
	default:
		log.Printf("Don't know how to process log type %d", t)
	}
}

func (c *clusterfunkCluster) serfEvents(ch <-chan NodeEvent) {
	if c.config.AutoJoin {
		go func(ch <-chan NodeEvent) {
			for ev := range ch {
				if ev.Update {
					// Update node list
					c.updateNode(ev.NodeID, ev.Tags)
					c.DumpNodes()
					continue
				}
				if ev.Joined {
					// add node to list
					c.addNode(ev.NodeID, ev.Tags)
					c.DumpNodes()
					if c.config.AutoJoin && c.raftNode.Leader() {
						if err := c.raftNode.AddMember(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
							log.Printf("Error adding member: %v - %+v", err, ev)
						}
					}
					continue
				}
				c.removeNode(ev.NodeID)
				c.DumpNodes()
				if c.config.AutoJoin && c.raftNode.Leader() {
					if err := c.raftNode.RemoveMember(ev.NodeID, ev.Tags[RaftEndpoint]); err != nil {
						log.Printf("Error removing member: %v - %+v", err, ev)
					}
				}
			}
		}(c.serfNode.Events())
	}
}

func (c *clusterfunkCluster) Stop() {
	c.setLocalState(Stopping, true)

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

	c.localRole = Unknown
	c.setLocalState(Invalid, false)
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

func (c *clusterfunkCluster) setLocalState(newState NodeState, lock bool) {
	if lock {
		c.mutex.Lock()
		defer c.mutex.Unlock()
	}
	if c.localState != newState {
		c.localState = newState
		go c.sendEvent(Event{LocalState: c.localState})
	}
}

func (c *clusterfunkCluster) LocalState() NodeState {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return c.localState
}

// Cluster mutations -- this is best expressed as a state machine. The state
// machine runs independently of the rest of the code. Mostly.

type internalFSMState int

// The FSM even is used to set the state with an optional node parameter
type fsmEvent struct {
	State  internalFSMState
	NodeID string
}

// These are the internal states. See implementation for a description. Most of these
// operations can be interrupted if there's a leader change midway in the process.
const (
	clusterSizeChanged internalFSMState = iota
	assumeLeadership
	assumeFollower
	ackReceived
	ackCompleted
	reshardCluster
	updateRaftNodeList
	newShardMapReceived
	commitLogReceived
	leaderLost
)

// Sets the new state unless a different state is waiting.
func (c *clusterfunkCluster) setFSMState(newState internalFSMState, nodeID string) {
	select {
	case c.stateChannel <- fsmEvent{State: newState, NodeID: nodeID}:
	default:
		panic("Unable to set cluster state")
		// channel is already full - skip
	}
}

// clusterStateMachine is the FSM for
func (c *clusterfunkCluster) clusterStateMachine() {
	log.Printf("STATE: Launching")
	var unacknowledgedNodes []string
	clusterNodes := newNodeCollection()

	shardMapLogIndex := uint64(0)
	for newState := range c.stateChannel {
		switch newState.State {

		case assumeLeadership:
			log.Printf("STATE: assume leader")
			c.setRole(Leader)
			// start with updating the node list
			c.setFSMState(updateRaftNodeList, "")

		case clusterSizeChanged:
			log.Printf("STATE: Cluster size changed")
			c.setLocalState(Resharding, true)
			c.setFSMState(updateRaftNodeList, "")

		case updateRaftNodeList:
			log.Printf("STATE: update node list")
			// list is updated. Start resharding.

			needReshard := clusterNodes.Sync(c.raftNode.Members()...)
			if needReshard {
				log.Printf("Have nodes %+v (I'm %s)", clusterNodes, c.raftNode.LocalNodeID())
				log.Printf("Need resharding")
				c.setFSMState(reshardCluster, "")
			}

		case reshardCluster:
			// reshard cluster, distribute via replicated log.

			// Reset the list of acked nodes.
			log.Printf("STATE: reshardCluster")
			// TODO: ShardManager needs a rewrite
			c.shardManager.UpdateNodes(clusterNodes.Nodes...)
			log.Printf("Nodes = %d, Shards = %d", len(clusterNodes.Nodes), len(c.shardManager.Shards()))
			proposedShardMap, err := c.shardManager.MarshalBinary()
			if err != nil {
				panic(fmt.Sprintf("Can't marshal the shard map: %v", err))
			}
			mapMessage := NewLogMessage(ProposedShardMap, proposedShardMap)
			// Build list of unacked nodes
			// Note that this might include the local node as well, which
			// is OK. The client part will behave like all other parts.
			unacknowledgedNodes = append([]string{}, clusterNodes.Nodes...)

			// TODO: use log message type

			// Replicate proposed shard map via log
			buf, err := mapMessage.MarshalBinary()
			if err != nil {
				panic(fmt.Sprintf("Unable to marshal the log message containing shard map: %v", err))
			}
			timeCall(func() {
				if err := c.raftNode.AppendLogEntry(buf); err != nil {
					// We might have lost the leadership here. Log and continue.
					if err := c.raftNode.ra.VerifyLeader().Error(); err == nil {
						panic("I'm the leader but I could not write the log")
					}
					// otherwise -- just log it and continue
					log.Printf("Could not write log entry for new shard map")
				}
			}, "Appending shard map log entry")
			// This is the index we want commits for.
			shardMapLogIndex = c.raftNode.LastIndex()
			log.Printf("Shard map index = %d", shardMapLogIndex)
			// Next messages will be ackReceived when the changes has replicated
			// out to the other nodes.
			// No new state here - wait for a series of ackReceived states
			// from the nodes.

		case ackReceived:
			log.Printf("STATE: ack received from %s", newState.NodeID)

			// when a new ack message is received the ack is noted for the node and
			// until all nodes have acked the state will stay the same.
			for i, v := range unacknowledgedNodes {
				if v == newState.NodeID {
					unacknowledgedNodes = append(unacknowledgedNodes[:i], unacknowledgedNodes[i+1:]...)
				}
			}
			// Timeouts are handled when calling the other nodes via gRPC
			allNodesHaveAcked := false
			if len(unacknowledgedNodes) == 0 {
				allNodesHaveAcked = true
			}

			if allNodesHaveAcked {
				c.setFSMState(ackCompleted, "")
			}
			continue

		case ackCompleted:
			// TODO: Log final commit message, establishing the new state in the cluster
			log.Printf("STATE: ack complete. operational leader.")
			// ack is completed. Enable the new shard map for the cluster by
			// sending a commit log message. No further processing is required
			// here.
			commitMessage := NewLogMessage(ShardMapCommitted, []byte{})
			buf, err := commitMessage.MarshalBinary()
			if err != nil {
				panic(fmt.Sprintf("Unable to marshal commit message: %v", err))
			}
			if err := c.raftNode.AppendLogEntry(buf); err != nil {
				// We might have lost the leadership here. Panic if we're still
				// the leader
				if err := c.raftNode.ra.VerifyLeader().Error(); err == nil {
					panic("I'm the leader but I could not write the log")
				}
				// otherwise -- just log it and continue
				log.Printf("Could not write log entry for new shard map: %v", err)
			}
			c.setLocalState(Operational, true)
			continue

		case assumeFollower:
			log.Printf("STATE: follower")
			c.setRole(Follower)
			c.setLocalState(Resharding, true)
			// Not much happens here but the next state should be - if all
			// goes well - a shard map log message from the leader.

		case newShardMapReceived:
			log.Printf("STATE: shardmap received")
			// update internal map and ack map via gRPC (yes, even if I'm the
			// leader)
			//endpoint := c.LeaderNode().GetEndpoint(LeaderEndpoint)
			c.setLocalState(Resharding, true)

		case commitLogReceived:
			log.Printf("STATE: commit log received. Operational.")
			// commit log received, set state operational and resume normal
			// operations. Signal to the rest of the library (channel)
			c.setLocalState(Operational, true)

		case leaderLost:
			log.Printf("STATE: leader lost")
			c.setLocalState(Voting, true)
			// leader is lost - stop processing until a leader is elected and
			// the commit log is received
		}
	}
}

// -----------------------------------------------------------------------------
// Node operations -- initiated by Serf events

// updateNode updates the node with new tags
func (c *clusterfunkCluster) updateNode(nodeID string, tags map[string]string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	log.Printf("CLUSTER Update node %s", nodeID)
}

// addNode adds a new node
func (c *clusterfunkCluster) addNode(nodeID string, tags map[string]string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	log.Printf("CLUSTER Add node %s", nodeID)
}

// removeNode removes the node
func (c *clusterfunkCluster) removeNode(nodeID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	log.Printf("CLUSTER Remove node %s", nodeID)
}

func (c *clusterfunkCluster) DumpNodes() {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
}
