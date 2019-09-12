package utils

import "testing"

func TestFindIPAddress(t *testing.T) {
	addr, err := FindPublicIPv4()
	t.Logf("Found address: %v", addr.String())
	if err != nil {
		t.Fatal(err)
	}
}

func TestFindLoopback(t *testing.T) {
	FindLoopbackIPv4Interface()
}
