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

