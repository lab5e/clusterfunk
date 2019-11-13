![Clusterfunk](img/cf_bullet_50x50.png)

# Production clusters

TBD - how to run clusterfunk cluster in production. Recommended settings and so on

Static configuration, on disk storage, fixed ports on relaunch and so on.

Sample launch parameters

```shell
bin/demo \
--cluster-raft-disk-storage \
--cluster-node-id=NodeA \
--cluster-auto-join=false \
--cluster-raft-endpoint=:12002 \
--cluster-liveness-endpoint=:13001 \
--cluster-management-endpoint=:14001
```

```shell
bin/demo \
--cluster-raft-disk-storage \
--cluster-node-id=NodeB \
--cluster-auto-join=false \
--cluster-raft-endpoint=:12002 \
--cluster-liveness-endpoint=:13002 \
--cluster-management-endpoint=:14002
```

```shell
bin/demo \
--cluster-raft-disk-storage \
--cluster-node-id=NodeC \
--cluster-auto-join=false \
--cluster-raft-endpoint=:12003 \
--cluster-liveness-endpoint=:13003 \
--cluster-management-endpoint=:14003
```
