# Networking

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
