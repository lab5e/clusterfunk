package cluster

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
