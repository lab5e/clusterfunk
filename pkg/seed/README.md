# Seed node

If you're running in environments without ZeroConf/mDNS support (like docker,
AWS, CGP or Azure) you can't rely on auto-discovery of the Serf nodes. The
simplest solution is to create one or more seed nodes that exists solely to seed
new members of the Serf cluster. From there on you can do discovery on Raft
(aka "real cluster nodes"), associated services and so on. To make your
configuration simpler you can use DNS for the seed nodes.

Since availability isn't a big issue - if a restart takes a few seconds it
just means that new nodes won't be able to join the Serf cluster through the
usual means; you can - in a pinch - use one of the other nodes to join the
cluster.

This is a simple seed node that dumps the nodes and endpoints in the Serf
cluster so it can be used as a diagnostic tool as well.

