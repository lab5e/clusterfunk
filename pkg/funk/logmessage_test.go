package funk
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
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLogMessage(t *testing.T) {
	assert := require.New(t)

	const testData = "hello there"
	const endpoint = "1.2.3.4:1234"
	lm := NewLogMessage(ProposedShardMap, endpoint, []byte(testData))
	assert.Equal(lm.MessageType, ProposedShardMap)
	assert.Equal(endpoint, lm.AckEndpoint)
	assert.Len(lm.Data, len(testData), "Length is %d, not %d", len(lm.Data), len(testData))

	buf, err := lm.MarshalBinary()
	assert.NoError(err)

	lm2 := NewLogMessage(0, "", nil)
	assert.NoError(lm2.UnmarshalBinary(buf))

	assert.Equal(lm2.MessageType, ProposedShardMap)
	assert.Equal(lm2.AckEndpoint, lm.AckEndpoint)
	assert.Len(lm2.Data, len(testData), "Length of lm2 is %d, not %d", len(lm2.Data), len(testData))
	assert.Equal(testData, string(lm2.Data))
}
