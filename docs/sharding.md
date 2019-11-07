![Clusterfunk](img/cf_bullet_50x50.png)

# Sharding

Describe underlying assumptions. How sharding works. Single-node mutations. Proxying.

## Sharding functions

There's two different sharding functions provided with the library, one for strings and one for integers. Both works very similar and uses a simple modulus to split the shards between the nodes.

## Weighting

Describe shard weights, how to implement. Also describe changes in shard weights over time. Communicating changes to the server.

## Considerations when designing sharding functions

Describe what to consider when using sharding functions.

