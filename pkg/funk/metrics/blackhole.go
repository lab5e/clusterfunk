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

// NewBlackHoleSink creates a metrics sink that discards all metrics
func NewBlackHoleSink() Sink {
	return &blackHoleSink{}
}

type blackHoleSink struct {
}

func (b *blackHoleSink) SetClusterSize(nodeid string, size int) {
	// do nothing
}

func (b *blackHoleSink) SetShardCount(nodeid string, shards int) {
	// do nothing
}

func (b *blackHoleSink) SetShardIndex(nodeid string, index uint64) {
	// do nothing
}

func (b *blackHoleSink) LogRequest(nodeid, destination, method string) {
	// do nothing
}
