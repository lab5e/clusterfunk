# TODOs

* Rewrite client pool to block when getting a new client. Also rename
  from "EndpointPool" to "ConnectionPool" since it isn't endpoint that
  are pooled.

* Non-members of the cluster, ie nodes that are part of the
  Serf cluster but not the Raft cluster. Useful for other kinds
  of services.

* Turn off auto-join/leave for Serf in production clusters

* DNS option for Serf nodes. Since Zeroconf doesn't work for AWS/GCP
  having one or more nodes registered in DNS makes sense. Might need
  some additional machinery on the outside.

* Open source the parameters library (flags is cumbersome)

* Complete demo with work spread across nodes

* There's no smooth way to discover the internals from the outside. Make gRPC
  interface on all nodes for discovery. Serf client is a bit clunky and creates
  lots of failed/left Serf nodes in the cluster. Hashicorp has their own RPC
  for each node. Might be an idea to implement a similar scheme (or just add to
  the existing management gRPC since it's already running on all nodes. Make
  utility functions to discover. Outside discovery might not be within the scope.
  Management function w/ gRPC might also be used for this.

* Make sure bootstrapping a cluster with the same names as an existing one
  returns an error.

* Consistent SerfNode/RaftNode interfaces

* Quarantine nodes that doesn't answer reshards. Add timer for responses, if no response has been sent in x ms quarantine the node as "unresponsive" and do a new sharding round.

* Run go test -race to ensure there's no race conditions (if possible -- there
  might be libraries that have issues)