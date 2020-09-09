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
	"fmt"
	"sync"

	"github.com/lab5e/clusterfunk/pkg/funk"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/resolver"
)

// TODO: Integrate the client and endpoint monitor into the resolverBuilder

var resolverBuilder *clusterResolverBuilder

func init() {
	resolverBuilder = &clusterResolverBuilder{
		mutex:             &sync.RWMutex{},
		resolverEndpoints: make(map[string][]resolver.Address),
	}

	// This must be called during init -- see gRPC documentation
	resolver.Register(resolverBuilder)
}

// ClusterfunkSchemaName is the schema used for the connections. Each server
// connection uses clusterfunk://[endpointname] when resolving. This should be
// combined with the service config settings included in this package.
const ClusterfunkSchemaName = "cluster"

// GRPCServiceConfig is the default service config for Clusterfunk clients.
// It will enable retries when the service is down but not when the call times
// out.
const GRPCServiceConfig = `{
	"loadBalancingPolicy": "round_robin"
}`

type clusterResolverBuilder struct {
	endpointChan      <-chan funk.Endpoint
	resolverEndpoints map[string][]resolver.Address
	mutex             *sync.RWMutex
}

func (b *clusterResolverBuilder) registerObserver(o funk.EndpointObserver) {
	b.mutex.Lock()
	ch := b.endpointChan
	b.mutex.Unlock()
	if ch != nil {
		return
	}
	for _, ep := range o.Endpoints() {
		b.addEndpoint(ep)
	}
	b.endpointChan = o.Observe()
	go b.observeEndpoints()
}

func (b *clusterResolverBuilder) observeEndpoints() {
	for ep := range b.endpointChan {
		if ep.Active {
			b.addEndpoint(ep)
			continue
		}
		b.removeEndpoint(ep)
	}
}

// updateEndpoints initiates the list with a new set of endpoints
func (b *clusterResolverBuilder) updateEndpoints(endpoints []funk.Endpoint) {
	b.mutex.Lock()
	defer b.mutex.Unlock()

	for _, ep := range endpoints {
		list, ok := b.resolverEndpoints[ep.Name]
		if !ok {
			list = make([]resolver.Address, 0)
		}
		list = append(list, resolver.Address{
			Addr: ep.ListenAddress,
		})
		b.resolverEndpoints[ep.Name] = list
	}
}

func (b *clusterResolverBuilder) addEndpoint(ep funk.Endpoint) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	eps, ok := b.resolverEndpoints[ep.Name]
	if !ok {
		eps = make([]resolver.Address, 0)
	}
	eps = append(eps, resolver.Address{Addr: ep.ListenAddress})
	b.resolverEndpoints[ep.Name] = eps
}

func (b *clusterResolverBuilder) removeEndpoint(ep funk.Endpoint) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	eps, ok := b.resolverEndpoints[ep.Name]
	if !ok {
		return
	}
	for i, v := range eps {
		if v.Addr == ep.ListenAddress {
			eps = append(eps[:i], eps[i+1:]...)
			break
		}
	}
	if len(eps) == 0 {
		delete(b.resolverEndpoints, ep.Name)
		return
	}
	b.resolverEndpoints[ep.Name] = eps

}

func (b *clusterResolverBuilder) getAddresses(name string) []resolver.Address {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	ret, ok := b.resolverEndpoints[name]
	if !ok {
		return nil
	}
	return ret
}
func (b *clusterResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	if target.Scheme != ClusterfunkSchemaName {
		return nil, fmt.Errorf("unsupported scheme: %s", target.Scheme)
	}
	b.mutex.RLock()
	defer b.mutex.RUnlock()

	ret := &clusterResolver{
		source: b,
		cc:     cc,
		target: target,
	}
	return ret, ret.updateState()
}

func (b *clusterResolverBuilder) Scheme() string {
	return ClusterfunkSchemaName
}

type clusterResolver struct {
	source *clusterResolverBuilder
	cc     resolver.ClientConn
	target resolver.Target
}

func (c *clusterResolver) updateState() error {
	addrs := c.source.getAddresses(c.target.Endpoint)
	if addrs == nil {
		return fmt.Errorf("unknown endpoint (%s) for resolver", c.target.Endpoint)
	}
	// Look up the name and respond with the updated list.
	c.cc.UpdateState(resolver.State{
		Addresses: addrs,
	})
	return nil
}

func (c *clusterResolver) ResolveNow(resolver.ResolveNowOptions) {
	if err := c.updateState(); err != nil {
		log.WithError(err).Error("couldn't re-resolve endpoint")
	}
}

func (c *clusterResolver) Close() {
	// nothing to do
}
