
# Note on timing (and performance)

All timings are on an i7 processor @ 3 GHz. YMMV. Typical size for the inital
clusters are determined by how many Raft can handle. Anything above 9 nodes
yields nervous ticks for the authors but there are ways around this.
Test data set is 10k shards on 50 nodes. Everything runs on the same host for
cluster tests; ie the RTT for the network is practically 0. For AWS we can expect
RTT in the range of 1-2ms.

| Where | What | Time | Comments
|-------|------|-----:|---------
| Network | RTT for packets | 1.5ms | Depends on region and instance type
| Raft | Replicating logs | 1.5ms | Measured by using ApplyLog
| Raft | Leader election | - | Time from entering candidate state to leader or follower
| Raft | Detecting a failed node | - | This can be tuned by adjusting heart beats. Higher rates yields better results but could cause unstable clusters/nodes
| Lib | Redistributing shards in memory| 200us | This is just the processing time
| Lib | Encoding shards for the log | 2ms | This can be sped up by encoding manually
| Lib | Decoding shards from the log | 3ms | This can also be sped up by manual decoding into a byte buffer
| Lib | Shard -> Node lookup | 80ns | This is a simple array lookup
| Lib | Initialize new shard manager | 450us | Used when leader starts up
| Lib | Computing shard from integer key | 27ns | Single op + array lookup
| Lib | Computing shard from string key | 108ns | Hash + array lookup

