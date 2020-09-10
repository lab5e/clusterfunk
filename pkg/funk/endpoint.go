package funk

import (
	"fmt"
	"net"

	"github.com/lab5e/gotoolbox/netutils"
)

// Endpoint stores information on endpoints
type Endpoint struct {
	NodeID        string // NodeID is the ID of the node that registered the endpoint
	ListenAddress string // ListenAddress is the registered address for the endpoint
	Name          string // Name is the name of the endpoint
	Active        bool   // Active is set to true if the endpoint is from an active node
	Local         bool   // Local is set to true if this is on the local node
	Cluster       bool   // Cluster is set to true if the node is a member of the cluster
}

// ToPublicEndpoint converts an ip:port string into a format suitable for
// publishing as endpoints. If the listen address is ":1288" or "0.0.0.0:1288"
// it should be specified as an address reachable for external clients, i.e.
// "[public ip]:1288".
func ToPublicEndpoint(listenHostPort string) (string, error) {
	host, port, err := net.SplitHostPort(listenHostPort)
	if err != nil {
		return listenHostPort, err
	}

	if host == "" || host == "0.0.0.0" {
		// use the public IP
		ip, err := netutils.FindPublicIPv4()
		if err != nil {
			return listenHostPort, err
		}
		return fmt.Sprintf("%s:%s", ip.String(), port), nil
	}
	return listenHostPort, nil
}
