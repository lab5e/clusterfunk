package http

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// The event producer is used to distribute the events to the websockets.There
// might be more than one client connected to a websocket so we'll have to
// dispatch the events to more than one channel.
//
// If one of the clients stops listening the channel will block and we close and
// drop the channel.
//

type eventProducer interface {
	SetPresets(msg []interface{})
	Presets() []interface{}
	Send(msg interface{})
	Messages() <-chan interface{}
	Done(<-chan interface{})
}

func newEventProducer() eventProducer {
	return &msgSender{mutex: &sync.Mutex{}}
}

type msgSender struct {
	mutex   *sync.Mutex
	presets []interface{}
	chans   []chan interface{}
}

func (m *msgSender) SetPresets(presets []interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.presets = presets[:]
}

func (m *msgSender) Presets() []interface{} {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.presets[:]
}

func (m *msgSender) Send(msg interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, v := range m.chans {
		select {
		case v <- msg:
			// ok - keep sending
		case <-time.After(10 * time.Millisecond):
			// drop the channel
			log.Info("Closing socket since it timed out")
			close(v)
			m.chans = append(m.chans[:i], m.chans[i+1:]...)
		}
	}
}

func (m *msgSender) Messages() <-chan interface{} {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	newCh := make(chan interface{})
	m.chans = append(m.chans, newCh)
	go func() {
		for _, v := range m.presets {
			newCh <- v
		}
	}()
	return newCh
}

func (m *msgSender) Done(ch <-chan interface{}) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	for i, v := range m.chans {
		if v == ch {
			m.chans = append(m.chans[:i], m.chans[i+1:]...)
		}
	}
}
