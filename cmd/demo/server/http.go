package main

import (
	"net/http"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"github.com/stalehd/clusterfunk/pkg/toolbox"
)

const uiPath = "./cmd/demo/server/html"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

type producer interface {
	Presets(msg []interface{})
	Send(msg interface{})
	Messages() <-chan interface{}
}

func newProducer() producer {
	return &msgSender{}
}

var messageProducer producer

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("Unable to upgrade connection")
		return
	}
	defer conn.Close()
	for msg := range messageProducer.Messages() {
		if err := conn.WriteJSON(msg); err != nil {
			log.WithError(err).Error(err)
			return
		}
	}
}

// Start a very simple websocket handler. It can only handle a single client at a time
func launchLocalWebserver(p producer) {
	messageProducer = p
	mux := http.NewServeMux()
	hostport := toolbox.RandomLocalEndpoint()
	mux.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(uiPath))))
	mux.HandleFunc("/statusws", websocketHandler)
	log.WithField("endpoint", hostport).Info("HTTP server started")

	srv := &http.Server{
		Addr:    hostport,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			panic(err.Error())
		}
	}()
}

type msgSender struct {
	presets []interface{}
	chans   []chan interface{}
}

func (m *msgSender) Presets(presets []interface{}) {
	m.presets = presets[:]
}

func (m *msgSender) Send(msg interface{}) {
	for i, v := range m.chans {
		select {
		case v <- msg:
			// ok - keep sending
		default:
			// drop the channel
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
