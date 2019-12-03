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
	"time"

	"github.com/stretchr/testify/require"
)

func TestAckCollectionHappyPath(t *testing.T) {
	assert := require.New(t)

	c := newAckCollection()
	assert.NotNil(c)

	c.StartAck([]string{"A", "B", "C"}, 1, 100*time.Millisecond)
	defer c.Done()

	c.Ack("A", 1)
	c.Ack("B", 1)
	c.Ack("C", 1)

	select {
	case <-c.Completed():
		// success
	case <-c.MissingAck():
		assert.Fail("Should not miss ack")
	case <-time.After(500 * time.Millisecond):
		assert.Fail("One of the channels should contain a response")
	}
}

func TestAckCollectionMissingAck(t *testing.T) {
	assert := require.New(t)

	c := newAckCollection()
	assert.NotNil(c)

	c.StartAck([]string{"A", "B", "C"}, 1, 100*time.Millisecond)
	defer c.Done()

	c.Ack("A", 1)
	c.Ack("B", 2)
	c.Ack("C", 1)

	select {
	case <-c.Completed():
		assert.Fail("Should not be complete")
	case <-c.MissingAck():
		// success
	case <-time.After(500 * time.Millisecond):
		assert.Fail("One of the channels should contain a response")
	}
}
