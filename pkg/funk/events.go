package funk
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
// This file contains events emitted by the cluster.
//

// NodeAddedEvent is emitted when a new node is added to the cluster.
type NodeAddedEvent struct {
}

// NodeRemovedEvent is emitted when a node is removed from the cluster
type NodeRemovedEvent struct {
}

// NodeRetiredEvent is emitted when a node is about to be retired.
type NodeRetiredEvent struct {
}

// LocalNodeStoppedEvent is emitted when the local node is stopping
type LocalNodeStoppedEvent struct {
}

// LeaderLostEvent is emitted when the leader is about to change.
type LeaderLostEvent struct {
}

// LeaderChangedEvent is emitted when the leader of the cluster has
// changed.
type LeaderChangedEvent struct {
}
