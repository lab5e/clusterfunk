package funk

import (
	"errors"
	"fmt"
	golog "log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

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
		members:       make(map[string]SerfMember, 0),
	}
	return ret
}

// SerfParameters holds parameters for the Serf client
type SerfParameters struct {
	Endpoint    string `param:"desc=Endpoint for Serf;default="`
	JoinAddress string `param:"desc=Join address and port for Serf cluster"`
}

// Start launches the serf node
func (s *SerfNode) Start(nodeID string, verboseLogging bool, cfg SerfParameters) error {
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

	if verboseLogging {
		config.Logger = golog.New(os.Stderr, "serf", golog.LstdFlags)
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
		delete(s.tags, name)
		return
	}
	s.tags[name] = value
	s.changedTags = true
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
func (s *SerfNode) addMember(nodeID string, tags map[string]string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	existing, ok := s.members[nodeID]
	if !ok {
		existing = SerfMember{NodeID: nodeID, Tags: tags}
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
				s.addMember(v.Name, v.Tags)
			}
		case serf.EventMemberLeave:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.removeMember(v.Name)
			}
		case serf.EventMemberReap:
		case serf.EventMemberUpdate:
			// No need to process member updates
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.updateMember(v.Name, v.Tags)
			}
		case serf.EventUser:
		case serf.EventQuery:
		case serf.EventMemberFailed:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.removeMember(v.Name)
			}

		default:
			log.WithField("event", ev).Error("Unknown event")
		}
	}
}

type muteWriter struct {
}

func (m *muteWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

// GetMutedLogger returns a pointer to a log.Logger instance that is logging
// to the Big Bit Bucket In The Sky...or Cloud
func newMutedLogger() *golog.Logger {
	return golog.New(&muteWriter{}, "sssh", golog.LstdFlags)
}
