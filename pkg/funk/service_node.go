package funk

import (
	"errors"
	"net"
)

// ServiceNode is a node for services that will be discoverable but not a
// member of the cluster. Nonvoting nodes requires a fair bit of capacity in
// the cluster but service nodes are more loosely related and may provide
// services to the cluster itself.
type ServiceNode interface {
	RegisterServiceEndpoint(endpointName string, address net.Addr) error
	Stop()
}

// NewServiceNode creates a new ServiceNode instance
func NewServiceNode() (ServiceNode, error) {
	return nil, errors.New("not implemented")
}
