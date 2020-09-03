package clientfunk

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
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
)

// EndpointMonitor monitors a Serf cluster for endpoint changes. Endpoints
// registered by servers with the Raft endpoint will be added to the gRPC
// endpoint list and used by the client. Note that the endpoints might not be
// valid at all times so errors may occur if the server is not yet a member of
// the cluster or simply unaavaible. Serf is more resilient to network
// partitions than Raft so a server might be in the Serf cluster while not a
// member of a quorum in Raft so the client should handle invalid endpoints
// gracefully.
type EndpointMonitor interface {
	// Stop halts the endpoint monitor and releases all of the resources
	Stop()

	// WaitForEndpoints waits for the first endpoint to be set.
	WaitForEndpoints()

	// ServiceEndpoints returns a list of service endpoints in the Serf cluster
	// which isn't from Raft nodes, ie associated services
	ServiceEndpoints() []Endpoint

	// ClusterEndpoints returns a list of cluster endpoints in the Serf cluster
	// that's also members of the cluster, ie endpoints in the cluster.
	ClusterEndpoints() []Endpoint
}

// Endpoint is a type to hold endpoint information
type Endpoint struct {
	ClusterMember bool   // Set to true if a member of the cluster
	Name          string // Name of endpoint
	Endpoint      string // Address:port of endpoint
}

type endpointMonitor struct {
	serfNode  *funk.SerfNode
	wait      chan bool
	endpoints []Endpoint
	mutex     *sync.Mutex
}

func (e *endpointMonitor) start(clientName, clusterName string, zeroConf bool, config funk.SerfParameters) error {
	config.Final()
	if zeroConf && config.JoinAddress == "" {
		reg := toolbox.NewZeroconfRegistry(clusterName)
		addrs, err := reg.Resolve(funk.ZeroconfSerfKind, 1*time.Second)
		if err != nil {
			return err
		}
		if len(addrs) == 0 {
			return errors.New("no clusters found in zeroconf")
		}
		config.JoinAddress = addrs[0]
	}
	if clientName == "" {
		clientName = fmt.Sprintf("client_%s", toolbox.RandomID())
	}
	go e.serfEventLoop(e.serfNode.Events(), clientName)
	return e.serfNode.Start(clientName, config)
}

func (e *endpointMonitor) newNode(n funk.SerfMember) {
	for k, v := range n.Tags {
		if strings.HasPrefix(k, funk.EndpointPrefix) {
			newEP := Endpoint{
				ClusterMember: n.Tags[funk.RaftEndpoint] != "",
				Name:          k,
				Endpoint:      v,
			}
			e.mutex.Lock()
			e.endpoints = append(e.endpoints, newEP)
			e.mutex.Unlock()

			// This is an endpoint inside the cluster
			// TODO: rewrite to use internal type
			if newEP.ClusterMember {
				AddClusterEndpoint(newEP.Name, newEP.Endpoint)
			}
		}
	}
}

func (e *endpointMonitor) removeNode(n funk.SerfMember) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	for k, v := range n.Tags {
		for i, ep := range e.endpoints {
			if ep.Name == k && ep.Endpoint == v {
				e.endpoints = append(e.endpoints[:i], e.endpoints[i+1:]...)
				if ep.ClusterMember {
					RemoveClusterEndpoint(ep.Name, ep.Endpoint)
				}
				return
			}
		}
	}
}

func (e *endpointMonitor) serfEventLoop(events <-chan funk.NodeEvent, clientName string) {
	for ev := range events {
		if ev.Node.NodeID == clientName {
			// Read existing nodes from Serf and add the endpoints
			for _, v := range e.serfNode.LoadMembers() {
				e.newNode(v)
			}
			close(e.wait)
		}
		switch ev.Event {
		case funk.SerfNodeJoined, funk.SerfNodeUpdated:
			e.newNode(ev.Node)
		case funk.SerfNodeLeft:
			e.removeNode(ev.Node)
		}
	}
}

func (e *endpointMonitor) Stop() {
	if err := e.serfNode.Stop(); err != nil {
		log.WithError(err).Debug("Got error stopping Serf client. Ignoring it.")
	}
}

func (e *endpointMonitor) WaitForEndpoints() {
	<-e.wait
}

func (e *endpointMonitor) ClusterEndpoints() []Endpoint {
	var ret []Endpoint
	e.mutex.Lock()
	defer e.mutex.Unlock()
	for _, v := range e.endpoints {
		if v.ClusterMember {
			ret = append(ret, v)
		}
	}
	return ret
}

func (e *endpointMonitor) ServiceEndpoints() []Endpoint {
	var ret []Endpoint
	e.mutex.Lock()
	defer e.mutex.Unlock()
	for _, v := range e.endpoints {
		if !v.ClusterMember {
			ret = append(ret, v)
		}
	}
	return ret
}

// StartEndpointMonitor monitors the Serf cluster for endpoints and updates
// the gRPC endpoint list continuously. The client will be a Serf node but not
// a Raft node and member of the cluster.
// The client name is optional. If it is empty a random client name will be set.
func StartEndpointMonitor(clientName, clusterName string, zeroConf bool, serfParameters funk.SerfParameters) (EndpointMonitor, error) {
	ret := &endpointMonitor{
		serfNode:  funk.NewSerfNode(),
		wait:      make(chan bool),
		endpoints: make([]Endpoint, 0),
		mutex:     &sync.Mutex{},
	}
	if err := ret.start(clientName, clusterName, zeroConf, serfParameters); err != nil {
		return nil, err
	}
	return ret, nil
}
