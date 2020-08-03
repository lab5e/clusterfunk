package http

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
	"net/http"
	"os"

	"github.com/lab5e/clusterfunk/pkg/funk"
	"github.com/lab5e/clusterfunk/pkg/funk/sharding"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// ConsoleEndpointName is the name of the HTTP endpoint in the cluster
const ConsoleEndpointName = "ep.httpConsole"

// MetricsEndpointName is the name of the metrics endpoint for each node
const MetricsEndpointName = "ep.metrics"

const uiPath = "./cmd/demo/server/html"

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
		log.Info("Using external HTML assets")
		mux.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(uiPath))))
	} else {
		log.WithError(err).Info("Using embedded HTML assets")
		mux.Handle("/", http.StripPrefix("/", http.FileServer(Assets)))
	}
	mux.HandleFunc("/statusws", websocketHandler)
	log.WithField("endpoint", endpoint).Info("HTTP server started")

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
