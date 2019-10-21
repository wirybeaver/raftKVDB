# raftKVDB
MIT 6.824

Optimization:
- DONE
    - Quickly backup over incorrect
    - Try to remove mutex on applyDaemon loop, since there's no data race when read committed log. But it seems useless to improve performance.
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
  ... Passed --   4.5  3  111    0
PASS
ok      raftKVDB/raft   7.602s
```

- [x] 2B Log Replication
```
➜  raft git:(master) go test -run 2B
Test (2B): basic agreement ...
  ... Passed --   0.3  5   47    3
Test (2B): agreement despite follower disconnection ...
  ... Passed --   5.6  3  123    8
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   3.6  5  187    3
Test (2B): concurrent Start()s ...
  ... Passed --   0.6  3   20    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   6.3  3  189    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  17.1  5 2104  102
Test (2B): RPC counts aren't too high ...
  ... Passed --   2.4  3   67   12
PASS
ok      raftKVDB/raft   36.049s
```
