package toolbox

import "testing"

func TestNodeID(t *testing.T) {
	if RandomID() == "" {
		t.Fatal()
	}
}
