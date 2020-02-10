
# Using Clusterfunk in a production cluster

Note: The library is not ready for production use yet. It has not been exetensively tested in a real world environment so use at your own peril. It **seems** stable but there's always skeletons in the closet.

## You'll need a leader for the cluster to work

The one big thing you have to think about is that the cluster must be able to elect a leader. Without a leader there will be no sharding and without a current shard map there is no requests served. No requests served means your service(s) are broken.

## Think about your cluster size

To ensure you have a quorum you should have an odd number of nodes in the cluster. For a three-node cluster you can loose one node and there will be enough remaining nodes (2) to get a majority vote for one of them, even if the leader is removed.

For a five-node cluster you can loose two nodes and still have enough remaining nodes for a majority vote (3). A seven-node cluster can loose three nodes and still survive.

## Think about the network topology

If you run a redundant setup the likelyhood of a network partition is non-zero. From separate physical hosts to separate racks, rooms or data centers it's conceptually the same. If ever is going to experience networking trouble between nodes it's when one of zones go away or looses connectivitiy to the other zones you have.

Let's use AWS as an example. Most regions have three or more availability zones with low latency between each zone. You would typically put either three nodes (one in each zone), five (2 + 2 + 1), seven (2 + 2 + 3) or nine (3 + 3 + 3) nodes spread across the zones. If one of the zones goes away the nodes in the existing zone won't get a majority vote while the two others together is enough to elect a new leader.

The worst case scenario is an even number of instances spread across two zones. If there's a network partition or a zone goes down you're guaranteed to have issues and a setup with everything in the same zone have less probability of issues.

You *can* split an odd number of nodes across two zones but when one falls out you'll loose a relatively large proportion of the total nodes. The ideal setup (for now) is a nine node cluster in three zones. If you use virtualization of some sort it makes a lot of sense to ensure the nodes are running on different hardware.

## Use persistent logs

Never use in-memory store for the replicated logs. First of all they consume memory but more importantly: You can't recover if there's a network partition or nodes have died unexpectedly. Since the leader is responsible for adding and removing members in the cluster a leader is *required* to change the cluster size. If the leader dies and the remaining nodes won't be able to establish a majority vote (they may also have stale information on how many members the cluster have as well, worsening the situation) the cluster can (and will) end up in limbo unable to elect a leader and at the same time unable to include more nodes in the cluster. The only solution to this problem is to bootstrap the cluster all over.  Relaunching one of the old nodes won't help either since the state is lost. if the log is persisted it is possible to relaunch a node when it goes down.

## Manage nodes manually

It might seem tempting to let the cluster autodetect new nodes and let the leader handle nodes leaving and joining automatically but the cluster won't handle network partitions gracefully, ie if the network connectivity between one of the zones is lost for all the other zones and the leader is in the zone that is partitioned the leader node will adjust the cluster size to whatever nodes it can reach, discard the others and they will happily serve requests even when they are in the minority. The remaining zones (with a majority vote) will elect a new leader, that leader will create a shard map and we will have multiple nodes serving requests to the same shards. In a *best case* scenario you end up with  data inconsistencies for all the requests you've handled and a lot of confused clients.

Cluster sizes changes infrequently and it's not a big deal to add or remove nodes in the cluster, f.e. when you are upgrading the nodes.
