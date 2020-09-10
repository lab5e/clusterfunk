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
	"fmt"
	"net"
	"time"

	"github.com/sirupsen/logrus"
)

// LivenessChecker is a liveness checker. It does a (very simple) high freqyency
// liveness check on nodes. If it fails more than a certain number of times an
// event is generated on the DeadEvents channel. Only a single subscriber is
// supported for this channel.
type LivenessChecker interface {
	// Add adds a new new check to the list.
	Add(id string, endpoint string)

	// Remove removes a single checker
	Remove(id string)

	// DeadEvents returns the event channel. The ID from the Add method is
	// echoed on this channel when a client stops responding.
	DeadEvents() <-chan string

	// AliveEvents returns an event channel for alive events.
	AliveEvents() <-chan string

	// Clear removes all endpoint checks.
	Clear()

	// Shutdown shuts down the checker and closes the event channel. The
	// checker will no longer be in an usable state after this.
	Shutdown()
}

// LocalLivenessEndpoint launches a local liveness client. The client will respond
// to (short) UDP packets by echoing back the packet on the same port.
// This is not a *health* check, just a liveness check if the node is
// reachable on the network.
type LocalLivenessEndpoint interface {
	Stop()
}

type udpLivenessClient struct {
	stopCh chan struct{}
}

// NewLivenessClient creates a new liveness endpoint with the specified
// port/address. Response is sent immideately
func NewLivenessClient(ep string) LocalLivenessEndpoint {
	ret := &udpLivenessClient{stopCh: make(chan struct{})}
	ret.launch(ep)
	return ret
}

// Stop will cause the liveness client to stop
func (u *udpLivenessClient) Stop() {
	u.stopCh <- struct{}{}
}

func (u *udpLivenessClient) launch(endpoint string) {
	conn, err := net.ListenPacket("udp", endpoint)
	if err != nil {
		// This is panic-worthy material since the other members of the cluster might think that
		// this node is dead
		panic(fmt.Sprintf("liveness: unable to listen on %s: %v", endpoint, err))
	}
	go func(conn net.PacketConn) {
		buf := make([]byte, 2)
		defer conn.Close()
		for {
			select {
			case <-u.stopCh:
				return
			default:
			}
			if err := conn.SetReadDeadline(time.Now().Add(1 * time.Second)); err != nil {
				logrus.WithError(err).Warning("Can't set deadline for socket")
				time.Sleep(1 * time.Second)
				continue
			}
			_, addr, err := conn.ReadFrom(buf)
			if err != nil {
				continue
			}
			// nolint This *might* cause an error but we'll ignore the erorr since it's handled elsewhere.
			conn.WriteTo(buf, addr)
		}
	}(conn)
}

// This type checks a single client for liveness.
type singleChecker struct {
	deadCh    chan<- string
	aliveCh   chan<- string
	stopCh    chan struct{}
	maxErrors int
}

func newSingleChecker(id, endpoint string, deadCh chan<- string, aliveCh chan<- string, interval time.Duration, retries int) singleChecker {
	ret := singleChecker{
		deadCh:    deadCh,
		aliveCh:   aliveCh,
		stopCh:    make(chan struct{}),
		maxErrors: retries,
	}
	go ret.checkerProc(id, endpoint, interval)
	return ret
}

func (c *singleChecker) getConnection(endpoint string, interval time.Duration) net.Conn {
	for i := 0; i < c.maxErrors; i++ {
		d := net.Dialer{Timeout: interval}
		conn, err := d.Dial("udp", endpoint)
		if err != nil {
			// Usually this wil work just fine, it's the write that will fail
			// but you can never be too sure.
			time.Sleep(interval)
			continue
		}
		return conn
	}
	return nil
}

func (c *singleChecker) checkerProc(id, endpoint string, interval time.Duration) {
	buffer := make([]byte, 2)

	// The error counter counts up and down - when it reaches the limit (aka maxErrors)
	// the client is set to either alive (with negative errors, ie success) or dead
	// (ie errors is positive)
	errors := 0
	var conn net.Conn
	alive := true

	// The waiting interval is bumped up and down depending on the state. If the
	// client is alive it is set to the check interval and when it is dead the
	// check interval is 10 times higher.
	waitInterval := interval

	for {
		select {
		case <-c.stopCh:
			logrus.Infof("Terminating leveness checker to %s", endpoint)
			return
		default:
			// keep on running
		}
		if alive && errors >= c.maxErrors {
			alive = false
			c.deadCh <- id
			waitInterval = 10 * interval
		}
		if !alive && errors <= 0 {
			alive = true
			c.aliveCh <- id
			waitInterval = interval
		}

		if conn == nil {
			conn = c.getConnection(endpoint, waitInterval)
			if conn == nil {
				errors++
				time.Sleep(waitInterval)
				continue
			}
		}

		waitCh := time.After(waitInterval)

		if err := conn.SetWriteDeadline(time.Now().Add(interval)); err != nil {
			logrus.WithError(err).Warning("Can't set deadline for liveness check socket")
			conn.Close()
			conn = nil
			time.Sleep(waitInterval)
			continue
		}
		_, err := conn.Write([]byte("Yo"))
		if err != nil {
			// Writes will usually succeed since UDP is a black hole but
			// this *might* fail. Close the connection and reopen.
			errors++
			conn.Close()
			conn = nil
			time.Sleep(waitInterval)
			continue
		}
		if err := conn.SetReadDeadline(time.Now().Add(interval)); err != nil {
			logrus.WithError(err).Warning("Can't set deadline for liveness check socket")
			conn.Close()
			conn = nil
			time.Sleep(waitInterval)
			continue
		}

		_, err = conn.Read(buffer)
		if err != nil {
			// This will fail if the client is dead.
			errors++
			conn.Close()
			conn = nil
			time.Sleep(waitInterval)
			continue
		}
		errors--
		if errors > c.maxErrors {
			errors = c.maxErrors
		}
		if errors < -c.maxErrors {
			errors = -c.maxErrors
		}
		<-waitCh
	}
}

func (c *singleChecker) Stop() {
	select {
	case c.stopCh <- struct{}{}:
	case <-time.After(10 * time.Millisecond):
		// it's already stopping
	}
}

// this is the liveness checker type that implements the LivenessChecker
// interface.
type livenessChecker struct {
	checkers    map[string]singleChecker
	deadEvents  chan string
	aliveEvents chan string
	retries     int
	interval    time.Duration
}

// NewLivenessChecker is a type that checks hosts for liveness
func NewLivenessChecker(interval time.Duration, retries int) LivenessChecker {
	return &livenessChecker{
		interval:    interval,
		retries:     retries,
		checkers:    make(map[string]singleChecker),
		deadEvents:  make(chan string, 10),
		aliveEvents: make(chan string, 10),
	}
}

func (l *livenessChecker) Add(id string, endpoint string) {
	l.checkers[id] = newSingleChecker(id, endpoint, l.deadEvents, l.aliveEvents, l.interval, l.retries)
}

func (l *livenessChecker) Remove(id string) {
	existing, ok := l.checkers[id]
	if ok {
		existing.Stop()
		delete(l.checkers, id)
	}
}

func (l *livenessChecker) DeadEvents() <-chan string {
	return l.deadEvents
}

func (l *livenessChecker) AliveEvents() <-chan string {
	return l.aliveEvents
}

func (l *livenessChecker) Clear() {
	for k, v := range l.checkers {
		v.Stop()
		delete(l.checkers, k)
	}
}

func (l *livenessChecker) Shutdown() {
	l.Clear()
	close(l.deadEvents)
	close(l.aliveEvents)
}
