# TODOs

* Node state handling. The nodes have semi-defined states when starting and
  shutting down. See several points below.

* Delay publishing endpoints for the Raft node until it's joined the cluster, just
  publish the ep.raft endpoint. Maybe create a new endpoint named ep.raft-notjoin or similar?

* Clean up endpoint handling. It's a mess. Use "endpoint" as a type when operating
  on endpoints.

* More diagnostics.

* DNS option for Serf nodes. Since Zeroconf doesn't work for AWS/GCP
  having one or more nodes registered in DNS makes sense. Might need
  some additional machinery on the outside. Resolve DNS and grab the first
  available.

* Quarantine nodes that doesn't answer reshards. Add timer for responses, if no
  response has been sent in x ms quarantine the node as "unresponsive" and do
  a new sharding round.

* Prettify demo console

* Docker stack for demo server

* Proper drain for nodes. Draining is mostly pure luck at the moment but they
  should be in state draining when a reshard happens.

* Commisioning and decommissioning of nodes. Flag as "to be retired" or similar.
  Add/remove node then trigger a reshard manually. States should be "Retiring",
  "Inactive".

* Serve read only requests while resharding. This requires some care when
  implementing the actual service so it's not at the top of the list but
  if a reshard/read operation is slow it makes sense to serve read only
  requests while new data is read into memory. A partial working service always
  (well, most of the time) trumps a not-working service.

* Read only state for nodes could be useful but requires custom implementation.
