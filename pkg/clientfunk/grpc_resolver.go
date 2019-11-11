package clientfunk

import (
	"fmt"
	"sync"

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

// GRPCServiceConfig is the default service config for Clusterfunk clients. It will enable retries.
const GRPCServiceConfig = `{
	"loadBalancingPolicy": "round_robin",
	"methodConfig": [{
		"retryPolicy": {
			"MaxAttempts": 10,
			"InitialBackoff": ".1s",
			"MaxBackoff": "2s",
			"BackoffMultiplier": 3.0,
			"RetryableStatusCodes": [ "UNAVAILABLE" ]
		}
	}]
}`

var resolverBuilder *clusterResolverBuilder
var mutex = &sync.RWMutex{}
var cfEndpoints = make(map[string][]resolver.Address)

type clusterResolverBuilder struct {
}

func (b *clusterResolverBuilder) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOption) (resolver.Resolver, error) {
	if target.Scheme != ClusterfunkSchemaName {
		return nil, fmt.Errorf("unsupported scheme: %s", target.Scheme)
	}
	mutex.RLock()
	defer mutex.RUnlock()

	addrs, ok := cfEndpoints[target.Endpoint]
	if !ok {
		return nil, fmt.Errorf("unknown endpoint: %s", target.Endpoint)
	}

	// Look up the name and respond with the updated list.
	cc.UpdateState(resolver.State{
		Addresses: addrs,
	})
	return &clusterResolver{
		Target:     target.Endpoint,
		ClientConn: cc,
	}, nil
}

func (b *clusterResolverBuilder) Scheme() string {
	return ClusterfunkSchemaName
}

// UpdateClusterEndpoints updates the cluster endpoints for the gRPC resolver.
// This will update the list of endpoints with the given name.
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

type clusterResolver struct {
	Target     string
	ClientConn resolver.ClientConn
}

func (c *clusterResolver) ResolveNow(resolver.ResolveNowOption) {
}

func (c *clusterResolver) Close() {
	// nothing to do
}
