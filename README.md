# raftKVDB
MIT 6.824

Optimization TODO:
- Find the median of matchIndex using Quick Selection, whose running time is linear
- Replace commitChannel with a Conditional Variable

Tricky Bug not mentioned in the paper and MIT's handout
- When election timeout elapses, check the current state before canvassing votes. If the node is leader, do nothing.
- When handle the reply of AppendEntriesRPC, check whether prevLogIdx+len(Entries) \> matchIndex[peer]. If not, trying to update leaderCommitIdx wastes time.

Reference:
1. [Student Guide issued by MIT TA](https://thesquareplanet.com/blog/students-guide-to-raft/)
2. [raft webpage](https://raft.github.io/)
3. [Course Schedule: Spring 2017](http://nil.csail.mit.edu/6.824/2017/schedule.html)
4. [Slides from Princeton](https://www.cs.princeton.edu/courses/archive/fall16/cos418/index.html)

Test Result
- [x] 2A Leader Election
```
➜  raft git:(master) go test -run 2A 
Test (2A): initial election ...
  ... Passed --   3.1  3   62    0
Test (2A): election after network failure ...
  ... Passed --   4.6  3  109    0
PASS
ok      raftKVDB/raft   7.696s
```

- [x] 2B Log Replication
```
➜  raft git:(master) go test -run 2B
Test (2B): basic agreement ...
  ... Passed --   0.6  5   49    3
Test (2B): agreement despite follower disconnection ...
  ... Passed --   6.3  3  118    7
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   3.9  5  178    4
Test (2B): concurrent Start()s ...
  ... Passed --   0.8  3   14    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   4.9  3  151    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  25.7  5 2209  102
Test (2B): RPC counts aren't too high ...
  ... Passed --   2.3  3   42   12
PASS
ok      raftKVDB/raft   44.553s
```
