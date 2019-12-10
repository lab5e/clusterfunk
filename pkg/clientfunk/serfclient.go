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
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/ExploratoryEngineering/clusterfunk/pkg/funk"
	"github.com/ExploratoryEngineering/clusterfunk/pkg/toolbox"
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
}

type endpointMonitor struct {
	serfNode *funk.SerfNode
	wait     chan bool
}

func (e *endpointMonitor) Start(clientName, clusterName string, zeroConf bool, config funk.SerfParameters) error {
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
	if n.Tags[funk.RaftEndpoint] == "" {
		// ignore node; this isn't a Raft node
		return
	}
	for k, v := range n.Tags {
		if strings.HasPrefix(k, funk.EndpointPrefix) {
			AddClusterEndpoint(k, v)
		}
	}
}

func (e *endpointMonitor) removeNode(n funk.SerfMember) {
	if n.Tags[funk.RaftEndpoint] == "" {
		// ignore node; this isn't a Raft node
		return
	}
	for k, v := range n.Tags {
		// remove endpoint
		if strings.HasPrefix(k, funk.EndpointPrefix) {
			RemoveClusterEndpoint(k, v)
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

// StartEndpointMonitor monitors the Serf cluster for endpoints and updates
// the gRPC endpoint list continuously. The client will be a Serf not but not
// a member of the Raft cluster.
// The client name is optional. If it is empty a random client name will be set.
func StartEndpointMonitor(clientName, clusterName string, zeroConf bool, serfParameters funk.SerfParameters) (EndpointMonitor, error) {
	ret := &endpointMonitor{
		serfNode: funk.NewSerfNode(),
		wait:     make(chan bool),
	}
	return ret, ret.Start(clientName, clusterName, zeroConf, serfParameters)
}
