# TODOs


* Proper FSM and log

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

* Make sure bootstrapping a cluster with the same names as an existing one
  returns an error.

* Integrate liveness checker into Raft (needs to handle nodes going away then coming back)
  Checker only handles nodes dying not coming back from the dead

* Consistent SerfNode/RaftNode interfaces
