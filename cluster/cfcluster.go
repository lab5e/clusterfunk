package cluster

import (
	"errors"
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
	mgmtServer    *grpc.Server
	nodeState     NodeState
	clusterState  State
	eventChannels []chan Event
	mutex         *sync.RWMutex
	stateChannel  chan fsmEvent
}

// NewCluster returns a new cluster (client)
func NewCluster(params Parameters) Cluster {

	ret := &clusterfunkCluster{
		config:        params,
		name:          params.ClusterName,
		nodeState:     Initializing,
		clusterState:  Startup,
		eventChannels: make([]chan Event, 0),
		mutex:         &sync.RWMutex{},
		stateChannel:  make(chan fsmEvent, 5),
	}
	go ret.clusterStateMachine()
	return ret
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
	// Populate local node list with list from Serf

	/*	go func() {
		for {
			time.Sleep(5 * time.Second)
			// note: needs a mutex when raft cluster shuts down. If a panic
			// is raised when everything is going down the cluster will be
			// up s**t creek
			if c.raftNode.Leader() {
				timeCall(func() {
					if err := c.raftNode.AppendLogEntry(make([]byte, 89999)); err != nil {
						log.Printf("Error writing log entry: %v", err)
					}
				}, "apply log")
			}
		}
	}()*/

	c.nodeState = Empty
	return nil
}

func (c *clusterfunkCluster) raftEvents(ch <-chan RaftEvent) {
	for e := range ch {
		switch e.Type {
		case RaftNodeAdded:
			log.Printf("EVENT: Node %s added", e.NodeID)
			c.setFSMState(clusterSizeChanged, e.NodeID)

		case RaftNodeRemoved:
			log.Printf("EVENT: Node %s removed", e.NodeID)
			c.setFSMState(clusterSizeChanged, e.NodeID)

		case RaftLeaderLost:
			log.Printf("EVENT: Leader lost")
			c.setFSMState(leaderLost, "")

		case RaftBecameLeader:
			log.Printf("EVENT: Became leader")
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
		}
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
	if c.raftNode != nil {
		c.raftNode.Stop()
	}
	if c.serfNode != nil {
		c.serfNode.Stop()
	}
}

func (c *clusterfunkCluster) Name() string {
	return c.name
}

func (c *clusterfunkCluster) Nodes() []Node {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	return nil
}

func (c *clusterfunkCluster) LocalNode() Node {
	return nil
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
	ret := make(chan Event)
	c.eventChannels = append(c.eventChannels, ret)
	return ret
}

// State functions for cluster.

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
	shardManager := sharding.NewShardManager()
	var proposedShardMap []byte
	var currentShardMap []byte
	var err error

	for newState := range c.stateChannel {
		switch newState.State {

		case assumeLeadership:
			log.Printf("STATE: assume leader")
			// start with updating the node list
			c.setFSMState(updateRaftNodeList, "")

		case clusterSizeChanged:
			log.Printf("STATE: Cluster size changed")
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
			shardManager.UpdateNodes(clusterNodes.Nodes...)

			proposedShardMap, err = shardManager.MarshalBinary()
			if err != nil {
				panic("Can't marshal the shard map")
			}
			// Build list of unacked nodes
			// Note that this might include the local node as well, which
			// is OK. The client part will behave like all other parts.
			unacknowledgedNodes = append([]string{}, clusterNodes.Nodes...)

			// TODO: use log message type

			// Replicate proposed shard map via log
			if err := c.raftNode.AppendLogEntry(proposedShardMap); err != nil {
				// We might have lost the leadership here. Log and continue.
				if err := c.raftNode.ra.VerifyLeader().Error(); err == nil {
					panic("I'm the leader but I could not write the log")
				}
				// otherwise -- just log it and continue
				log.Printf("Could not write log entry for new shard map")
			}

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

			// Confirm the shard map by writing a commit message in the log
			currentShardMap = proposedShardMap
			if err := c.raftNode.AppendLogEntry(currentShardMap); err != nil {
				// We might have lost the leadership here. Panic if we're still
				// the leader
				if err := c.raftNode.ra.VerifyLeader().Error(); err == nil {
					panic("I'm the leader but I could not write the log")
				}
				// otherwise -- just log it and continue
				log.Printf("Could not write log entry for new shard map")
			}
			// ack is completed. Enable the new shard map for the cluster by
			// sending a commit log message. No further processing is required
			// here.
			continue

		case assumeFollower:
			log.Printf("STATE: follower")
			// Not much happens here but the next state should be - if all
			// goes well - a shard map log message from the leader.

		case newShardMapReceived:
			log.Printf("STATE: shardmap received")
			// update internal map and ack map via gRPC (yes, even if I'm the
			// leader)

		case commitLogReceived:
			log.Printf("STATE: commit log received. Operational.")
			// commit log received, set state operational and resume normal
			// operations. Signal to the rest of the library (channel)

		case leaderLost:
			log.Printf("STATE: leader lost")
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

}

// addNode adds a new node
func (c *clusterfunkCluster) addNode(nodeID string, tags map[string]string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

}

// removeNode removes the node
func (c *clusterfunkCluster) removeNode(nodeID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

}

func (c *clusterfunkCluster) DumpNodes() {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
}
