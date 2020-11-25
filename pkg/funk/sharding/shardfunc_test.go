package sharding

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
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

const numTests = 100000
const numShards = 50

func TestIntegerSharding(t *testing.T) {
	assert := require.New(t)

	calc := NewIntSharder(numShards)
	distribution := make(map[int]int)
	for i := 0; i < numTests; i++ {
		shardNo := calc(rand.Int31())
		distribution[shardNo]++
	}
	assert.LessOrEqual(len(distribution), numShards, "Expected a maximum of %d shards but got %d", numShards, len(distribution))

	for k, v := range distribution {
		assert.LessOrEqual(v, 4*(numTests/numShards), "Shard on %d is unbalanced with %d elements (typical = %d, max = %d)", k, v, numTests/numShards, 4*numTests/numShards)
	}
}

func BenchmarkIntegerSharding(b *testing.B) {
	calc := NewIntSharder(numShards)
	for i := 0; i < b.N; i++ {
		calc(i)
	}
}

func TestStringSharding(t *testing.T) {
	calc := NewStringSharder(numShards)
	distribution := make(map[int]int)

	for i := 0; i < numTests; i++ {
		randstr := make([]byte, 128)
		for p := 0; p < len(randstr); p++ {
			randstr[p] = byte(rand.Int31n(64) + '@')
		}
		shardNo := calc(randstr)
		distribution[shardNo]++
	}
	if len(distribution) > numShards {
		t.Fatalf("Expected a maximum of %d shards but got %d", numShards, len(distribution))
	}
	for k, v := range distribution {
		t.Logf("Shard %-4d: %d", k, v)
		if v > 4*(numTests/numShards) {
			t.Fatalf("Shard on %d is unbalanced with %d elements (typical = %d, max = %d)", k, v, numTests/numShards, 4*numTests/numShards)
		}
	}
}

func BenchmarkStringSharding(b *testing.B) {
	tests := make([]string, b.N)
	for i := 0; i < b.N; i++ {
		tests[i] = fmt.Sprintf("%08x", rand.Int63())
	}
	b.ResetTimer()
	calc := NewStringSharder(numShards)
	for i := 0; i < b.N; i++ {
		calc(tests[i])
	}
}
