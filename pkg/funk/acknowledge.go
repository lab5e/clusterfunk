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
import (
	"sync/atomic"
	"time"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/toolbox"
)

// ackCollection handles acknowledgement and timeouts on acknowledgements
// The collection can be acked one time only and
type ackCollection interface {
	// StartAck starts the acking
	StartAck(nodes []string, shardIndex uint64, timeout time.Duration)

	// Ack adds another node to the acknowledged list. Returns true if that node
	// is in the list of nodes that should ack
	Ack(nodeID string, shardIndex uint64) bool

	// ShardIndex returns the shard index that is acked. This returns 0 when
	// the ack is completed.
	ShardIndex() uint64

	// MissingAck returns a channel that sends a list of nodes that haven't acknowledged within the timeout.
	// If something is sent on the MissingAck channel the Completed channel won't trigger.
	MissingAck() <-chan []string

	// Completed returns a channel that is triggered when all nodes have
	// acknowledged
	Completed() <-chan struct{}

	// Done clears up the resources and closes all open channels
	Done()
}

func newAckCollection() ackCollection {
	return &ackColl{
		nodes:         toolbox.NewStringSet(),
		completedChan: make(chan struct{}),
		missingChan:   make(chan []string),
		shardIndex:    new(uint64),
	}
}

type ackColl struct {
	nodes         toolbox.StringSet
	completedChan chan struct{}
	missingChan   chan []string
	shardIndex    *uint64
}

func (a *ackColl) StartAck(nodes []string, shardIndex uint64, timeout time.Duration) {
	atomic.StoreUint64(a.shardIndex, shardIndex)
	a.nodes.Sync(nodes...)
	go func() {
		time.Sleep(timeout)
		if atomic.LoadUint64(a.shardIndex) == 0 {
			// Done() has been called on this so just terminate
			return
		}
		if a.nodes.Size() > 0 {
			a.missingChan <- a.nodes.List()
		}
	}()
}

func (a *ackColl) Ack(nodeID string, shardIndex uint64) bool {
	if atomic.LoadUint64(a.shardIndex) != shardIndex {
		return false
	}
	if atomic.LoadUint64(a.shardIndex) == 0 {
		return false
	}
	if a.nodes.Remove(nodeID) {
		if a.nodes.Size() == 0 {
			go func() { a.completedChan <- struct{}{} }()
		}
		return true
	}
	return false
}

func (a *ackColl) MissingAck() <-chan []string {
	return a.missingChan
}

func (a *ackColl) Completed() <-chan struct{} {
	return a.completedChan
}

func (a *ackColl) Done() {
	if atomic.LoadUint64(a.shardIndex) > 0 {
		atomic.StoreUint64(a.shardIndex, 0)
		close(a.missingChan)
		close(a.completedChan)
	}
}

func (a *ackColl) ShardIndex() uint64 {
	return atomic.LoadUint64(a.shardIndex)
}
