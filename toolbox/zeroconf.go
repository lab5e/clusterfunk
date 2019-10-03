package toolbox

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grandcat/zeroconf"
)

// This is a zeroconf setup for the cluster. It will register the Serf endpoints
// in mDNS on startup which makes it easier to add new nodes ad hoc.
//
// The library have a few log droppings but that is only in the default logger
// so it will be silenced by regular log filtering.
//
// Also this won't work for Kubernetes or AWS/GCP/Azure since they have no
// support for UDP broadcasts.
//
// The zeroconf code doesn't work on loopback addresses...yet. It uses the
// external IP address of the host when registering.
//

// ZeroconfRegistry is the type for a zeroconf registry. It will announce one
// or more endpoints via mDNS/Zeroconf/Bonjour until Shutdown() is called.
type ZeroconfRegistry struct {
	mutex       *sync.Mutex
	server      *zeroconf.Server
	ClusterName string
}

// This should really be something registered with IANA if we are doing it
// The Proper Way but we'll stick with an unofficial name for now.
const serviceString = "_clusterfunk._udp"

// This is the domain we'll use when announcing the service
const defaultDomain = "local."

var txtRecords = []string{"txtv=0", "name=clusterfunk cluster node"}

// NewZeroconfRegistry creates a new zeroconf server
func NewZeroconfRegistry(clusterName string) *ZeroconfRegistry {
	return &ZeroconfRegistry{
		mutex:       &sync.Mutex{},
		ClusterName: clusterName,
	}
}

// Register registers a new endpoint. Only one endpoint can be created at a time
func (zr *ZeroconfRegistry) Register(name string, port int) error {
	zr.mutex.Lock()
	defer zr.mutex.Unlock()
	if zr.server != nil {
		return errors.New("endpoint already registered")
	}
	var err error

	zr.server, err = zeroconf.Register(fmt.Sprintf("%s_%s", zr.ClusterName, name), serviceString, defaultDomain, port, txtRecords, nil)
	if err != nil {
		return err
	}

	return nil

}

// Shutdown shuts down the Zeroconf server.
func (zr *ZeroconfRegistry) Shutdown() {
	zr.mutex.Lock()
	defer zr.mutex.Unlock()
	if zr.server != nil {
		zr.server.Shutdown()
		zr.server = nil
	}
}

// Resolve looks for another service
func (zr *ZeroconfRegistry) Resolve(waitTime time.Duration) ([]string, error) {

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return nil, err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(entries chan *zeroconf.ServiceEntry) {
		ctx, cancel := context.WithTimeout(context.Background(), waitTime)
		defer cancel()
		err = resolver.Browse(ctx, serviceString, defaultDomain, entries)
		if err != nil {
			close(entries)
			return
		}
		<-ctx.Done()
	}(entries)

	var ret []string
	clusterPrefix := fmt.Sprintf("%s_", zr.ClusterName)
	for entry := range entries {
		if entry.Service == serviceString {
			if strings.HasPrefix(entry.Instance, clusterPrefix) {
				for i := range entry.AddrIPv4 {
					ret = append(ret, fmt.Sprintf("%s:%d", entry.AddrIPv4[i], entry.Port))
				}
			}
		}
	}
	return ret, nil
}

// ResolveFirst looks for another service and returns only the first matching element
func (zr *ZeroconfRegistry) ResolveFirst(waitTime time.Duration) (string, error) {

	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return "", err
	}

	entries := make(chan *zeroconf.ServiceEntry)
	go func(entries chan *zeroconf.ServiceEntry) {
		ctx, cancel := context.WithTimeout(context.Background(), waitTime)
		defer cancel()
		err = resolver.Browse(ctx, serviceString, defaultDomain, entries)
		if err != nil {
			close(entries)
			return
		}
		<-ctx.Done()
	}(entries)

	clusterPrefix := fmt.Sprintf("%s_", zr.ClusterName)
	for {
		select {
		case entry := <-entries:
			if entry.Service == serviceString {
				if strings.HasPrefix(entry.Instance, clusterPrefix) {
					for i := range entry.AddrIPv4 {
						return fmt.Sprintf("%s:%d", entry.AddrIPv4[i], entry.Port), nil
					}
				}
			}
		case <-time.After(waitTime):
			return "", errors.New("timed out")
		}
	}
}
