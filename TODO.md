# TODOs

* Corner case: Client gets parts of replicated log and latest index points to
  acked shard map (needs two pointers for leader's wanted index and my acked
  shardmap index)

* Make SerfNode more user friendly, keep node state local w/ accessors. Also
  make events more concise. Allow endpoints to be added before Start() in cluster.

* Corner case: Client dies on startup when logs are replicated to it -- Raft does not
  detect a just-joined-and-died client. It's probably relevant for clients with
  persistent storage as well. (PR for Raft is in the works)
* Move raft node management into the raftnode type. Make coalesced events to avoid
  raft spamming.
* Proper FSM and log
* Replicate logs with SQLite
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
  utility functions to discover.
