package cluster

import "testing"

func TestFindIPAddress(t *testing.T) {
	addr, err := FindPublicIPv4()
	t.Logf("Found address: %v", addr.String())
	if err != nil {
		t.Fatal(err)
	}
}

func TestNodeID(t *testing.T) {
	if randomID() == "" {
		t.Fatal()
	}
}
