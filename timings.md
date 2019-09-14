
# Note on timing (and performance)

All timings are on an i7 processor @ 3 GHz. YMMV. Typical size for the inital
clusters are determined by how many Raft can handle. Anything above 9 nodes
yields nervous ticks for the authors but there are ways around this.
Test data set is 10k shards on 50 nodes. Everything runs on the same host for
cluster tests; ie the RTT for the network is practically 0. For AWS we can expect
RTT in the range of 1-3ms (or up to 10ms - no official numbers from AWS on this)

| Where | What | Time | Comments
|-------|------|-----:|---------
| Network | RTT for packets | 3ms | Depends on region and instance type
| Raft | Replicating logs | 1.5ms | Measured by using ApplyLog. Typical is lower but worst case is around 1.5ms
| Raft | Leader election | 1500ms | Time from entering candidate state to leader or follower
| Raft | Detecting a failed node | - | This can be tuned by adjusting heart beats. Higher rates yields better results but could cause unstable clusters/nodes
| Lib | Redistributing shards in memory| 200us | This is just the processing time
| Lib | Encoding shards for the log | 2ms | This can be sped up by encoding manually
| Lib | Decoding shards from the log | 3ms | This can also be sped up by manual decoding into a byte buffer
| Lib | Shard -> Node lookup | 80ns | This is a simple array lookup
| Lib | Initialize new shard manager | 450us | Used when leader starts up
| Lib | Computing shard from integer key | 27ns | Single op + array lookup
| Lib | Computing shard from string key | 108ns | Hash + array lookup

## Time for leader election

This time varies with the configured timeouts. The defaults are quite liberal (or it does looks like it) so if they are tuned down the time span will be shorter.

These are the defaults:

```golang
HeartbeatTimeout:   1000 * time.Millisecond,
ElectionTimeout:    1000 * time.Millisecond,
CommitTimeout:      50 * time.Millisecond,
SnapshotInterval:   120 * time.Second,
LeaderLeaseTimeout: 500 * time.Millisecond,

```

Scaling down the parameters by 10 yields ten times better results... No surprises there. We'd have to test and see what the figures are.

Decreasing the leader lease to 50ms makes the detection quicker and we can expect something in the region of 200ms to detect and resolve a leader that drops out. Instances dropping out will behave similarly; one missed heartbeat plus replication time.
