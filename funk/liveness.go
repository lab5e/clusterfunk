package funk

import (
	"net"
	"time"

	log "github.com/sirupsen/logrus"
)

// LocalLivenessEndpoint launches a local liveness client. The client will respond
// to (short) UDP packets by echoing back the packet on the same port.
// This is not a *health* check, just a liveness check if the node is
// reachable on the network.
type LocalLivenessEndpoint interface {
	Stop()
}

type udpLivenessClient struct {
	stopCh chan bool
}

// NewLivenessClient creates a new liveness endpoint with the specified
// port/address. Response is sent immideately
func NewLivenessClient(ep string) LocalLivenessEndpoint {
	ret := &udpLivenessClient{stopCh: make(chan bool)}
	ret.launch(ep)
	return ret
}

// Stop will cause the liveness client to stop
func (u *udpLivenessClient) Stop() {
	u.stopCh <- true
}

func (u *udpLivenessClient) launch(endpoint string) {
	conn, err := net.ListenPacket("udp", endpoint)
	if err != nil {
		log.WithField("endpoint", endpoint).WithError(err).Error("unable to create local connection")
		return
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
			conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			_, addr, err := conn.ReadFrom(buf)
			if err != nil {
				continue
			}
			conn.WriteTo(buf, addr)
		}
	}(conn)
}

// LivenessChecker is a liveness checker. It does a (very simple) high freqyency
// liveness check on nodes. If it fails more than a certain number of times an
// event is generated on the DeadEvents channel. Only a single subscriber is
// supported for this channel.
type LivenessChecker interface {
	// Add adds a new new check to the list.
	Add(id string, endpoint string)
	// Clear removes all checks
	Clear()
	// DeadEvents returns the event channel. The ID from the Add method is
	// echoed on this channel when a client stops responding.
	DeadEvents() <-chan string
}

// This type checks a single client for liveness.
type singleChecker struct {
	deadCh    chan string
	stopCh    chan struct{}
	maxErrors int
}

func newSingleChecker(id, endpoint string, deadCh chan string, interval time.Duration, retries int) singleChecker {
	ret := singleChecker{
		deadCh:    deadCh,
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

	errors := 0
	var conn net.Conn
	for {
		if errors >= c.maxErrors {
			c.deadCh <- id
			return
		}
		if conn == nil {
			conn = c.getConnection(endpoint, interval)
			if conn == nil {
				c.deadCh <- id
				return
			}
			defer conn.Close()
		}

		waitCh := time.After(interval)

		conn.SetWriteDeadline(time.Now().Add(interval))
		_, err := conn.Write([]byte("Yo"))
		if err != nil {
			// Writes will usually succeed since UDP is a black hole but
			// this *might* fail.
			errors++
			conn = nil
			continue
		}
		conn.SetReadDeadline(time.Now().Add(interval))
		_, err = conn.Read(buffer)
		if err != nil {
			errors++
			conn = nil
			continue
		}

		select {
		case <-waitCh:
			// keep checking
		case <-c.stopCh:
			// stop checking
			return
		}
	}
}

func (c *singleChecker) Stop() {
	select {
	case c.stopCh <- struct{}{}:
	default:
		// it's already stopping

	}
}

// this is the liveness checker type that implements the LivenessChecker
// interface.
type livenessChecker struct {
	checkers map[string]singleChecker
	events   chan string
	retries  int
	interval time.Duration
}

// NewLivenessChecker is a type that checks hosts for liveness
func NewLivenessChecker(interval time.Duration, retries int) LivenessChecker {
	return &livenessChecker{
		interval: interval,
		retries:  retries,
		checkers: make(map[string]singleChecker),
		events:   make(chan string, 10),
	}
}

func (l *livenessChecker) Clear() {
	for _, v := range l.checkers {
		v.Stop()
	}
}

func (l *livenessChecker) Add(id string, endpoint string) {
	l.checkers[id] = newSingleChecker(id, endpoint, l.events, l.interval, l.retries)
}

func (l *livenessChecker) DeadEvents() <-chan string {
	return l.events
}
