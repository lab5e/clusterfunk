# Shard distribution

The initial distribution with a single node (in red) holds all shards on the same node:

![Single node](img/a_01node.png)

A second node (in green) will get half of the shards from the first node:

![Two nodes](img/a_02node.png)

The third node (in blue) will get half of its shards from each of the two original nodes:

![Three nodes](img/a_03node.png)

A fourth node will get a third of its shards from the other three nodes:

![Four nodes](img/a_04node.png)

Finally a fifth and sixth node will get an equal distribution of shards:

![Five nodes](img/a_05node.png) ![Six nodes](img/a_06node.png)

When reducing the number of nodes the existing nodes will get the shards from the node that left.

![Five nodes](img/b_05node.png)
![Four nodes](img/b_04node.png)
![Three nodes](img/b_03node.png)
![Two nodes](img/b_02node.png)
![One node](img/b_01node.png)
