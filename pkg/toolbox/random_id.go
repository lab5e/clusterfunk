package toolbox
//
//Copyright 2019 Telenor Digital AS
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//http://www.apache.org/licenses/LICENSE-2.0
//
//Unless required by applicable law or agreed to in writing, software
//distributed under the License is distributed on an "AS IS" BASIS,
//WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//See the License for the specific language governing permissions and
//limitations under the License.
//
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
