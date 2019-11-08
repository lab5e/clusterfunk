package http

import (
	"net/http"

	"github.com/stalehd/clusterfunk/pkg/funk"
	"github.com/stalehd/clusterfunk/pkg/funk/sharding"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
)

// ConsoleEndpoint is the name of the HTTP endpoint in the cluster
const ConsoleEndpoint = "ep.httpConsole"

const uiPath = "./cmd/demo/server/html"

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

var messageProducer eventProducer
var presets []interface{}

const (
	nodeInfoPreset = iota
	memberListPreset
	shardMapPreset
	clusterStatusPreset
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
func StartWebserver(endpoint string, cluster funk.Cluster, shards sharding.ShardManager) {
	messageProducer = newEventProducer()
	presets = make([]interface{}, lastPreset)

	presets := make([]interface{}, 4)
	presets[nodeInfoPreset] = newNodeInfo(cluster)
	presets[memberListPreset] = newMemberList(cluster)
	presets[shardMapPreset] = newShardMap(shards)
	presets[clusterStatusPreset] = newClusterStatus(cluster)
	messageProducer.SetPresets(presets)

	mux := http.NewServeMux()
	mux.Handle("/", http.StripPrefix("/", http.FileServer(http.Dir(uiPath))))
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
func ClusterOperational(cluster funk.Cluster, shards sharding.ShardManager) {
	// update member list and shards
	shardMap := newShardMap(shards)
	memberList := newMemberList(cluster)
	presets[memberListPreset] = memberList
	presets[shardMapPreset] = shardMap
	presets[clusterStatusPreset] = newClusterStatus(cluster)
	messageProducer.SetPresets(presets)
	messageProducer.Send(shardMap)
	messageProducer.Send(memberList)
}
