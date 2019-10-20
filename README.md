# raftKVDB
MIT 6.824

Tricky Points:

Reference:
1. [Student Guide issued by MIT TA](https://thesquareplanet.com/blog/students-guide-to-raft/)
2. [raft webpage](https://raft.github.io/)
3. [Course Schedule: Spring 2017](http://nil.csail.mit.edu/6.824/2017/schedule.html)
4. [Slides from Princeton](https://www.cs.princeton.edu/courses/archive/fall16/cos418/index.html)

Timeline
- [x] 2A Leader Election
```
➜  raft git:(master) go test -run 2A
Test (2A): initial election ...
warning: term changed even though there were no failures  ... Passed --   3.6  3  170    0
Test (2A): election after network failure ...
  ... Passed --   6.0  3   86    0
PASS
ok      raftKVDB/raft   9.596s
```

- [x] 2B Log Replication
```
➜  raft git:(master) go test -run 2B
Test (2B): basic agreement ...
  ... Passed --   1.2  5   56    3
Test (2B): agreement despite follower disconnection ...
  ... Passed --   3.9  3  184    7
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   4.1  5  149    3
Test (2B): concurrent Start()s ...
  ... Passed --   0.8  3   10    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   3.6  3   81    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  25.6  5 6884  110
Test (2B): RPC counts aren't too high ...
  ... Passed --   2.2  3   68   12
PASS
ok      raftKVDB/raft   41.436s
```
