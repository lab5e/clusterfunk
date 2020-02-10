# Implementing a client

From the client's perspective it is quite simple to talk to the cluster. Since every node proxies the requests to all the other nodes you *could* just find one of the nodes, connect to that and conduct your business as usual. If the node dies or becomes unavailable you can just find another node and repeat. If you put a load balancer in front of your cluster this would work but that brings additional moving parts into your system and you might not want that.

The first issue you'll encounter is "how do i find the nodes?" -- not surprisingly. A gut instinct might be "Ah, easy. Use DNS" but DNS has its downsides: It isn't made to work with sub-second changes. You can set the TTL for DNS entries to 60 seconds but in many cases that's as low as you can go. If one of the nodes goes away the clients will continue to send requests to it for up to a minute afterwards.

The `clientfunk` package can help find nodes. This will

```golang
clientName := "client"
clusterName := "demo"
zeroConf := true
serfConfig := funk.SerfParameters{ /* ... */ }

em, err := clientfunk.StartEndpointMonitor(clientName, clusterName, zeroConf, serfConfig)
if err != nil {
    panic(fmt.Sprintf("Could not start endpoint monitor: %v", err)
}
defer em.Stop()

// This blocks execution until the endpoint monitor listener above have
// discovered endpoints in the Serf cluster. This might take anything from a
// few hundred milliseconds to a second.
em.WaitForEndpoints()
```

This will start an event listener on a Serf cluster that will update the list of endpoints the nodes are advertising.

If you are using gRPC it's all a matter of adding a custom service configuration to your clients. The service configuration specifies that an internal load balancer in gRPC shall be used. To simplify the process further there's a custom resolver in the `clientfunk` package that will resolve custom endpoint names for us:

```golang
grpcConnection, err := grpc.Dial("cluster:///ep.myservice",
    grpc.WithInsecure(),
    grpc.WithDefaultServiceConfig(clientfunk.GRPCServiceConfig))
```

There's quite a lot that happens in the background here but the net effect is that the requests will be spread across the nodes in the cluster via the built-in gRPC mechanisms. The `cluster:///ep.myservice` URL (note that there's *three* slashes and not two. This can cause a lot of headscratching) resolves to the entire set of nodes with that endpoint.

The client connection can in turn be used to create a client instances for gRPC and the calls are nothing out of the ordinary, just regular gRPC calls.

It is worth noting that the client is responsible for retries. If the call fails it can be retried with the same connection. A simple pattern like this lets the client recover from failed nodes with no errors:

```golang
res, err := grpcClient.SomeCall(ctx, req)
if err != nil {
    // ...new request context here...
    res, err = grpcClient.SomeCall(ctx, req)
}
```
