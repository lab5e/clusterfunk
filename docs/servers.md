# Implementing a server

Each endpoint that handles requests in the cluster must be capable of proxying requests to the other nodes. The demo server uses gRPC and the library itself includes some gRPC interceptors that will take care of all the nitty gritty details of proxying but if you are using a different client prototcol you'll have to implement the proxying code yourself. The algorithm (in go-like pseudocode) is quite simple:

```golang
func proxying(request RequestObject) (ResponsObject, eror) {
    shard := shardFunction(request)
    nodeID := shardMap(shard).NodeID

    if nodeID == LocalNodeID {
        return local(request)
    }

    return remote(nodeID, request)
}
```

Error handling is up to the client. If the proxying call fails the client must retry the request.

If you are using a gRPC client (it's highly recommended for all your RPC needs) you can use the interceptors declared in the `serverfunk` package when you create your server object. You'll need a function to extract the shard from the request and a list of connections to each node in the cluster:

```golang

func createServer(cluster funk.Cluster, shards sharding.ShardMap) {

    // This is the endpoint name
    endpointName := "ep.myrpcendpoint"

    // Create the gRPC server object
    myServer := newGRPCServer()

    // The shard function. This creates a simple shard function with values in
    // the range 0...len(#shards)
    shardFunc := sharding.NewIntSharder(int64(len(shardMap.Shards)))

    // This is a function that extracts the shard from the request by applying
    // the shard function to the identifier in the request
    conversionFunc := func(request interface{}) (int, interface{}) {
        // Handle a conversion for each request/response pair in your server
		switch v := request.(type) {
		case *request1:
            return shardFunc(v.ID), &response1{}
        case *request2:
            return shardFunc(v.ID), &response2{}

            // ....
        }
		panic(fmt.Sprintf("Unknown request type %T", request))
	}
    // This is the list of proxy connections to the cluster. This list will
    // update automatically when there's a change in the cluster.
    proxyClients := serverfunk.NewProxyConnections(endpointName, shards, cluster)

    // Create the server itself with the interceptor
    server := grpc.NewServer(serverfunk.WithClusterFunk(cluster,
		conversionFunc, proxyClients, metrics.PrometheusSink)...)
}

```

There is some boilerplate code that must be added but the (local) implementation of the gRPC server will work as is. The interceptor takes care of the routing of requests.
