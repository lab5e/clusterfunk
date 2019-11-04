# TODOs

* Non-members of the cluster, ie nodes that are part of the
  Serf cluster but not the Raft cluster. Useful for other kinds
  of services.

* Turn off auto-join/leave for Serf in production clusters

* DNS option for Serf nodes. Since Zeroconf doesn't work for AWS/GCP
  having one or more nodes registered in DNS makes sense. Might need
  some additional machinery on the outside.

* Open source the parameters library (flags is cumbersome)

* Complete demo with work spread across nodes

* Consistent SerfNode/RaftNode interfaces

* Quarantine nodes that doesn't answer reshards. Add timer for responses, if no
  response has been sent in x ms quarantine the node as "unresponsive" and do
  a new sharding round.

* Prettify demo console

* Docker stack for demo server

* Metrics for proxying (preferrably not tied to any particular metrics library) and cluster.

* Client connection pooling code
