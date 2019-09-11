package cluster

import "testing"

func TestNodeID(t *testing.T) {
	if randomID() == "" {
		t.Fatal()
	}
}
