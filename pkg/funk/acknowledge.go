package funk

import (
	"time"

	"github.com/stalehd/clusterfunk/pkg/toolbox"
)

// ackCollection handles acknowledgement and timeouts on acknowledgements
// The collection can be acked one time only and
type ackCollection interface {
	// StartAck starts the acking
	StartAck(nodes []string, timeout time.Duration)

	// Ack adds another node to the acknowledged list
	Ack(nodeID string)

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
	}
}

type ackColl struct {
	nodes         toolbox.StringSet
	completedChan chan struct{}
	missingChan   chan []string
}

func (a *ackColl) StartAck(nodes []string, timeout time.Duration) {
	a.nodes.Sync(nodes...)
	go func() {
		time.Sleep(timeout)
		if a.nodes.Size() > 0 {
			a.missingChan <- a.nodes.List()
		}
	}()
}

func (a *ackColl) Ack(nodeID string) {
	if a.nodes.Remove(nodeID) && a.nodes.Size() == 0 {
		go func() { a.completedChan <- struct{}{} }()
	}
}

func (a *ackColl) MissingAck() <-chan []string {
	return a.missingChan
}

func (a *ackColl) Completed() <-chan struct{} {
	return a.completedChan
}

func (a *ackColl) Done() {
	close(a.missingChan)
	close(a.completedChan)
}
