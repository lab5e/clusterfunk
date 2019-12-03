package toolbox
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
	servers     map[string]*zeroconf.Server
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
		servers:     make(map[string]*zeroconf.Server),
		mutex:       &sync.Mutex{},
		ClusterName: clusterName,
	}
}

// Register registers a new endpoint. Only one endpoint can be created at a time
// The ID parameter is an unique ID.
func (zr *ZeroconfRegistry) Register(kind string, id string, port int) error {
	zr.mutex.Lock()
	defer zr.mutex.Unlock()
	var err error
	entry := fmt.Sprintf("%s_%s_%s", zr.ClusterName, kind, id)
	_, ok := zr.servers[entry]
	if ok {
		return errors.New("entry is already registered")
	}
	zr.servers[entry], err = zeroconf.Register(entry, serviceString, defaultDomain, port, txtRecords, nil)
	if err != nil {
		return err
	}
	return nil
}

// Shutdown shuts down the Zeroconf server.
func (zr *ZeroconfRegistry) Shutdown() {
	zr.mutex.Lock()
	defer zr.mutex.Unlock()
	for k, v := range zr.servers {
		v.Shutdown()
		delete(zr.servers, k)
	}
}

// Resolve looks for another service
func (zr *ZeroconfRegistry) Resolve(kind string, waitTime time.Duration) ([]string, error) {

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
	clusterPrefix := fmt.Sprintf("%s_%s", zr.ClusterName, kind)
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
func (zr *ZeroconfRegistry) ResolveFirst(kind string, waitTime time.Duration) (string, error) {

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

	clusterPrefix := fmt.Sprintf("%s_%s", zr.ClusterName, kind)
	for {
		select {
		case entry := <-entries:
			if entry == nil {
				return "", errors.New("no entry returned")
			}
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
