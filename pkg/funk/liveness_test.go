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

	"github.com/stalehd/clusterfunk/pkg/toolbox"
)

// Ensure clients gets events when they die while acking
func TestLiveness(t *testing.T) {
	const interval = 10 * time.Millisecond
	const retries = 3
	assert := require.New(t)

	ep1 := toolbox.RandomLocalEndpoint()
	ep2 := toolbox.RandomLocalEndpoint()
	ep3 := toolbox.RandomLocalEndpoint()

	localA := NewLivenessClient(ep1)
	localB := NewLivenessClient(ep2)
	localC := NewLivenessClient(ep3)
	/*	defer localA.Stop()
		defer localB.Stop()
		defer localC.Stop()*/
	time.Sleep(interval)

	checker := NewLivenessChecker(interval, retries)
	checker.Add("A", ep1)
	checker.Add("B", ep2)
	checker.Add("C", ep3)

	timeout := false
	for !timeout {
		select {
		case <-checker.AliveEvents():
			assert.FailNow("Should not receive an alive event")
		case <-checker.DeadEvents():
			assert.FailNow("Should not receive a dead event")
		case <-time.After(interval * 5):
			timeout = true
		}
	}
	localA.Stop()
	foundA := false
	for !foundA {
		select {
		case id := <-checker.DeadEvents():
			assert.Equal("A", id, "Expected A to fail")
			foundA = true
		case <-time.After(interval * 15):
			assert.FailNow("Timed out waiting for dead message")
		}
	}
	localB.Stop()
	localC.Stop()

	foundB := false
	foundC := false

	for !foundB && !foundC {
		select {
		case id := <-checker.DeadEvents():
			switch id {
			case "B":
				foundB = true
			case "C":
				foundC = true
			case "A":
				assert.FailNow("A should not die again")
			}
		case <-time.After(interval * 15):
			assert.FailNow("Timed out waiting for dead messages")
		}
	}
	checker.Remove("B")
	checker.Remove("C")

	// Bring back A
	localA = NewLivenessClient(ep1)
	for !foundA {
		select {
		case id := <-checker.AliveEvents():
			assert.Equal("A", id, "Expected A to fail")
			foundA = true
		case <-time.After(interval * 45):
			assert.FailNow("Timed out waiting for alive message")
		}
	}
	localA.Stop()
	checker.Clear()
	//defer checker.Shutdown()
}
