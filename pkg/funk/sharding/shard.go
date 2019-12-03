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
type weightedShard struct {
	id     int
	weight int
	nodeID string
}

// NewShard creates a new shard with a weight
func NewShard(id, weight int) Shard {
	return &weightedShard{
		id:     id,
		weight: weight,
		nodeID: "",
	}
}

func (w *weightedShard) ID() int {
	return w.id
}

func (w *weightedShard) Weight() int {
	return w.weight
}

func (w *weightedShard) NodeID() string {
	return w.nodeID
}

func (w *weightedShard) SetNodeID(nodeID string) {
	w.nodeID = nodeID
}
