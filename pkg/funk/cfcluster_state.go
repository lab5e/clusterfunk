package funk

import (
	"time"

	"github.com/sirupsen/logrus"
)

// This file contains the internal state methods
func (c *clusterfunkCluster) logStateChange() {
	logrus.WithFields(logrus.Fields{
		"state": c.state.String(),
		"role":  c.role.String()}).Debug("State changed")
}

func (c *clusterfunkCluster) sendEvent(ev Event) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	for _, v := range c.eventChannels {
		select {
		case v <- ev:
			// great success
		case <-time.After(1 * time.Second):
			// drop event
		}
	}
}

func (c *clusterfunkCluster) setState(newState NodeState) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	if c.state != newState {
		c.state = newState
		c.logStateChange()
		c.sendEvent(Event{State: newState, Role: c.role})
	}
}

func (c *clusterfunkCluster) State() NodeState {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.state
}

func (c *clusterfunkCluster) Role() NodeRole {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.role
}

func (c *clusterfunkCluster) setRole(newRole NodeRole) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.role = newRole
	c.logStateChange()
}

func (c *clusterfunkCluster) ProcessedIndex() uint64 {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.processedIndex
}

// SetProcessedIndex sets the last processed index. The returned value is
// the current value of the processed index.
func (c *clusterfunkCluster) setProcessedIndex(index uint64) uint64 {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	if index > c.processedIndex {
		c.processedIndex = index
	}
	return c.processedIndex
}

func (c *clusterfunkCluster) CurrentShardMapIndex() uint64 {
	c.stateMutex.RLock()
	defer c.stateMutex.RUnlock()
	return c.currentShardMapIndex
}

func (c *clusterfunkCluster) setCurrentShardMapIndex(index uint64) {
	c.stateMutex.Lock()
	defer c.stateMutex.Unlock()
	c.currentShardMapIndex = index
}
