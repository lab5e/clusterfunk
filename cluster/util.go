package cluster

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	prand "math/rand"
)

// randomID returns a random ID
func randomID() string {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		// fall back to pseudorandom value
		return fmt.Sprintf("%08x", prand.Int31())
	}
	return fmt.Sprintf("%08x", binary.BigEndian.Uint64(bytes))
}
