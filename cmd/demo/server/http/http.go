package http

import (
	"embed"
	"fmt"
	"net/http"
	"os"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/sharding"
	"github.com/sirupsen/logrus"

	"github.com/gorilla/websocket"
)

// ConsoleEndpointName is the name of the HTTP endpoint in the cluster
const ConsoleEndpointName = "ep.httpConsole"

// MetricsEndpointName is the name of the metrics endpoint for each node
const MetricsEndpointName = "ep.metrics"

const uiPath = "./cmd/demo/server/http"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var messageProducer eventProducer
var presets []interface{}

const (
	memberListPreset = iota
	clusterStatusPreset
	shardMapPreset
	lastPreset
)

func websocketHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logrus.WithError(err).Error("Unable to upgrade connection")
		return
	}
	defer conn.Close()

	// Send the preset events from the producer first
	for _, msg := range messageProducer.Presets() {
		if err := conn.WriteJSON(msg); err != nil {
			logrus.WithError(err).Error(err)
			return
		}
	}

	p := messageProducer.Messages()
	defer messageProducer.Done(p)
	// ... then forward updated events
	for msg := range p {
		if err := conn.WriteJSON(msg); err != nil {
			logrus.WithError(err).Error("error writing message")
			return
		}
	}
}

//go:embed index.html
//go:embed images/*
//go:embed js/*
//go:embed css/*
var assets embed.FS

// StartWebserver starts the web server that hosts the status page.
func StartWebserver(endpoint string, cluster funk.Cluster, shards sharding.ShardMap) {
	messageProducer = newEventProducer()
	presets = make([]interface{}, lastPreset)

	presets := make([]interface{}, 4)
	presets[memberListPreset] = newMemberList(cluster)
	presets[shardMapPreset] = newShardMap(shards)
	presets[clusterStatusPreset] = newClusterStatus(cluster)
	messageProducer.SetPresets(presets)

	mux := http.NewServeMux()
	// Serve locally if the file system is found, otherwise use the
	// included assets.
	_, err := os.Lstat(fmt.Sprintf("%s/index.html", uiPath))
	if err == nil {
		logrus.Info("Using external HTML assets")
		mux.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(uiPath))))
	} else {
		logrus.WithError(err).Info("Using embedded HTML assets")
		mux.Handle("/", http.FileServer(http.FS(assets)))
	}
	mux.HandleFunc("/statusws", websocketHandler)
	logrus.WithField("endpoint", endpoint).Info("HTTP server started")

	srv := &http.Server{
		Addr:    endpoint,
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			panic(err.Error())
		}
	}()
}

// UpdateClusterStatus updates the cluster status
func UpdateClusterStatus(cluster funk.Cluster) {
	messageProducer.Send(newClusterStatus(cluster))
}

// ClusterOperational sends notification to the connected HTTP clients
// TODO: naming. It makes zero sense.
func ClusterOperational(cluster funk.Cluster, shards sharding.ShardMap) {
	// update member list and shards
	shardMap := newShardMap(shards)
	memberList := newMemberList(cluster)
	presets[memberListPreset] = memberList
	presets[shardMapPreset] = shardMap
	presets[clusterStatusPreset] = newClusterStatus(cluster)
	messageProducer.SetPresets(presets)
	messageProducer.Send(memberList)
	messageProducer.Send(shardMap)
}
