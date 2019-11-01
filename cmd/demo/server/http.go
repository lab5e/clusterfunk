package main

import (
	"net/http"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

const httpConsoleEndpoint = "ep.httpConsole"

const uiPath = "./cmd/demo/server/html"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var messageProducer eventProducer

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.WithError(err).Error("Unable to upgrade connection")
		return
	}
	defer conn.Close()

	// Send the preset events from the producer first
	for _, msg := range messageProducer.Presets() {
		if err := conn.WriteJSON(msg); err != nil {
			log.WithError(err).Error(err)
			return
		}
	}

	p := messageProducer.Messages()
	defer messageProducer.Done(p)
	// ... then forward updated events
	for msg := range p {
		if err := conn.WriteJSON(msg); err != nil {
			log.WithError(err).Error("error writing message")
			return
		}
	}
}

// Start a very simple websocket handler. It can only handle a single client at a time
func launchLocalWebserver(p eventProducer, hostport string) {
	messageProducer = p
	mux := http.NewServeMux()
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
