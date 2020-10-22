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
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/sirupsen/logrus"

	"github.com/hashicorp/serf/serf"
)

// SerfEventType is the type of events the SerfNode emits
type SerfEventType int

// Serf event types.
const (
	SerfNodeJoined  SerfEventType = iota // A node joins the cluster
	SerfNodeLeft                         // A node has left the cluster
	SerfNodeUpdated                      // A node's tags are updated
)

var (
	// SerfLeft is the status of the serf node when it has left the cluster
	SerfLeft = serf.StatusLeft.String()
	// SerfAlive is the status of the serf node when it is alive and well
	SerfAlive = serf.StatusAlive.String()
	// SerfFailed is the status of the serf node when it has failed
	SerfFailed = serf.StatusFailed.String()
)

func (s SerfEventType) String() string {
	switch s {
	case SerfNodeJoined:
		return "SerfNodeJoined"
	case SerfNodeLeft:
		return "SerfNodeLeft"
	case SerfNodeUpdated:
		return "SerfNodeUpdated"
	default:
		panic(fmt.Sprintf("Unknown serf node type %d", s))
	}
}

// NodeEvent is used for channel notifications
type NodeEvent struct {
	Event SerfEventType
	Node  SerfMember
}

// SerfMember holds information on members in the Serf cluster.
type SerfMember struct {
	NodeID string
	State  string
	Tags   map[string]string
}

// SerfNode is a wrapper around the Serf library
type SerfNode struct {
	mutex         *sync.RWMutex
	se            *serf.Serf
	tags          map[string]string // Local tags.
	changedTags   bool              // Keeps track of changes in tags.
	notifications []chan NodeEvent
	members       map[string]SerfMember
}

// NewSerfNode creates a new SerfNode instance
func NewSerfNode() *SerfNode {
	ret := &SerfNode{
		mutex:         &sync.RWMutex{},
		tags:          make(map[string]string),
		notifications: make([]chan NodeEvent, 0),
		members:       make(map[string]SerfMember),
	}
	return ret
}

// SerfParameters holds parameters for the Serf client
type SerfParameters struct {
	Endpoint    string `kong:"help='Endpoint for Serf',default=''"`
	JoinAddress string `kong:"help='Join address and port for Serf cluster'"`
	Verbose     bool   `kong:"help='Verbose logging for Serf'"`
}

// Final populates empty fields with default values
func (s *SerfParameters) Final() {
	if s.Endpoint == "" {
		s.Endpoint = toolbox.RandomPublicEndpoint()
	}
}

// Start launches the serf node
func (s *SerfNode) Start(nodeID string, cfg SerfParameters) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.se != nil {
		return errors.New("serf node is already started")
	}

	config := serf.DefaultConfig()
	config.NodeName = nodeID
	host, portStr, err := net.SplitHostPort(cfg.Endpoint)
	if err != nil {
		return err
	}
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return err
	}

	// Lower the default reap and tombstone intervals
	// The tombstone timeout is for nodes that leave
	// gracefully.
	config.ReapInterval = time.Minute * 5
	config.TombstoneTimeout = time.Minute * 10
	config.MemberlistConfig.BindAddr = host
	config.MemberlistConfig.BindPort = int(port)

	// The advertise address is the public-reachable address. Since we're running on
	// a LAN (or a LAN-like) infrastructure this is the public IP address of the host.
	config.MemberlistConfig.AdvertiseAddr = host
	config.MemberlistConfig.AdvertisePort = int(port)

	config.SnapshotPath = "" // empty since we're using dynamic clusters.

	config.Init()
	eventCh := make(chan serf.Event)
	config.EventCh = eventCh

	if cfg.Verbose {
		config.Logger = log.New(os.Stderr, "serf", log.LstdFlags)
	} else {
		mutedLogger := newMutedLogger()
		config.Logger = mutedLogger
		config.MemberlistConfig.Logger = mutedLogger
	}

	// Assign tags
	config.Tags = s.tags

	go s.serfEventHandler(eventCh)

	if s.se, err = serf.Create(config); err != nil {
		return err
	}

	if cfg.JoinAddress != "" {
		_, err := s.se.Join([]string{cfg.JoinAddress}, true)
		if err != nil {
			return err
		}
	}
	return nil

}

// Stop shuts down the node
func (s *SerfNode) Stop() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.se == nil {
		return errors.New("serf node is not started")
	}
	if err := s.se.Leave(); err != nil {
		return err
	}
	s.se = nil
	return nil
}

// SetTag sets a tag on the serf node. The tags are not updated until PublishTags
// are called by the client
func (s *SerfNode) SetTag(name, value string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if value == "" {
		logrus.Debugf("Deleting tag %s", name)
		delete(s.tags, name)
		return
	}
	s.tags[name] = value
	s.changedTags = true
}

// GetClusterTag returns the first tag in the cluster that matches the name.
// if no matching tag is found an empty string is returned.
func (s *SerfNode) GetClusterTag(name string) string {
	for _, node := range s.Nodes() {
		for k, v := range node.Tags {
			if k == name {
				return v
			}
		}
	}
	return ""
}

// PublishTags publishes the tags to the other members of the cluster
func (s *SerfNode) PublishTags() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.se == nil {
		// *technically* this should be an error but the tags will be published
		// once the node goes live.
		return nil
	}
	if !s.changedTags {
		return nil
	}
	logrus.WithField("tags", s.tags).Debug("publishing tags")
	s.changedTags = false
	return s.se.SetTags(s.tags)
}

// Events returns a notification channel. If the client isn't reading the events
// will be dropped.
func (s *SerfNode) Events() <-chan NodeEvent {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	newChan := make(chan NodeEvent)
	s.notifications = append(s.notifications, newChan)
	return newChan
}

// Node returns information on a particular node. If the node isn't found the
// node returned will be empty
func (s *SerfNode) Node(nodeID string) SerfMember {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return s.members[nodeID]
}

// Nodes returns a list of known member nodes
func (s *SerfNode) Nodes() []SerfMember {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	ret := make([]SerfMember, 0)

	for k, v := range s.members {
		n := SerfMember{}
		n.NodeID = k
		n.Tags = make(map[string]string)
		for name, value := range v.Tags {
			n.Tags[name] = value
		}
		ret = append(ret, n)
	}
	return ret
}

// Size returns the size of the member list
func (s *SerfNode) Size() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.members)
}

// addMember adds a new member. Returns true if the member does not exist
func (s *SerfNode) addMember(nodeID string, state string, tags map[string]string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing, ok := s.members[nodeID]
	if !ok {
		existing = SerfMember{NodeID: nodeID, Tags: tags, State: state}
		s.members[nodeID] = existing
		s.sendEvent(NodeEvent{
			Event: SerfNodeJoined,
			Node:  existing,
		})
		return
	}
	s.sendEvent(NodeEvent{
		Event: SerfNodeUpdated,
		Node:  existing,
	})
	existing.Tags = tags
	s.members[nodeID] = existing
}

// removeMember removes a member from the collection. Returns true if the member doe snot e
func (s *SerfNode) removeMember(nodeID string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing, ok := s.members[nodeID]
	if !ok {
		return
	}
	s.sendEvent(NodeEvent{
		Event: SerfNodeLeft,
		Node:  existing,
	})

	delete(s.members, nodeID)
}

func (s *SerfNode) updateMember(nodeID string, tags map[string]string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing, ok := s.members[nodeID]
	if !ok {
		existing.NodeID = nodeID
		existing.Tags = tags
		s.sendEvent(NodeEvent{
			Event: SerfNodeJoined,
			Node:  existing,
		})
		return
	}
	existing.Tags = tags
	s.members[nodeID] = existing
	s.sendEvent(NodeEvent{
		Event: SerfNodeUpdated,
		Node:  existing,
	})

}

func (s *SerfNode) sendEvent(ev NodeEvent) {
	for _, v := range s.notifications {
		select {
		case v <- ev:
		default:
		}
	}
}

func (s *SerfNode) serfEventHandler(events chan serf.Event) {
	for ev := range events {
		switch ev.EventType() {

		case serf.EventMemberJoin:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.addMember(v.Name, v.Status.String(), v.Tags)
			}

		case serf.EventMemberLeave:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.removeMember(v.Name)
			}

		case serf.EventMemberUpdate:
			// No need to process member updates
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.updateMember(v.Name, v.Tags)
			}

		case serf.EventMemberFailed:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.removeMember(v.Name)
			}

		case serf.EventMemberReap, serf.EventUser, serf.EventQuery:
			// Do nothing

		default:
			logrus.WithField("event", ev).Error("Unknown event")
		}
	}
}

// LoadMembers reads the list of existing members in the Serf cluster
func (s *SerfNode) LoadMembers() []SerfMember {
	var ret []SerfMember
	for _, v := range s.se.Members() {
		newNode := SerfMember{
			NodeID: v.Name,
			State:  v.Status.String(),
			Tags:   make(map[string]string),
		}
		for k, v := range v.Tags {
			newNode.Tags[k] = v
		}
		ret = append(ret, newNode)
	}
	return ret
}

func (s *SerfNode) memberList() []nodeItem {
	var ret []nodeItem
	for _, v := range s.se.Members() {
		ret = append(ret, nodeItem{
			ID:     v.Name,
			State:  v.Status.String(),
			Leader: false,
		})
	}
	return ret
}

// ID returns the
func (s *SerfNode) ID() string {
	return s.se.LocalMember().Name
}

// Endpoints returns all the endpoints in the cluster
func (s *SerfNode) Endpoints() []Endpoint {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	// Get a refreshed list of endpoints from the serf implementation
	endpoints := make([]Endpoint, 0)
	if s.se == nil {
		// TODO(stalehd): Consider changing the startup to make this a non-issue
		panic("Serf node is nil. Can't retrieve endpoints without a running node")
	}
	for _, m := range s.se.Members() {
		_, clusterNode := m.Tags[RaftEndpoint]
		for k, v := range m.Tags {
			if strings.HasPrefix(k, EndpointPrefix) {
				endpoints = append(endpoints, Endpoint{
					NodeID:        m.Name,
					Name:          k,
					ListenAddress: v,
					Local:         (m.Name == s.ID()),
					Cluster:       clusterNode,
				})
			}
		}
	}
	return endpoints
}

type muteWriter struct {
}

func (m *muteWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

// GetMutedLogger returns a pointer to a logrus.Logger instance that is logging
// to the Big Bit Bucket In The Sky...or Cloud
func newMutedLogger() *log.Logger {
	return log.New(&muteWriter{}, "sssh", log.LstdFlags)
}
