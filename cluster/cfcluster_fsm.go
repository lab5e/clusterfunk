package cluster

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"

	"github.com/stalehd/clusterfunk/cluster/clusterproto"
	"github.com/stalehd/clusterfunk/cluster/fsmtool"

	"github.com/stalehd/clusterfunk/utils"
	"google.golang.org/grpc"
)

// Cluster mutations -- this is best expressed as a state machine. The state
// machine runs independently of the rest of the code. Mostly.

type internalFSMState int

// The FSM even is used to set the state with an optional node parameter
type fsmEvent struct {
	State  internalFSMState
	NodeID string // This field is set if there's an ack from a node.
}

type fsmFollowerEvent struct {
	State      internalFSMState // This is the new state to set
	LogMessage LogMessage       // This field is set when a log message is received.
}

// These are the internal states for both leaders and followers but they use separate FSMs.
const (
	initialClusterState internalFSMState = iota
	clusterSizeChanged                   // Leader state
	assumeLeadership                     // Leader state
	assumeFollower                       // Leader state
	ackReceived                          // Leader state
	ackCompleted                         // Leader state
	reshardCluster                       // Leader state
	newShardMapReceived                  // Follower state
	commitLogReceived                    // Follower state
	leaderLost                           // Leader and follower state
	leaderElected                        // Follower state
)

func (t internalFSMState) String() string {
	switch t {
	case initialClusterState:
		return "initialClusterState"
	case clusterSizeChanged:
		return "clusterSizeChanged"
	case assumeLeadership:
		return "assumeLeadership"
	case assumeFollower:
		return "assumeFollower"
	case ackReceived:
		return "ackReceived"
	case ackCompleted:
		return "ackCompleted"
	case reshardCluster:
		return "reshardCluster"
	case newShardMapReceived:
		return "newShardMapReceived"
	case commitLogReceived:
		return "commitLogReceived"
	case leaderLost:
		return "leaderLost"
	case leaderElected:
		return "leaderElected"
	default:
		panic(fmt.Sprintf("Unknown state: %d", t))
	}
}

// Sets the new state unless a different state is waiting.
func (c *clusterfunkCluster) setFSMState(newState internalFSMState, nodeID string) {
	select {
	case c.stateChannel <- fsmEvent{State: newState, NodeID: nodeID}:
	case <-time.After(1 * time.Second):
		log.Printf("Unable to set cluster FSM state (%d) after 1 second", newState)
		// channel is already full - skip
	}
}

func (c *clusterfunkCluster) setFollowerState(newState internalFSMState, msg LogMessage) {
	select {
	case c.followerStateChannel <- fsmFollowerEvent{State: newState, LogMessage: msg}:
	case <-time.After(1 * time.Second):
		log.Printf("********************* Follower FSM not reading from channel")
	}
}
func (c *clusterfunkCluster) followerStateMachine() {
	state := fsmtool.NewStateTransitionTable(initialClusterState)
	state.LogOnError = true
	state.LogTransitions = true
	state.PanicOnError = false
	state.Name = "Follower"
	state.AddTransitions(
		initialClusterState, leaderLost, // Leader is lost, wait for vote
		initialClusterState, leaderElected,

		leaderLost, leaderElected,
		leaderElected, leaderLost,

		leaderElected, newShardMapReceived,

		newShardMapReceived, commitLogReceived,
		newShardMapReceived, leaderLost,
		commitLogReceived, leaderLost,
		commitLogReceived, newShardMapReceived,
		commitLogReceived, commitLogReceived,

		// Investigate:
		newShardMapReceived, newShardMapReceived,
	)

	for newState := range c.followerStateChannel {
		state.Apply(newState.State, func(sst *fsmtool.StateTransitionTable) {
			switch newState.State {
			case initialClusterState:
				// no-op

			case leaderLost:
				// halt processing
				c.setLocalState(Voting)

			case leaderElected:
				// wait for shard map and commit message
				c.setLocalState(Resharding)

			case newShardMapReceived:
				// apply shard map
				if err := c.shardManager.UnmarshalBinary(newState.LogMessage.Data); err != nil {
					panic(fmt.Sprintf("Could not unmarshal shard map from log message: %v", err))
				}
				wantedIndex := atomic.LoadUint64(c.wantedShardLogIndex)
				if wantedIndex > 0 && newState.LogMessage.Index != wantedIndex {
					log.Printf("Ignoring shard map with index %d", newState.LogMessage.Index)
					break
				}
				atomic.StoreUint64(c.reshardingLogIndex, newState.LogMessage.Index)
				c.ackShardMap(newState.LogMessage.AckEndpoint)

			case commitLogReceived:

				// start serving
				wantedIndex := atomic.LoadUint64(c.wantedShardLogIndex)
				if wantedIndex > 0 && newState.LogMessage.Index < wantedIndex {
					log.Printf("Ignoring commit message with index %d", newState.LogMessage.Index)
					break
				}
				log.Printf("Shard map is comitted by leader. Start serving!")
				c.dumpShardMap()

				c.setLocalState(Operational)
			}
		})
	}
}

// clusterStateMachine is the FSM for
func (c *clusterfunkCluster) clusterStateMachine() {
	log.Printf("STATE: Launching")
	state := fsmtool.NewStateTransitionTable(initialClusterState)
	state.LogOnError = true
	state.LogTransitions = false
	state.PanicOnError = false
	state.Name = "Leader"

	state.AddTransitions(
		initialClusterState, assumeLeadership,
		initialClusterState, assumeFollower,

		assumeLeadership, reshardCluster,
		assumeLeadership, clusterSizeChanged,

		clusterSizeChanged, reshardCluster,

		reshardCluster, ackReceived,

		ackReceived, ackReceived,
		ackReceived, ackCompleted,
		ackReceived, clusterSizeChanged,

		// Sketchy transitions below.
		assumeFollower, assumeLeadership,
		assumeFollower, assumeFollower,
		ackCompleted, ackReceived,
		ackCompleted, clusterSizeChanged,
		ackCompleted, assumeFollower,
		reshardCluster, assumeFollower,
		reshardCluster, clusterSizeChanged,
		reshardCluster, reshardCluster,
	)

	var unacknowledgedNodes []string
	shardMapLogIndex := uint64(0)
	for newState := range c.stateChannel {
		state.Apply(newState.State, func(stt *fsmtool.StateTransitionTable) {
			switch stt.CurrentState.(internalFSMState) {
			case assumeLeadership:
				c.setRole(Leader)
				c.setFSMState(reshardCluster, "")

			case clusterSizeChanged:
				c.setFSMState(reshardCluster, "")
				c.setLocalState(Resharding)

			case reshardCluster:
				// reshard cluster, distribute via replicated log.

				// Reset the list of acked nodes.
				list := c.raftNode.Members()

				c.shardManager.UpdateNodes(list...)
				proposedShardMap, err := c.shardManager.MarshalBinary()
				if err != nil {
					panic(fmt.Sprintf("Can't marshal the shard map: %v", err))
				}
				mapMessage := NewLogMessage(ProposedShardMap, c.config.LeaderEndpoint, proposedShardMap)
				// Build list of unacked nodes
				// Note that this might include the local node as well, which
				// is OK. The client part will behave like all other parts.
				unacknowledgedNodes = append([]string{}, list...)

				// Replicate proposed shard map via log
				buf, err := mapMessage.MarshalBinary()
				if err != nil {
					panic(fmt.Sprintf("Unable to marshal the log message containing shard map: %v", err))
				}
				index, err := c.raftNode.AppendLogEntry(buf)
				if err != nil {
					// We might have lost the leadership here. Log and continue.
					if err := c.raftNode.ra.VerifyLeader().Error(); err == nil {
						panic("I'm the leader but I could not write the log")
					}
					// otherwise -- just log it and continue
					log.Printf("Could not write log entry for new shard map")
				}
				atomic.StoreUint64(c.reshardingLogIndex, index)
				// This is the index we want commits for.
				shardMapLogIndex = c.raftNode.LastIndex()
				log.Printf("Shard map index = %d", shardMapLogIndex)
				//c.updateNodes(c.shardManager.NodeList())
				// Next messages will be ackReceived when the changes has replicated
				// out to the other nodes.
				// No new state here - wait for a series of ackReceived states
				// from the nodes.

			case ackReceived:
				// when a new ack message is received the ack is noted for the node and
				// until all nodes have acked the state will stay the same.
				for i, v := range unacknowledgedNodes {
					if v == newState.NodeID {
						unacknowledgedNodes = append(unacknowledgedNodes[:i], unacknowledgedNodes[i+1:]...)
					}
				}
				log.Printf("STATE: ack received from %s, %d nodes remaining", newState.NodeID, len(unacknowledgedNodes))
				// Timeouts are handled when calling the other nodes via gRPC
				allNodesHaveAcked := false
				if len(unacknowledgedNodes) == 0 {
					allNodesHaveAcked = true
				}

				if allNodesHaveAcked {
					c.setFSMState(ackCompleted, "")
				}
				return // continue

			case ackCompleted:
				// TODO: Log final commit message, establishing the new state in the cluster
				// ack is completed. Enable the new shard map for the cluster by
				// sending a commit log message. No further processing is required
				// here.
				commitMessage := NewLogMessage(ShardMapCommitted, c.raftNode.LocalNodeID(), []byte{})
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
					log.Printf("Could not write log entry for new shard map: %v", err)
				}
				return // continue

			case assumeFollower:
				// Not much happens here but the next state should be - if all
				// goes well - a shard map log message from the leader.
				c.setRole(Follower)
			}
		})
	}
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
		log.Printf("Unable to acknowledge gRPC client parameters: %v", err)
		return
	}
	conn, err := grpc.Dial(clientParam.ServerEndpoint, opts...)
	if err != nil {
		//panic(fmt.Sprintf("Unable to dial server when acking shard map: %v", err))
		log.Printf("Unable to dial server when acking shard map: %v", err)
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
		//panic(fmt.Sprintf("Unable to confirm shard map: %v", err))
		log.Printf("Unable to confirm shard map: %v", err)
		return
	}
	if !resp.Success {
		log.Printf("Leader rejected ack. I got index %d and leader wants %d", logIndex, resp.CurrentIndex)
		atomic.StoreUint64(c.wantedShardLogIndex, uint64(resp.CurrentIndex))
		return
	}
	log.Printf("Shard map ack successfully sent to leader (leader=%s, index=%d)", endpoint, logIndex)
	atomic.StoreUint64(c.wantedShardLogIndex, 0)
}
