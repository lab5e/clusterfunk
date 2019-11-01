package main

import (
	"time"

	log "github.com/sirupsen/logrus"
)

// The event producer is used to distribute the events to the websockets
type eventProducer interface {
	SetPresets(msg []interface{})
	Presets() []interface{}
	Send(msg interface{})
	Messages() <-chan interface{}
	Done(<-chan interface{})
}

func newEventProducer() eventProducer {
	return &msgSender{}
}

type msgSender struct {
	presets []interface{}
	chans   []chan interface{}
}

func (m *msgSender) SetPresets(presets []interface{}) {
	m.presets = presets[:]
}

func (m *msgSender) Presets() []interface{} {
	return m.presets
}

func (m *msgSender) Send(msg interface{}) {
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
	for i, v := range m.chans {
		if v == ch {
			m.chans = append(m.chans[:i], m.chans[i+1:]...)
		}
	}
}
