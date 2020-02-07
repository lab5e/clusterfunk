# Run the demo

You probably want to see it in action before doing anything else so here we go:

Build it by running `make`, then launch the demo server in a terminal window. The first node must be bootstrapped so use the `--bootstrap` parameter:

```shell
[home ~]$ bin/demo --bootstrap
```

Launch other nodes (three or five should suffice. The maximum number is somewhere around 15 but more on that later) in new terminals. These nodes will autodiscover the first node via Zeroconf and Serf so you won't need the bootstrap parameter:

```shell
[home ~]$ bin/demo
```

You should see the first node picking up the nodes as they start up and if you look in the log you'll see a HTTP endpoint. Point your browser to it and you should see a status page.
