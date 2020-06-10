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

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/resolver"
)

func init() {
	resolverBuilder = &clusterResolverBuilder{}

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

var resolverBuilder *clusterResolverBuilder
var mutex = &sync.RWMutex{}
var cfEndpoints = make(map[string][]resolver.Address)

type clusterResolverBuilder struct {
}

func (b *clusterResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	if target.Scheme != ClusterfunkSchemaName {
		return nil, fmt.Errorf("unsupported scheme: %s", target.Scheme)
	}
	mutex.RLock()
	defer mutex.RUnlock()

	ret := &clusterResolver{
		cc:     cc,
		target: target,
	}
	return ret, ret.updateState()
}

func (b *clusterResolverBuilder) Scheme() string {
	return ClusterfunkSchemaName
}

type clusterResolver struct {
	cc     resolver.ClientConn
	target resolver.Target
}

func (c *clusterResolver) updateState() error {
	addrs, ok := cfEndpoints[c.target.Endpoint]
	if !ok {
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

// UpdateClusterEndpoints updates all of the cluster endpoints with a given
// name. The existing cluster endpoints with that name is removed from the
// collection used by the gRPC resolver.
func UpdateClusterEndpoints(endpointName string, endpoints []string) {
	mutex.Lock()
	defer mutex.Unlock()

	delete(cfEndpoints, endpointName)

	list := make([]resolver.Address, 0)

	for _, v := range endpoints {
		list = append(list, resolver.Address{
			Addr: v,
		})
	}
	cfEndpoints[endpointName] = list
}

// AddClusterEndpoint adds a single cluster endpoint to the collection used by
// the gRPC resolver. The same endpoint can be added multiple times.
func AddClusterEndpoint(endpointName, endpoint string) {
	mutex.Lock()
	defer mutex.Unlock()
	eps, ok := cfEndpoints[endpointName]
	if !ok {
		eps = make([]resolver.Address, 0)
	}
	eps = append(eps, resolver.Address{Addr: endpoint})
	cfEndpoints[endpointName] = eps
}

// RemoveClusterEndpoint removes a single cluster endpoint from the collection
// used by the gRPC resolver. All endpoints with the given name and endpoint
// will be removed from the collection.
func RemoveClusterEndpoint(endpointName, endpoint string) {
	mutex.Lock()
	defer mutex.Unlock()
	eps, ok := cfEndpoints[endpointName]
	if !ok {
		return
	}
	for i, v := range eps {
		if v.Addr == endpoint {
			eps = append(eps[:i], eps[i+1:]...)
		}
	}
	if len(eps) == 0 {
		delete(cfEndpoints, endpointName)
		return
	}
	cfEndpoints[endpointName] = eps
}
