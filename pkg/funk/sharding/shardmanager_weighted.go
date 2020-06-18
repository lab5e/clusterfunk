package sharding

//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
import (
	"errors"
	"fmt"
	"sync"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk/sharding/shardpb"
	"google.golang.org/protobuf/proto"
)

type nodeData struct {
	NodeID       string
	TotalWeights int
	Shards       []Shard
	WorkerID     int
}

func newNodeData(nodeID string) *nodeData {
	return &nodeData{NodeID: nodeID, TotalWeights: 0, Shards: make([]Shard, 0)}
}

func (nd *nodeData) checkTotalWeight() {
	tot := 0
	for _, n := range nd.Shards {
		tot += n.Weight()
	}
}
func (nd *nodeData) AddShard(shard Shard) {
	shard.SetNodeID(nd.NodeID)
	nd.TotalWeights += shard.Weight()
	nd.Shards = append(nd.Shards, shard)
	nd.checkTotalWeight()
}

func (nd *nodeData) RemoveShard(preferredWeight int) Shard {
	for i, v := range nd.Shards {
		if v.Weight() <= preferredWeight {
			nd.Shards = append(nd.Shards[:i], nd.Shards[i+1:]...)
			nd.TotalWeights -= v.Weight()
			nd.checkTotalWeight()
			return v
		}
	}
	if len(nd.Shards) == 0 {
		panic("no shards remaining")
	}
	// This will cause a panic if there's no shards left. That's OK.
	// Future me might disagree.
	ret := nd.Shards[0]
	if len(nd.Shards) >= 1 {
		nd.Shards = nd.Shards[1:]
	}
	nd.TotalWeights -= ret.Weight()
	nd.checkTotalWeight()
	return ret
}

type weightedShardMap struct {
	shards          []Shard
	mutex           *sync.RWMutex
	totalWeight     int
	nodes           map[string]*nodeData
	maxWorkerID     int
	workerIDCounter int
}

// NewShardMap creates a new shard mapper instance.
func NewShardMap() ShardMap {
	return &weightedShardMap{
		shards:          make([]Shard, 0),
		mutex:           &sync.RWMutex{},
		totalWeight:     0,
		nodes:           make(map[string]*nodeData),
		workerIDCounter: 1,
		maxWorkerID:     16383, // TODO: User-supplied parameter later on
	}
}

func (sm *weightedShardMap) Init(maxShards int, weights []int) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if maxShards < 1 {
		return errors.New("maxShards must be > 0")
	}
	if len(sm.shards) != 0 {
		return errors.New("shards already set")
	}

	if weights != nil && len(weights) != maxShards {
		return errors.New("maxShards and len(weights) must be the same")
	}

	sm.shards = make([]Shard, maxShards)
	for i := range sm.shards {
		weight := 1
		if weights != nil {
			weight = weights[i]
		}
		sm.totalWeight += weight
		sm.shards[i] = NewShard(i, weight)
		if weight == 0 {
			return fmt.Errorf("can't use weight = 0 for shard %d", i)
		}
	}
	return nil
}

func (sm *weightedShardMap) UpdateNodes(nodeID ...string) {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	var newNodes []string
	var removedNodes []string

	// Find the new nodes
	for _, v := range nodeID {
		_, exists := sm.nodes[v]
		if !exists {
			newNodes = append(newNodes, v)
		}
	}
	for k := range sm.nodes {
		found := false
		for _, n := range nodeID {
			if n == k {
				// node exists, ignore it
				found = true
				break
			}
		}
		if !found {
			removedNodes = append(removedNodes, k)
		}
	}

	for _, v := range newNodes {
		sm.addNode(v)
	}
	for _, v := range removedNodes {
		sm.removeNode(v)
	}
}
func (sm *weightedShardMap) nextWorkerID() int {
	sm.workerIDCounter++
	return sm.workerIDCounter % sm.maxWorkerID
}

func (sm *weightedShardMap) addNode(nodeID string) {
	newNode := newNodeData(nodeID)
	newNode.WorkerID = sm.nextWorkerID()
	// Invariant: First node
	if len(sm.nodes) == 0 {
		for i := range sm.shards {
			newNode.AddShard(sm.shards[i])
		}
		sm.nodes[nodeID] = newNode
		return
	}

	//Invariant: Node # 2 or later
	targetCount := sm.totalWeight / (len(sm.nodes) + 1)

	for k, v := range sm.nodes {
		for v.TotalWeights > targetCount && v.TotalWeights > 0 {
			shardToMove := v.RemoveShard(targetCount - v.TotalWeights)
			newNode.AddShard(shardToMove)
		}
		sm.nodes[k] = v
	}
	sm.nodes[nodeID] = newNode
}

func (sm *weightedShardMap) removeNode(nodeID string) {
	nodeToRemove, exists := sm.nodes[nodeID]
	if !exists {
		panic(fmt.Sprintf("Unknown node ID: %s", nodeID))
	}
	delete(sm.nodes, nodeID)
	// Invariant: This is the last node in the cluster. No point in
	// generating transfers
	if len(sm.nodes) == 0 {
		for i := range sm.shards {
			sm.shards[i].SetNodeID("")
		}
		return
	}

	targetCount := sm.totalWeight / len(sm.nodes)
	for k, v := range sm.nodes {
		//		fmt.Printf("Removing node %s: Node %s w=%d target=%d\n", nodeID, k, v.TotalWeights, targetCount)
		for v.TotalWeights <= targetCount && nodeToRemove.TotalWeights > 0 {
			shardToMove := nodeToRemove.RemoveShard(targetCount - v.TotalWeights)
			v.AddShard(shardToMove)
		}
		sm.nodes[k] = v
	}
}

func (sm *weightedShardMap) MapToNode(shardID int) Shard {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	if shardID > len(sm.shards) || shardID < 0 {
		// This might be too extreme but useful for debugging.
		// another alternative is to return a catch-all node allowing
		// the proxying to fix it but if the shard ID is invalid it is
		// probably an error with the shard function itself and warrants
		// a panic() from the library.
		panic(fmt.Sprintf("shard ID is outside range [0-%d]: %d", len(sm.shards), shardID))
	}
	return sm.shards[shardID]
}

func (sm *weightedShardMap) Shards() []Shard {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	ret := make([]Shard, len(sm.shards))
	copy(ret, sm.shards)
	return ret
}

func (sm *weightedShardMap) TotalWeight() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.totalWeight
}

func (sm *weightedShardMap) ShardCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.shards)
}

func (sm *weightedShardMap) NodeList() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var ret []string
	for k := range sm.nodes {
		ret = append(ret, k)
	}
	return ret
}

func (sm *weightedShardMap) MarshalBinary() ([]byte, error) {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	if len(sm.nodes) == 0 {
		return nil, errors.New("map does not contain any nodes")
	}

	msg := &shardpb.ShardDistribution{}
	nodeMap := make(map[string]int32)
	n := int32(0)
	for _, v := range sm.nodes {
		msg.Nodes = append(msg.Nodes, &shardpb.WireNodes{
			NodeId:   n,
			NodeName: v.NodeID,
			WorkerId: int32(v.WorkerID),
		})
		nodeMap[v.NodeID] = n
		n++
	}

	for _, shard := range sm.shards {
		msg.Shards = append(msg.Shards, &shardpb.WireShard{
			Id:     int32(shard.ID()),
			Weight: int32(shard.Weight()),
			NodeId: nodeMap[shard.NodeID()],
		})
	}
	buf, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (sm *weightedShardMap) UnmarshalBinary(buf []byte) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	msg := &shardpb.ShardDistribution{}
	if err := proto.Unmarshal(buf, msg); err != nil {
		return err
	}

	sm.totalWeight = 0
	sm.nodes = make(map[string]*nodeData)
	sm.shards = make([]Shard, 0)

	nodeMap := make(map[int32]string)
	for _, v := range msg.Nodes {
		nodeMap[v.NodeId] = v.NodeName
		newNode := newNodeData(v.NodeName)
		newNode.WorkerID = int(v.WorkerId)
		sm.nodes[v.NodeName] = newNode
	}
	for _, v := range msg.Shards {
		newShard := NewShard(int(v.Id), int(v.Weight))
		sm.nodes[nodeMap[v.NodeId]].AddShard(newShard)
		sm.totalWeight += int(v.Weight)
		sm.shards = append(sm.shards, newShard)
	}
	return nil
}

func (sm *weightedShardMap) ShardCountForNode(nodeid string) int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	node, ok := sm.nodes[nodeid]
	if !ok {
		return 0
	}
	return len(node.Shards)
}

func (sm *weightedShardMap) WorkerID(nodeID string) int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	node, ok := sm.nodes[nodeID]
	if !ok {
		return -1
	}
	return node.WorkerID
}
