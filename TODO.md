# TODOs

* mutex when accessing the Raft instance (panics are not great when shutting down)
* Proper FSM and log
* Replicate logs with SQLite
* Turn off auto-join/leave for Serf in production clusters
* DNS option for Serf nodes. Since Zeroconf doesn't work for AWS/GCP
having one or more nodes registered in DNS makes sense. Might need
some additional machinery on the outside.
* Make utility functions for common operations (discover zeroconf, create
serf client etc)
* Open source the parameters library (flags is cumbersome)
* Complete demo with work spread across nodes
* gRPC for leader tasks (redistribute shards, node join et al)
