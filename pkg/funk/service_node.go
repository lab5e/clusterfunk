package funk

import (
	"time"

	"github.com/lab5e/clusterfunk/pkg/toolbox"
	"github.com/sirupsen/logrus"
)

// ServiceNode is a node for services that will be discoverable but not a
// member of the cluster. Nonvoting nodes requires a fair bit of capacity in
// the cluster but service nodes are more loosely related and may provide
// services to the cluster itself.
type ServiceNode interface {
	RegisterServiceEndpoint(endpointName string, listenAddress string) error
	Stop()
}

// NewServiceNode creates a new ServiceNode instance
func NewServiceNode(serviceName string, params ServiceParameters) (ServiceNode, error) {
	params.Final()
	ret := &serviceNode{
		Serf: NewSerfNode(),
	}
	if params.ZeroConf {
		// TODO: Merge this with cluster init code. Some similarities
		registry := toolbox.NewZeroconfRegistry(params.Name)
		var err error
		addrs, err := registry.Resolve(ZeroconfSerfKind, 1*time.Second)
		if err != nil {
			return nil, err
		}

		if params.Serf.JoinAddress == "" {
			if len(addrs) > 0 {
				params.Serf.JoinAddress = addrs[0]
			}
			if len(addrs) == 0 {
				logrus.Debug("No serf nodes found via zeroconf, wont join")
			}
		}

		if err := registry.Register(ZeroconfSerfKind, params.NodeID, toolbox.PortOfHostPort(params.Serf.Endpoint)); err != nil {
			return nil, err
		}

	}

	if err := ret.Serf.Start(params.NodeID, serviceName, params.Serf); err != nil {
		return nil, err
	}
	return ret, nil
}

type serviceNode struct {
	Serf *SerfNode
}

func (s *serviceNode) RegisterServiceEndpoint(endpointName string, listenAddress string) error {
	s.Serf.SetTag(endpointName, listenAddress)
	return s.Serf.PublishTags()
}

func (s *serviceNode) Stop() {
	if err := s.Serf.Stop(); err != nil {
		logrus.WithError(err).Error("Error stopping Serf node")
	}
}
