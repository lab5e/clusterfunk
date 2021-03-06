package clientfunk

import (
	"errors"
	"fmt"
	"time"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/toolbox"
)

// Client is a cluster client interface.
type Client interface {
	// Register an endpoint . Clients do not provide endpoints but if they
	// provide some other interface out to the rest of the world it's nice to
	// include it in the node metadata.
	RegisterEndpoint(name string, listenAddress string) error

	// WaitForEndpoint waits for an endpoint to become available.
	WaitForEndpoint(name string)

	// Endpoints returns all of the available endpoints
	Endpoints() []funk.Endpoint

	// RefreshPeers refreshes the list of peers. This ensures you'll have a
	// reasonable updated worldview.
	RefreshPeers()
}

// ClientParameters is the client configuration parameters
type ClientParameters struct {
	ClusterName     string   `kong:"help='Name of cluster',default='clusterfunk'"`
	Name            string   `kong:"help='Client name',default='client'"`
	ZeroConf        bool     `kong:"help='Enable/disable ZeroConf/mDNS for cluster lookups',default='true'"`
	SerfJoinAddress []string `kong:"help='Serf nodes to join'"`
	Verbose         bool     `kong:"help='Verbose logging'"`
}

// NewClusterClient creates a new cluster client. clusterName is the name of the
// cluster. If zeroConf is set to true mDNS/ZeroConf will be used to find a
// serf node to attach to. If the zeroConf parameter is set to false the
// seedNode parameter is used to attach to a Serf node.
func NewClusterClient(params ClientParameters) (Client, error) {
	serfConfig := funk.SerfParameters{}
	serfConfig.JoinAddress = params.SerfJoinAddress
	serfConfig.Verbose = params.Verbose
	serfConfig.Final()
	cc := &clusterClient{}
	if params.ZeroConf {
		reg := toolbox.NewZeroconfRegistry(params.ClusterName)
		addrs, err := reg.Resolve(funk.ZeroconfSerfKind, 1*time.Second)
		if err != nil {
			return nil, err
		}
		if len(addrs) == 0 {
			return nil, errors.New("no clusters found in zeroconf")
		}
		serfConfig.JoinAddress = addrs
	}
	cc.serfNode = funk.NewSerfNode()
	nodeID := fmt.Sprintf("%s_%s", params.Name, toolbox.RandomID())
	cc.serfNode.SetTag(funk.SerfServiceName, params.Name)
	if err := cc.serfNode.Start(nodeID, "", serfConfig); err != nil {
		return nil, err
	}
	cc.observer = funk.NewEndpointObserver(nodeID, cc.serfNode.Events(), cc.serfNode.Endpoints())
	// Attach this to the resolverBuilder if required
	resolverBuilder.registerObserver(cc.observer)
	return cc, nil
}

type clusterClient struct {
	serfNode *funk.SerfNode
	observer funk.EndpointObserver
}

func (c *clusterClient) RegisterEndpoint(name, listenAddress string) error {
	c.serfNode.SetTag(name, listenAddress)
	return c.serfNode.PublishTags()
}

func (c *clusterClient) RefreshPeers() {
	c.serfNode.LoadMembers()
}

func (c *clusterClient) WaitForEndpoint(name string) {
	obs := c.observer.Observe()
	found := make(chan string, 1)
	go func() {
		defer c.observer.Unobserve(obs)
		defer close(found)
		for ev := range obs {
			if ev.Name == name {
				found <- ev.ListenAddress
				return
			}
		}
	}()

	evts := c.observer.Endpoints()
	for _, ep := range evts {
		if ep.Name == name {
			select {
			case found <- ep.ListenAddress:

			default:
				// An endpoint is already found, ignore
			}
			break
		}
	}
	<-found
}

func (c *clusterClient) Endpoints() []funk.Endpoint {
	return c.observer.Endpoints()
}
