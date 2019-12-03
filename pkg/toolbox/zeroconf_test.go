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
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestZeroConf(t *testing.T) {
	assert := require.New(t)

	zr := NewZeroconfRegistry("test-cluster")
	assert.NoError(zr.Register("some-node", "0", 9999))

	assert.Error(zr.Register("some-node", "0", 9998), "Should not be able to register two endpoints")

	assert.NoError(zr.Register("some-node", "1", 9999))

	defer zr.Shutdown()

	results, err := zr.Resolve("some-node", 550*time.Millisecond)
	assert.NoError(err)
	assert.NotNil(results)

	res, err := zr.ResolveFirst("some-node", 250*time.Millisecond)
	assert.NoError(err)
	ip, err := FindPublicIPv4()
	assert.NoError(err)
	assert.Equal(res, fmt.Sprintf("%s:9999", ip))
}
