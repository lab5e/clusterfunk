# TODOs
* Corner case: Client dies on startup when logs are replicated to it -- Raft does not
  detect a just-joined-and-died client. It's probably relevant for clients with
  persistent storage as well. (PR for Raft is in the works)
* Proper FSM and log
* Replicate logs with SQLite
* Turn off auto-join/leave for Serf in production clusters
* DNS option for Serf nodes. Since Zeroconf doesn't work for AWS/GCP
  having one or more nodes registered in DNS makes sense. Might need
  some additional machinery on the outside.
* Open source the parameters library (flags is cumbersome)
* Complete demo with work spread across nodes
