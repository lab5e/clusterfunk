package funk

import (
	"errors"
	"strings"
	"sync"
)

// Endpoint is
type Endpoint struct {
	NodeID        string // NodeID is the ID of the node that registered the endpoint
	ListenAddress string // ListenAddress is the registered address for the endpoint
	Name          string // Name is the name of the endpoint
	Active        bool   // Active is set to true if the endpoint is from an active node
	Local         bool   // Local is set to true if this is on the local node
	Cluster       bool   // Cluster is set to true if the node is a member of the cluster
}

// EndpointObserver observes the cluster and generates events when endpoints
// are registered and deregistered. Note that the endpoints might be registered
// but the service might not be available. The list of endpoints is only
// advisory. Nodes can't unregister endpoints; once it is registered it will
// stick around until the node goes away. The listen addresses can be changed.
type EndpointObserver interface {
	// Observe returns a channel that will send newly discovered endpoints. The
	// channel is closed when the observer shuts down. The consumers should read
	// the channel as soon as possible.
	Observe() <-chan Endpoint

	// Unobserve turns off observation for the channel
	Unobserve(<-chan Endpoint)

	// Shutdown closes all observer channels
	Shutdown()

	// Endpoints returns the entire list of endpoints. Inactive endpoints are
	// omitted.
	Endpoints() []Endpoint

	// Find returns matching endpoints
	Find(name string) []Endpoint

	// FindEndpoint returns the first partially or complete matching endpoint.
	FindFirst(name string) (Endpoint, error)
}

// NewEndpointObserver creates a new EndpointObserver instance.
func NewEndpointObserver(localNodeID string, events <-chan NodeEvent, existing []Endpoint) EndpointObserver {
	ret := &endpointObserver{
		mutex:       &sync.Mutex{},
		endpoints:   make(map[string][]Endpoint),
		subscribers: make([]chan Endpoint, 0),
	}
	// Populate existing endpoints if they're set.
	for _, ep := range existing {
		ret.appendEndpoints(ep)
	}
	go ret.startObserving(localNodeID, events)
	return ret
}

type endpointObserver struct {
	mutex       *sync.Mutex
	endpoints   map[string][]Endpoint
	subscribers []chan Endpoint
}

func (e *endpointObserver) Observe() <-chan Endpoint {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.endpoints == nil {
		return nil
	}
	ret := make(chan Endpoint, 1)
	e.subscribers = append(e.subscribers, ret)
	return ret
}

func (e *endpointObserver) Unobserve(ch <-chan Endpoint) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.endpoints == nil {
		return
	}
	for i, v := range e.subscribers {
		if v == ch {
			e.subscribers = append(e.subscribers[:i], e.subscribers[i+1:]...)
			close(v)
			return
		}
	}
}

func (e *endpointObserver) Shutdown() {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	for _, v := range e.subscribers {
		close(v)
	}
	e.subscribers = make([]chan Endpoint, 0)

	for k := range e.endpoints {
		delete(e.endpoints, k)
	}
	e.endpoints = nil
}

func (e *endpointObserver) Endpoints() []Endpoint {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	ret := make([]Endpoint, 0)
	for _, eps := range e.endpoints {
		ret = append(ret, eps...)
	}
	return ret
}

// endpointNameMatches returns true if the name matches the search term
func endpointNameMatches(name, search string) bool {
	if name == search {
		return true
	}
	if name[3:] == search {
		return true
	}
	return false
}

// Find returns a list of matching endpoints. Only active endpoints are
// returned.
func (e *endpointObserver) Find(name string) []Endpoint {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	ret := make([]Endpoint, 0)
	for k, eps := range e.endpoints {
		if endpointNameMatches(k, name) {
			ret = append(ret, eps...)
		}
	}
	return ret
}

// FindFirst returns the first matching active endpoint
func (e *endpointObserver) FindFirst(name string) (Endpoint, error) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	for k, eps := range e.endpoints {
		if endpointNameMatches(k, name) && len(eps) > 0 {
			return eps[0], nil
		}
	}
	return Endpoint{}, errors.New("no endpoint found")
}

func (e *endpointObserver) startObserving(localNode string, events <-chan NodeEvent) {
	for ev := range events {
		for name, listen := range ev.Node.Tags {
			if strings.HasPrefix(name, EndpointPrefix) {
				ep := Endpoint{
					NodeID:        ev.Node.NodeID,
					ListenAddress: listen,
					Name:          name,
					Local:         (ev.Node.NodeID == localNode),
					Cluster:       (ev.Node.Tags[RaftEndpoint] != ""),
				}
				switch ev.Event {
				case SerfNodeJoined:
					e.appendEndpoints(ep)
				case SerfNodeLeft:
					e.removeEndpoints(ep)
				case SerfNodeUpdated:
					switch ev.Node.State {
					case SerfAlive:
						e.appendEndpoints(ep)
					default:
						// ie SerfFailed
						e.removeEndpoints(ep)
					}
				}
			}
		}
	}
}

func (e *endpointObserver) appendEndpoints(ep Endpoint) {
	ep.Active = true
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if e.endpoints == nil {
		// we've shut down
		return
	}

	for _, v := range e.subscribers {
		v <- ep
	}

	for name, eps := range e.endpoints {
		if name == ep.Name {
			// Append to list if it doesn't already exist
			for i, v := range eps {
				if v.Name == ep.Name && v.NodeID == ep.NodeID {
					// Check if the listen address has changed
					if v.ListenAddress != ep.ListenAddress {
						eps[i] = ep
						e.endpoints[name] = eps
					}
					return
				}
			}
			eps = append(eps, ep)
			e.endpoints[name] = eps
			return
		}
	}
	eps := make([]Endpoint, 0)
	eps = append(eps, ep)
	e.endpoints[ep.Name] = eps
}

func (e *endpointObserver) removeEndpoints(ep Endpoint) {
	ep.Active = false
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for _, v := range e.subscribers {
		v <- ep
	}

	for name, eps := range e.endpoints {
		if name == ep.Name {
			// Remove from list if it exists
			for i, v := range eps {
				if v.ListenAddress == ep.ListenAddress {
					eps = append(eps[:i], eps[i+1:]...)
					e.endpoints[name] = eps
					return
				}
			}
		}
	}
}
