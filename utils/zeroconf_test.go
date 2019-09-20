package utils

import (
	"testing"
	"time"
)

func TestZeroConf(t *testing.T) {

	zr := NewZeroconfRegistry("test-cluster")
	if err := zr.Register("some-node", 9999); err != nil {
		t.Fatal(err)
	}

	if err := zr.Register("another-node", 9998); err == nil {
		t.Fatal("Should not be able to register two endpoints")
	}
	defer zr.Shutdown()

	results, err := zr.Resolve(250 * time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("No client found")
	}
}
