![Clusterfunk](img/cf_logo_280x50.png)

# Clustering library

## Run the demo

You probably want to see it in action before doing anything else so here we go:

Build it by running `make`, then launch the demo server in a terminal window. The first node must be bootstrapped so use the `--bootstrap` parameter:

```shell
[home ~]$ bin/demo --bootstrap
```

Launch other nodes (three or five should suffice. The maximum number is somewhere around 15 but more on that later) in new terminals. These nodes will autodiscover the first node via Zeroconf and Serf so you won't need the bootstrap parameter:

```shell
[home ~]$ bin/demo
```

You should see the first node picking up the nodes as they start up and if you look in the log you'll see a HTTP endpoint. Point your browser to it and you should see a status page.

## Features

* Zeroconf cluster discovery
* Simple endpoint discovery via Serf
* Server-side proxying of requests to other cluster nodes
* Client-side connection pooling
* Metrics to measure cluster health
* Quick failover (typically less than 1 second when a node fails)

## Limitations

A typical cluster of Raft nodes should not exceed 10-15 nodes. This is a limitation in the Raft protocol (and library).

## What it is

## What it isn't

## How it works

## How to use it

Simple service walkthrough.

## The demo client

Describe demo client.

## Using clusterfunk without Zeroconf

Describe how to run without zeroconf. Alternate strategies for Docker/K8s, AWS, GCP and Azure.

## Server side

Describe server side library, how to implement services. What's included and what's not (`serverfunk`)

## Client side

Describe client side library (`clientfunk`)
## Performance

Describe expected performance

## Interfaces

All ports are by default assigned to a random (free) port. Ports can be overriden to static assignments if required.

| Port | Protocol | Description
| ---- | -------- | -----------
| random | UDP+TCP | Serf
| random | TCP | Raft
| random | TCP | gRPC management RPC
| random | TCP | gRPC internal cluster RPC
| random | UDP | Liveness checking

## Autodiscovery caveats

Describe limitations wrt autodiscover clusters (and why it's a really bad idea to use it in a production setting).