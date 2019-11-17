# raftKVDB
MIT 6.824

Optimization:
- DONE
    - Quickly backup over incorrect
    - Try to remove mutex on applyDaemon loop, since there's no data race when read committed log. But it seems useless to improve performance.
    - Speed up Read. Current Read is perceived as the same as PutAppend. However KVserver ID may not be identical to its Raft peer ID per the test setting.
- TODO
    - Find the median of matchIndex using Quick Selection. The running time is linear. Current strategy is to justify one peer's latest commitIdx when receive the AppendEntries Reply. The frequency of updating leader's commit is relatively low for current strategy 
    - Replace commitChannel with a Conditional Variable. Current commitChannel has 100 cache slots. In the extreme case, the write throughput could be super high and may results in dead lock.
    - Exponential BackOff for RPC

Tricky Bug not mentioned in the paper and MIT's handout
- When election timeout elapses, check the current state before canvassing votes. If the node is leader, do nothing.
- When handle the reply of AppendEntriesRPC, check whether prevLogIdx+len(Entries) \> matchIndex[peer]. If not, trying to update leaderCommitIdx wastes time since the reply either comes from normal hearbeats or is stale arising from RPC retries.

Reference:
1. [Student Guide issued by MIT TA](https://thesquareplanet.com/blog/students-guide-to-raft/)
2. [raft webpage](https://raft.github.io/)
3. [Course Schedule: Spring 2018](https://pdos.csail.mit.edu/6.824/schedule.html)
4. [Slides from Princeton](https://www.cs.princeton.edu/courses/archive/fall16/cos418/index.html)

Test Result
- [x] 2A Leader Election
```
➜  raft git:(master) go test -run 2A          
Test (2A): initial election ...
  ... Passed --   3.1  3   60    0
Test (2A): election after network failure ...
  ... Passed --   4.5  3  110    0
PASS
ok      raftKVDB/raft   7.540s
```

- [x] 2B Log Replication
```
➜  raft git:(master) go test -run 2B
Test (2B): basic agreement ...
  ... Passed --   0.3  5   32    3
Test (2B): agreement despite follower disconnection ...
  ... Passed --   5.6  3  118    8
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   3.6  5  195    3
Test (2B): concurrent Start()s ...
  ... Passed --   0.7  3   21    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   6.4  3  190    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  17.1  5 2106  102
Test (2B): RPC counts aren't too high ...
  ... Passed --   2.0  3   58   12
PASS
ok      raftKVDB/raft   35.638s
```

- [x] 2C Persist
```
➜  raft git:(master) go test -run 2C
Test (2C): basic persistence ...
  ... Passed --   5.6  3  134    7
Test (2C): more persistence ...
  ... Passed --  15.6  5  909   16
Test (2C): partitioned leader and one follower crash, leader restarts ...
  ... Passed --   1.4  3   40    4
Test (2C): Figure 8 ...
  ... Passed --  31.5  5 1231   62
Test (2C): unreliable agreement ...
  ... Passed --   1.6  5 1040  246
Test (2C): Figure 8 (unreliable) ...
  ... Passed --  33.4  5 10965  404
Test (2C): churn ...
  ... Passed --  16.4  5 12127 2868
Test (2C): unreliable churn ...
  ... Passed --  16.4  5 5572 1227
PASS
ok      raftKVDB/raft   121.955s
```

- [x] 3A Client Interaction
```
➜  kvraft git:(master) go test -run 3A
Test: one client (3A) ...
  ... Passed --  15.0  5 11854 2245
Test: many clients (3A) ...
  ... Passed --  15.3  5 35420 2785
Test: unreliable net, many clients (3A) ...
  ... Passed --  15.8  5 10515 1581
Test: concurrent append to same key, unreliable (3A) ...
  ... Passed --   0.9  3   235   52
Test: progress in majority (3A) ...
  ... Passed --   0.5  5    56    2
Test: no progress in minority (3A) ...
  ... Passed --   1.0  5   108    3
Test: completion after heal (3A) ...
  ... Passed --   1.0  5    67    3
Test: partitions, one client (3A) ...
  ... Passed --  22.0  5 19683 1729
Test: partitions, many clients (3A) ...
  ... Passed --  22.7  5 67288 2434
Test: restarts, one client (3A) ...
  ... Passed --  19.1  5 30636 2407
Test: restarts, many clients (3A) ...
  ... Passed --  19.8  5 98377 2874
Test: unreliable net, restarts, many clients (3A) ...
  ... Passed --  20.4  5 11413 1591
Test: restarts, partitions, many clients (3A) ...
  ... Passed --  26.8  5 57615 2176
Test: unreliable net, restarts, partitions, many clients (3A) ...
  ... Passed --  28.1  5  7173  811
Test: unreliable net, restarts, partitions, many clients, linearizability checks (3A) ...
  ... Passed --  25.2  7 19555 1578
PASS
ok      raftKVDB/kvraft 234.067s
```
