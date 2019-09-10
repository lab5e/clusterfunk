package cluster

import (
	"testing"
)

func TestZeroConf(t *testing.T) {
	if err := zeroconfRegister("127.0.0.1:1234"); err != nil {
		t.Fatal(err)
	}
	defer zeroconfShutdown()

	str, err := zeroconfLookup("127.0.0.1:4321")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Found address: %v", str)
}
