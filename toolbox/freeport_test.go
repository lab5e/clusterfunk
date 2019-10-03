package toolbox

import "testing"

func TestFreeTCPPort(t *testing.T) {
	if p, err := FreeTCPPort(); err != nil || p == 0 {
		t.Fatal()
	}
}

func TestFreeUDPPort(t *testing.T) {
	if p, err := FreeUDPPort(); err != nil || p == 0 {
		t.Fatal()
	}
}
