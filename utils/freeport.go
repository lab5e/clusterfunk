package utils

import (
	"net"
)

// FreeTCPPort returns a free TCP port by using net.ListenTCP on port :0
func FreeTCPPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", ":0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// FreeUDPPort returns a free UDP port by using net.ListenUDP on port :0
func FreeUDPPort() (int, error) {
	addr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenUDP("udp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.LocalAddr().(*net.UDPAddr).Port, nil
}
