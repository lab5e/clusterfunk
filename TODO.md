# TODOs

* DNS option for Serf nodes. Since Zeroconf doesn't work for AWS/GCP
  having one or more nodes registered in DNS makes sense. Might need
  some additional machinery on the outside.

* Quarantine nodes that doesn't answer reshards. Add timer for responses, if no
  response has been sent in x ms quarantine the node as "unresponsive" and do
  a new sharding round.

* Prettify demo console

* Docker stack for demo server

