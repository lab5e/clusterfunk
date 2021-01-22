package sharding

import (
	"errors"
	"fmt"
	"math/rand"
	"sync"

	"github.com/lab5e/clusterfunk/pkg/funk/sharding/shardpb"
	"google.golang.org/protobuf/proto"
)

type nodeData struct {
	NodeID   string
	Shards   []Shard
	WorkerID int
}

func newNodeData(nodeID string) *nodeData {
	return &nodeData{NodeID: nodeID, Shards: make([]Shard, 0)}
}

func (nd *nodeData) AddShard(shard Shard) {
	shard.SetNodeID(nd.NodeID)
	nd.Shards = append(nd.Shards, shard)
}

func (nd *nodeData) RemoveShard() Shard {
	// Remove a random shard from the list. Rather than removing shards
	// sequentially a random pick ensures that sequential keys will be
	// distributed roughly evenly even when theres more shards than keys.
	if len(nd.Shards) < 1 {
		panic("No more shards to remove")
	}
	index := rand.Intn(len(nd.Shards))
	shard := nd.Shards[index]
	nd.Shards = append(nd.Shards[:index], nd.Shards[index+1:]...)
	return shard
}

// A simple shard map implementation that
type simpleShardMap struct {
	shards          []Shard
	mutex           *sync.RWMutex
	nodes           map[string]*nodeData
	maxWorkerID     int
	workerIDCounter int
}

// NewShardMap creates a new shard mapper instance.
func NewShardMap() ShardMap {
	return &simpleShardMap{
		shards:          make([]Shard, 0),
		mutex:           &sync.RWMutex{},
		nodes:           make(map[string]*nodeData),
		workerIDCounter: 1,
		maxWorkerID:     16383, // TODO: User-supplied parameter later on
	}
}

func (sm *simpleShardMap) Init(maxShards int) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()
	if maxShards < 1 {
		return errors.New("maxShards must be > 0")
	}
	if len(sm.shards) != 0 {
		return errors.New("shards already set")
	}

	sm.shards = make([]Shard, maxShards)
	for i := range sm.shards {
		sm.shards[i] = NewShard(i)
	}
	return nil
}

func (sm *simpleShardMap) UpdateNodes(nodeID ...string) {
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
func (sm *simpleShardMap) nextWorkerID() int {
	sm.workerIDCounter++
	return sm.workerIDCounter % sm.maxWorkerID
}

func (sm *simpleShardMap) addNode(nodeID string) {
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
	targetCount := len(sm.shards) / (len(sm.nodes) + 1)

	for k, v := range sm.nodes {
		for len(v.Shards) > targetCount {
			shardToMove := v.RemoveShard()
			newNode.AddShard(shardToMove)
		}
		sm.nodes[k] = v
	}
	sm.nodes[nodeID] = newNode
}

func (sm *simpleShardMap) removeNode(nodeID string) {
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

	targetCount := len(sm.shards) / len(sm.nodes)
	for k, v := range sm.nodes {
		for len(v.Shards) < targetCount {
			shardToMove := nodeToRemove.RemoveShard()
			v.AddShard(shardToMove)
		}
		sm.nodes[k] = v
	}
	for len(nodeToRemove.Shards) > 0 {
		shardToMove := nodeToRemove.RemoveShard()
		for k := range sm.nodes {
			sm.nodes[k].AddShard(shardToMove)
			break
		}
	}
}

func (sm *simpleShardMap) MapToNode(shardID int) Shard {
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

func (sm *simpleShardMap) Shards() []Shard {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	ret := make([]Shard, len(sm.shards))
	copy(ret, sm.shards)
	return ret
}

func (sm *simpleShardMap) ShardCount() int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return len(sm.shards)
}

func (sm *simpleShardMap) NodeList() []string {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	var ret []string
	for k := range sm.nodes {
		ret = append(ret, k)
	}
	return ret
}

func (sm *simpleShardMap) MarshalBinary() ([]byte, error) {
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
			NodeId: nodeMap[shard.NodeID()],
		})
	}
	buf, err := proto.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return buf, nil
}

func (sm *simpleShardMap) UnmarshalBinary(buf []byte) error {
	sm.mutex.Lock()
	defer sm.mutex.Unlock()

	msg := &shardpb.ShardDistribution{}
	if err := proto.Unmarshal(buf, msg); err != nil {
		return err
	}
	maxWorkerID := -1
	sm.nodes = make(map[string]*nodeData)
	sm.shards = make([]Shard, 0)

	nodeMap := make(map[int32]string)
	for _, v := range msg.Nodes {
		nodeMap[v.NodeId] = v.NodeName
		newNode := newNodeData(v.NodeName)
		newNode.WorkerID = int(v.WorkerId)
		if newNode.WorkerID > maxWorkerID {
			maxWorkerID = newNode.WorkerID
		}
		sm.nodes[v.NodeName] = newNode
	}
	for _, v := range msg.Shards {
		newShard := NewShard(int(v.Id))
		sm.nodes[nodeMap[v.NodeId]].AddShard(newShard)
		sm.shards = append(sm.shards, newShard)
	}
	sm.workerIDCounter = maxWorkerID
	return nil
}

func (sm *simpleShardMap) ShardCountForNode(nodeid string) int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	node, ok := sm.nodes[nodeid]
	if !ok {
		return 0
	}
	return len(node.Shards)
}

func (sm *simpleShardMap) WorkerID(nodeID string) int {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	node, ok := sm.nodes[nodeID]
	if !ok {
		return -1
	}
	return node.WorkerID
}

func (sm *simpleShardMap) DeletedShards(nodeID string, oldMap ShardMap) []Shard {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()

	return sm.notInSet(nodeID, sm.shards, oldMap.Shards())
}

func (sm *simpleShardMap) NewShards(nodeID string, oldMap ShardMap) []Shard {
	sm.mutex.RLock()
	defer sm.mutex.RUnlock()
	return sm.notInSet(nodeID, oldMap.Shards(), sm.shards)
}

// Return the shards in set2 but not in set1
func (sm *simpleShardMap) notInSet(nodeID string, set1 []Shard, set2 []Shard) []Shard {
	existing := make(map[int]int)
	for _, v := range set1 {
		if v.NodeID() == nodeID {
			existing[v.ID()] = v.ID()
		}
	}

	var removed []Shard
	for _, v := range set2 {
		if v.NodeID() == nodeID {
			if _, ok := existing[v.ID()]; !ok {
				removed = append(removed, v)
			}
		}
	}
	return removed
}
