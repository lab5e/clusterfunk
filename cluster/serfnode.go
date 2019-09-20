package cluster

import (
	"errors"
	"log"
	"net"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/hashicorp/serf/serf"
)

// NodeEvent is used for channel notifications
type NodeEvent struct {
	NodeID string
	Joined bool
	Tags   map[string]string
}

// SerfNode is a wrapper around the Serf library
type SerfNode struct {
	mutex         *sync.RWMutex
	se            *serf.Serf
	tags          map[string]string
	changedTags   bool // Keeps track of changes in tags.
	notifications []chan NodeEvent
}

// NewSerfNode creates a new SerfNode instance
func NewSerfNode() *SerfNode {
	ret := &SerfNode{
		mutex:         &sync.RWMutex{},
		tags:          make(map[string]string),
		notifications: make([]chan NodeEvent, 0),
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
		config.Logger = log.New(os.Stderr, "serf", log.LstdFlags)
	} else {
		serfLogger := newClusterLogger("serf")
		config.Logger = serfLogger.Logger
		config.MemberlistConfig.Logger = serfLogger.Logger
	}

	// Assign tags and default endpoint
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
	if s.tags[name] != value {
		s.tags[name] = value
		s.changedTags = true
	}
}

// PublishTags publishes the tags to the other members of the cluster
func (s *SerfNode) PublishTags() error {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	if s.se == nil {
		return errors.New("serf node not created")
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

// ----------------------------------------------------------------------------
// Possible keep this internal. It is nice to have a view into the Serf and
// raft internals but maybe not for a management tools since it operates on
// a slighlty higher level. Methods below are TBD
//

//
// MemberCount is a temporary method until we've made a layer on top of Raft and Serf.
func (s *SerfNode) MemberCount() int {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	return len(s.se.Members())
}

// SerfMemberInfo holds info on a single Serf member
type SerfMemberInfo struct {
	NodeID string
	Status string
	Tags   map[string]string
}

// Members lists the members in
func (s *SerfNode) Members() []SerfMemberInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	ret := make([]SerfMemberInfo, 0)
	for _, v := range s.se.Members() {
		ret = append(ret, SerfMemberInfo{
			NodeID: v.Name,
			Status: v.Status.String(),
			Tags:   v.Tags,
		})
	}
	return ret
}

func (s *SerfNode) sendEvent(ev NodeEvent) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
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
				s.sendEvent(NodeEvent{
					NodeID: v.Name,
					Tags:   v.Tags,
					Joined: true,
				})
			}
		case serf.EventMemberLeave:
			e, ok := ev.(serf.MemberEvent)
			if !ok {
				continue
			}
			for _, v := range e.Members {
				s.sendEvent(NodeEvent{
					NodeID: v.Name,
					Tags:   v.Tags,
					Joined: false,
				})
			}
		case serf.EventMemberReap:
		case serf.EventMemberUpdate:
			// No need to process member updates
		case serf.EventUser:
		case serf.EventQuery:

		default:
			log.Printf("Unknown event: %+v", ev)
		}
	}
}
