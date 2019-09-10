package cluster

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	prand "math/rand"
	"net"
)

// FindPublicIPv4 returns the public IPv4 address of the. If there's more than
// one public IP(v4) address the first found is returned.
// Ideally this should use IPv6 but we're currently running in AWS and IPv6
// support is so-so.
//
func FindPublicIPv4() (net.IP, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	for _, addr := range addrs {
		switch a := addr.(type) {
		case *net.IPNet:
			if ipv4 := a.IP.To4(); ipv4 != nil && !ipv4.IsLoopback() {
				return a.IP, nil
			}
		}
	}
	return nil, errors.New("no ipv4 address found")
}

// randomID returns a random ID
func randomID() string {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		// fall back to pseudorandom value
		return fmt.Sprintf("%08x", prand.Int31())
	}
	return fmt.Sprintf("%08x", binary.BigEndian.Uint64(bytes))
}
