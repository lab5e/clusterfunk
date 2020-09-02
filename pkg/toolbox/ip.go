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
	"fmt"
	"net"
	"strconv"

	"github.com/lab5e/gotoolbox/netutils"
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
	// Will ignore the error here since we want to return 0 if there's an error
	ret, _ := strconv.ParseInt(port, 10, 32)
	return int(ret)
}

// RandomPublicEndpoint returns a random public endpoint on the host. It will use the first IPv4 address found on the host.
func RandomPublicEndpoint() string {
	port, err := netutils.FreeTCPPort()
	if err != nil {
		panic(err)
	}
	ip, err := FindPublicIPv4()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}

// RandomLocalEndpoint returns a random endpoint on the loppback interface.
func RandomLocalEndpoint() string {
	port, err := netutils.FreeTCPPort()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("127.0.0.1:%d", port)
}
