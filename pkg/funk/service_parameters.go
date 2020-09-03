package funk

import (
	"fmt"

	"github.com/lab5e/clusterfunk/pkg/toolbox"
)

// ServiceParameters is configuration parameters for service nodes, ie nodes that
// aren't part of the cluster but provides discoverable services to the cluster
// (and cluster clients)
type ServiceParameters struct {
	Name     string         `kong:"help='Cluster name',default='clusterfunk'"`
	NodeID   string         `kong:"help='Node ID for Serf node',default=''"`
	Verbose  bool           `kong:"help='Verbose logging for Serf and Raft'"`
	ZeroConf bool           `kong:"help='Zero-conf startup',default='true'"`
	Serf     SerfParameters `kong:"embed,prefix='serf-'"`
}

// Final assigns default values to the fields that have defaults if they're not
// asigned yet (like the NodeID)
func (s *ServiceParameters) Final() {
	if s.NodeID == "" {
		s.NodeID = fmt.Sprintf("svc_%s", toolbox.RandomID())
	}
	s.Serf.Final()
}
