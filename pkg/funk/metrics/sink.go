package metrics

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

// Sink is the metrics sink for the cluster. Implement this interface to write
// to other kinds of systems.
type Sink interface {
	SetClusterSize(nodeid string, size int)
	SetShardCount(nodeid string, shards int)
	SetShardIndex(nodeid string, index uint64)
	LogRequest(nodeid, destination, method string)
}

// The list of supported metrics
const (
	PrometheusSink = "prometheus"
	NoSink         = "none"
)

// NewSinkFromString returns a named sink
func NewSinkFromString(name string) Sink {
	switch name {
	case "prometheus":
		return NewPrometheusSink()
	default:
		return NewBlackHoleSink()
	}
}
