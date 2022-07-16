raft 算法

1. 描述一下raft算法
   1.1每个server有三个状态：
   Leader,follower,candidate
   leader处理所有的请求，这一点是原论文的实现。etcd的实现中，使用了 Linearizable Read来使得follower也可以读。zk的是实现中也可以follow读，但是不保证一致性。

   1.2每个server有一个term
   应该是一个正整数。

   ​	1.2.1 如果一个server的当前term小于别的server,它会立即更新自己的term。
   ​					如果是leader或者candidate发现他的term小于别的，除了更新自己的term,还会立即同步到follow的状态。如果一个server发现收到的term是过时的，它会拒绝请求。

   1.3 使用RPC来进行沟通。
   	1.3.1 RequestVote RPC投票,将由candidates在投票的时候初始化 

   ​	1.3.2 AppendEntries RPC 由leader初始化，用来同步日志和心跳 

   ​	1.3.3 Thrid RPC用来在server之间传递snapshot

   

   1.4 leader 选举

   ​	1.4.1 所有的server开始都是follower状态，直到收到leader或者candidate的合法RPC。leader需要周期性的发送 AppendEntries RPC来维持follower的授权。如果follower超过"election timeout",就会开始选举一个新的leader。

   ​	1.4.2 选举开始的时候，follower先将自己的term加一，然后变成candidate状态。它会投自己一票，然后不停并行发送AppendEntries RPC给其他的server，直到以下情况发生，candidate状态结束：1.4.2.1 它赢了选举 1.4.2.2 其他server赢得了选举 1.4.2.3一个选举最长时间过了，还是没有选举出来。

   ​	1.4.3 下面定义几个要点，使得1.4.2更加完善。1.4.3.1 什么是赢得选举：收到超过1/2的server回应 1.4.3.2 单个server什么时候结束并认可leader：收到了leader的心跳 1.4.3.3 

   ​               a. 在开始选举的时候，每个server的时钟是随机的。 b.在极端情况下，很多节点，运气很好，会出现多个candidate同时发送的情况AppendEntries RPC ，如果都没有收到了足够的选票，会进行重新选举(可以想想如果两个同时发送，没有一个可以得到超过1/2的选票，也就都不能当选)。 

   

   1.4 Log replication
      对于一个复制状态机来说，最重要的东西就是这个了。client的每个请求本质上都是整个复制状态机的运行commond，这些commond必须保证同步，并不可避免的受到CAP的约束。leader首先将commond加入到自己的log里面，然后发送AppendEntries RPC给别的server，只有别的server确认了，server才会将commond加入自己的状态机。否则，server会一直发送AppendEntries RPC(indenfinitely)给别的server。

   ​	注意一下Figure 6,这个定义了log term, 这个我理解为etcd中的index.还提到一个flag,commited, 这个标志了这个commond被确认加入entry.

    下面定义要一下什么是确认了，确认就是大部分follower发送一个确认的RPC给leader.

   ​	定义一下log 和term(这个term就是指的leader term)： 如果两个server有同样index和term，说明1.其余server存储了相同的的commond 2.其余的server有完全相同的entry

   这里有个地方让人困惑：难道有index相同但是term不同的情况吗？有的，leader挂了的时候 

    	1.4.1 描述一下执行一条命令的整个过程：a. leader收到命令，放到log里面，并不放到自己的entry中。b. 发送AppendEntries RPC给其他的server。c. 其他的server收到，放到log里面也不放到entry里面，返回leader收到了。d. leader收到超过半数确认，执行commond, 发送心跳的时候告诉其他节点可以执行了，再执行。

   如果d收到的时候，leader挂了，就会造成，每个节点都有index相同的log,但是由于另一个leader并不会确认这一条commond，所以leader term并不一致。

   d中还有一种可能，就是在leader收到确认，自己提交了，返回客户端可以了。然后没来得及发给别的follower,这时候由于新选出来的leader是会覆盖掉集群大多数未提交的commond，对于集群来说是一致的，但是对于客户端来说可能会拿到错误的确认。这个是raft算法不负责处理的，如果需要处理就需要两阶段提交，就是再等其余servers确认提交了，再返回客户端成功。

   ​	1.4.2 leader具有一致性检查的能力。当leader发给follower一个commond, follower返回拒绝，那么leader就会发送前一个term的前一个index的commond，知道follower返回确认。

      1.4.3 leader会不断重发AppendEntries RPC，直到follower回复了确认最新的term和index, 即使leader已经回复了client。

      1.4.4  上面第一条提到，在leader挂了的时候，是可能存在没有确认的commond的，这个同样存在在那些已经跟挂了的老leader同步的机器上。新leader既可能有也可能没有，但由于没有确认提交，不管有没有，新leader都不会去确认上一个term的commond.这个时候leader会强制老leader(恢复之后成follower)和别的follower覆盖掉自己的没提交commond。由于没有提交，leader也没有向client返回确认，所以是没问题的。所以client 提交失败就是这种情况。

   ​	

   1.5 安全性保证

   ​	这个主要是处理一种corner case, 就是上面提到leader挂了之后，新选出的leader再次挂了，那么第一次挂了的leader是可能出现再次当选的情况的。此时有可能这个挂过的leader是落后整个集群很多的，所以需要在AppendEntries RPC中添加两个字段lasterlogindex和lastlogterm来保证这个leader不会再次当选。

​      1.* 客户端交互行为
​		前面说过对于原始的raft来说，只有leader具有读写功能。客户端一开始会随机选择一个server，当server发现客户端选错了之后，会拒绝，并把最新的leader告诉客户端。

​		如果leader在提交之后，还没有返回client就挂了。client的请求会被新的leader接受，新的leader会根据client请求中的一个序列号返回确认，而不会重复操作。

​			

1. 将算法分解到跟demo一个等级的水平

   2.1 看一遍当前的代码注释

   ```
   // rf = Make(...)
   //   create a new Raft server.
   // rf.Start(command interface{}) (index, term, isleader)
   //   start agreement on a new log entry
   // rf.GetState() (term, isLeader)
   //   ask a Raft for its current term, and whether it thinks it is leader
   ```

   这三个是公开的api

   2.2 大致看一下实现的指标

   2.3 可以先实现leader选举功能
   
   

   

   





