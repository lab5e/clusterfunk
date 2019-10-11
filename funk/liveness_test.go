package funk

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/stalehd/clusterfunk/toolbox"
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
	time.Sleep(interval)
	checker := NewLivenessChecker(interval, retries)

	mutex := &sync.Mutex{}
	deads := make([]string, 0)
	go func(ch <-chan string) {
		for k := range ch {
			mutex.Lock()
			t.Logf("%s died", k)
			deads = append(deads, k)
			mutex.Unlock()
		}
	}(checker.DeadEvents())

	mutex.Lock()
	assert.Len(deads, 0)
	mutex.Unlock()

	t.Log("Server's up")
	checker.Add("A", ep1)
	checker.Add("B", ep2)
	checker.Add("C", ep3)

	time.Sleep(retries * interval)

	mutex.Lock()
	assert.Len(deads, 0)
	mutex.Unlock()

	localA.Stop()

	time.Sleep((retries + 1) * interval)

	mutex.Lock()
	assert.Len(deads, 1)
	assert.Contains(deads, "A")
	mutex.Unlock()

	checker.Remove("B")
	localB.Stop()

	localC.Stop()

	time.Sleep((retries + 1) * interval)

	mutex.Lock()
	assert.Len(deads, 2)
	assert.Contains(deads, "A")
	assert.Contains(deads, "C")
	mutex.Unlock()

	checker.Clear()
}

// Ensure clients that are DOA are detected
func TestLiveCheckerDeadOnArrival(t *testing.T) {
	const interval = 10 * time.Millisecond
	const retries = 3

	ep1 := toolbox.RandomLocalEndpoint()

	checker := NewLivenessChecker(interval, retries)
	checker.Add("A", ep1)
	time.Sleep(interval)
	start := time.Now()

	time.Sleep(interval * 4)
	for k := range checker.DeadEvents() {
		if k == "A" {
			break
		}
		if time.Now().Sub(start) > (interval * (retries + 1)) {
			t.Fatal("Didn't detect DOA client")
		}
	}
}

func TestLiveCheckerPerformance(t *testing.T) {
	assert := require.New(t)

	const interval = 10 * time.Millisecond
	const retries = 3

	ep1 := toolbox.RandomLocalEndpoint()

	localA := NewLivenessClient(ep1)
	time.Sleep(interval)
	checker := NewLivenessChecker(interval, retries)
	checker.Add("A", ep1)
	start := time.Now()
	var stop time.Time
	localA.Stop()
	for k := range checker.DeadEvents() {
		if k == "A" {
			stop = time.Now()
			break
		}
	}
	// With 3 retries and 10 ms interval the client should fail after
	// 30 ms (+/- 1ms for good measure)
	assert.Less(int64(stop.Sub(start)), int64(31*time.Millisecond))
}
