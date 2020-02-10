# Demo server

There is a sample demo server. It uses a very simple gRPC service that basically responds to all requests with a random definition from The Meaning of Liff.

When launched with all the defaults the demo server can be started with `bin/demo`. It will use the zeroconf feature to discover nodes already running on the network and join the cluster it can find.

The command line parameters are as follows. There are sensible defaults for all parameters.

    Usage: server

    Demo server

    Flags:
      --help                                   Show context-sensitive help.
      --cpu-profiler-file=STRING               Turn on profiling and store the profile data in a file
      --cluster-auto-join                      Auto join via SerfEvents
      --cluster-name="clusterfunk"             Cluster name
      --cluster-interface=STRING               Interface address for services
      --cluster-verbose                        Verbose logging for Serf and Raft
      --cluster-node-id=STRING                 Node ID for Serf and Raft
      --cluster-zero-conf                      Zero-conf startup
      --cluster-non-voting                     Nonvoting node
      --cluster-non-member                     Non-member
      --cluster-liveness-interval=150ms        Liveness checker intervals
      --cluster-liveness-retries=3             Number of retries for liveness checks
      --cluster-liveness-endpoint=STRING       Liveness UDP endpoint
      --cluster-ack-timeout=500ms              Ack timeout for nodes in the cluster
      --cluster-metrics="prometheus"           Metrics sink to use
      --cluster-raft-raft-endpoint=STRING      Endpoint for Raft
      --cluster-raft-disk-store                Disk-based store
      --cluster-raft-bootstrap                 Bootstrap a new Raft cluster
      --cluster-raft-verbose                   Verbose Raft logging
      --cluster-raft-debug-log                 Show debug log messages for Raft
      --cluster-serf-endpoint=STRING           Endpoint for Serf
      --cluster-serf-join-address=STRING       Join address and port for Serf cluster
      --cluster-serf-verbose                   Verbose logging for Serf
      --cluster-management-endpoint=STRING     Server endpoint
      --cluster-management-tls                 Enable TLS
      --cluster-management-cert-file=STRING    Certificate file
      --cluster-management-key-file=STRING     Certificate key file
      --cluster-leader-endpoint=STRING

Once you've launched a single server you can check out its status by pointing your browser to the HTTP endpoint. Look up the endpoint by running the `ctrlc` tool. The port for the http server is selected at random:

    $ bin/ctrlc endpoints http
    Node ID              Name                 Endpoint
    a825ad3470a73dbd     ep.httpConsole       192.168.1.47:54146

    Reporting node: a825ad3470a73dbd

Here the endpoint is at `192.168.1.47:54146` so if we open `http://192.168.1.47:54146` in our browser we'll see the status page. It's quite boring with just one node so let's launch another one. Open another terminal window and launch a new demo server. It should discover the other node and join the cluster automatically. Repeat this a couple of times until there's four or five nodes in the cluster.

The status page should refresh when the new nodes join.

Run the demo client to send some requests to the cluster. If you use the defaults for the client it will send 50 requests in five seconds, then exit:

    $ bin/client
    [==============================================================================]
        437b1dbc81ef35f4: 16 items min:  0.359  max:  4.371  mean:  1.115  stddev:  0.957
        a825ad3470a73dbd: 10 items min:  0.547  max:  2.406  mean:  1.131  stddev:  0.633
        1ba9640647262fe3: 14 items min:  0.404  max:  3.187  mean:  1.011  stddev:  0.708
        23daeb9c29c202f4: 10 items min:  0.440  max:  2.059  mean:  1.078  stddev:  0.602
    50 in total, 0 retries, 0 with errors
     9.734 reqs/sec on average

The requests should be distributed across all nodes roughly equally and the status page should update to reflect the requests. The min, max mean and stddev measurements are in milliseconds.

Use the `--repeats` command line parameter to increase the number of requests the client generates. This will make the client run for a longer time (the rate will be fixed to about 10 requests per second). If you pick 1000 or 10000 the client will run for a bit longer.

While the client is running you can terminate one of the nodes, then start another one. If you terminate the node that shows the status page it will stop updating so pick one of the other nodes. The number of requests handled by the cluster should briefly drop to zero while it is removing then adding a new node but client should not show any error messages:

    $ bin/client --repeats=10000
    ==============================================================================]
       1ba9640647262fe3: 2591 items min:  0.209  max: 783.294  mean:  1.131  stddev: 15.396
       a825ad3470a73dbd: 921 items min:  0.285  max: 373.312  mean:  1.219  stddev: 12.304
       23daeb9c29c202f4: 2531 items min:  0.214  max: 1003.286  mean:  1.256  stddev: 20.158
       437b1dbc81ef35f4: 23 items min:  0.333  max:  3.685  mean:  0.958  stddev:  0.676
       e563b2ecbe0e28e7: 2572 items min:  0.202  max: 804.452  mean:  1.253  stddev: 16.040
       8d9e5719a963a355: 1362 items min:  0.399  max: 12.188  mean:  0.963  stddev:  0.805
    0000 in total, 3 retries, 0 with errors
    9.724 reqs/sec on average

Let's try to remove the leader from the cluster. Run the client again if it isn't running and go to the status page. Click on one of the links at the bottom to go to one of the other nodes' status page. Terminate the leader node. One of the other nodes should assume leadership, reshard the cluster and it should be up and running.

Experiment a bit with the `--repeat` and `--sleep` parameters on the client to send more or less requests to the cluster. The sleep parameter is the time the client sleeps between requests, Set it to `1ms` to send approximately 1000 requests per second. You can launch multiple clients at once.

If you launch a node on your local network that runs on another computer you should see it pick up requests just like the nodes you are running on your local computer.

Note: The nodes will need a quorum to process requests. If you terminate the leader in a three-node cluster it will not have enough nodes to elect a leader. New nodes in the cluster must be added by the leader so the cluster won't be able to recover. Production clusters should not add or remove nodes automatically.
