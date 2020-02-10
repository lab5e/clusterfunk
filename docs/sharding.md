# About sharding

Sharding isn't very magical. It's basically just a way to divide your requests into chunks that can be distributed across the nodes in the cluster. This does require *some* thinking when determining which shard function to use:

* Will you mutate values in the request? In that case you want to make sure the shard function maps to one and only one node and that it's the same node every time. If the shard function maps to different nodes requests can cross in flight and you will get weird results from your service. Unless you have a way to tell "this is version N" of each entity and "please change entity with version N into this" for updates you can get stale data
* Is it just a lookup that doesn't depend on any previous requests? A round robin scheme with random assignments works fine.

In general you want the following properties for your shard functions:

* It can be calculated on a single node without doing any lookups
* It should be fast. The shard function will be used for every single request and every micro-second (or worse: milli-) you spend executing the shard function is added on top of every single request.

One of the simplest (if not **the** simplest) shard functions are the modulo operator. If you need 10 000 shards you can just calculate the shard from the identifier directly:

```golang
func idToShard(id int64) int {
    return id % numberOfShards
}
```

This implies a few key properties of the identifier:

* It's uniformly distributed. If your identifiers are unevenly distributed or clustered around certain ranges it won't work as expected.
* It's an integer.

The latter isn't a big problem since floating point numbers can be easily converted. The distribution requirement still applies though. If all of your identifers are in the range 0.0-1.0 you *definitively* don't want to use a floor function to convert your identifiers.

Strings are also easy to convert into integers - just use a hashing function like CRC64, SHA1 or MD5. Computing these are quite quick. CRC64 can be calculated in 100 - 150ns for a 16 character string.

## Shards

The shards are represented as simple integers and the shard mapping function can be expressed as `f(request) = n` where `n` is the shard. The shard map is an array of assignments to the nodes in the cluster. The index of the array is the shard identifier. A (very simple) shard map for the nodes A, B, C with nine shards looks like the following:

```golang
shardMap := [
    "A",
    "B",
    "C",
    "A",
    "B",
    "C",
    "A",
    "B",
    "C",
]
```

The shard is implicitly defined by the index of the array so

* Node A is responsible for shard 0, 3 and 6
* Node B is responsible for shard 1, 4 and 7
* Node C is responsible for shard 2, 5 and 8
