package toolbox

import (
	"net"
	"strconv"
)

// FindPublicIPv4 returns the public IPv4 address of the. If there's more than
// one public IP(v4) address the first found is returned.
// Ideally this should use IPv6 but we're currently running in AWS and IPv6
// support is so-so.
//
func FindPublicIPv4() (net.IP, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, ifi := range ifaces {
		if (ifi.Flags & net.FlagUp) == 0 {
			continue
		}
		if (ifi.Flags & net.FlagMulticast) > 0 {

			addrs, err := ifi.Addrs()
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
		}
	}
	panic("no ipv4 address found")
}

// FindLoopbackIPv4Interface finds the IPv4 loopback interface. It's usually
// the one with the 127.0.0.1 address but you never know what sort of crazy
// config you can stumble upon.
func FindLoopbackIPv4Interface() net.Interface {
	ifaces, err := net.Interfaces()
	if err != nil {
		panic("Can't get network interfaces")
	}
	for _, ifi := range ifaces {
		if (ifi.Flags & net.FlagUp) == 0 {
			continue
		}
		if (ifi.Flags & net.FlagLoopback) > 0 {
			addrs, err := ifi.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				switch a := addr.(type) {
				case *net.IPNet:
					if ipv4 := a.IP.To4(); ipv4 != nil && ipv4.IsLoopback() {
						return ifi
					}
				}
			}
		}
	}
	panic("no ipv4 loopback adapter found")
}

// PortOfHostPort returns the port number for the host:port string. If there's
// an error it will panic -- use with caution.
func PortOfHostPort(hostport string) int {
	_, port, err := net.SplitHostPort(hostport)
	if err != nil {
		panic(err.Error())
	}
	ret, err := strconv.ParseInt(port, 10, 32)
	return int(ret)
}
