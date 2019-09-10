package cluster

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"time"

	"github.com/hashicorp/mdns"
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
var server *mdns.Server

const advertisingString = "_horde._udp"

// zeroconfRegister registers a Serf endpoint on the local network.
func zeroconfRegister(endpoint string) error {
	info := []string{"Horde Cluster"}
	host, portStr, err := net.SplitHostPort(endpoint)
	if err != nil {
		return err
	}
	port, err := strconv.ParseInt(portStr, 10, 32)
	if err != nil {
		return err
	}
	svc, err := mdns.NewMDNSService(host, advertisingString, "", "", int(port), nil, info)
	if err != nil {
		return err
	}
	server, err = mdns.NewServer(&mdns.Config{Zone: svc})
	if err != nil {
		return err
	}

	return nil
}

// zeroconfLookup does a mDNS query to find Serf nodes on the local
// network.
func zeroconfLookup(localEndpoint string) (string, error) {

	found := make(chan string)
	defer close(found)
	for i := 0; i < 3; i++ {
		entries := make(chan *mdns.ServiceEntry, 4)
		go func(ch chan *mdns.ServiceEntry) {
			for entry := range ch {
				serfEP := fmt.Sprintf("%s:%d", entry.AddrV4.String(), entry.Port)
				if serfEP != localEndpoint {
					log.Printf("Found host: %s", serfEP)
					found <- serfEP
				}
			}
		}(entries)
		mdns.Lookup(advertisingString, entries)
		select {
		case h := <-found:
			return h, nil
		case <-time.After(5 * time.Second):
			log.Printf("Retrying query (%d)", i+1)
		}
	}
	return "", errors.New("No hosts found")
}

// zeroconfShutdown shuts down the running mDNS server. This isn't thread safe.
func zeroconfShutdown() {
	if server != nil {
		server.Shutdown()
	}
}
