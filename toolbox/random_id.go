package toolbox

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	prand "math/rand"
)

// RandomID returns a random ID
func RandomID() string {
	bytes := make([]byte, 8)
	_, err := rand.Read(bytes)
	if err != nil {
		// fall back to pseudorandom value
		return fmt.Sprintf("%08x", prand.Int31())
	}
	return fmt.Sprintf("%08x", binary.BigEndian.Uint64(bytes))
}
