package clientfunk

import (
	"time"

	"github.com/stalehd/clusterfunk/pkg/funk"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
)

// ZeroconfManagementLookup does a lookup in Zeroconf to find endpoints. The cluster
// nodes must have Zeroconf enabled for this to work
func ZeroconfManagementLookup(clusterName string) (string, error) {
	zr := toolbox.NewZeroconfRegistry(clusterName)
	return zr.ResolveFirst(funk.ZeroconfManagementKind, 1*time.Second)
}
