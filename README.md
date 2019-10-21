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
  ... Passed --   3.6  3   68    0
Test (2A): election after network failure ...
  ... Passed --   4.6  3  114    0
PASS
ok      raftKVDB/raft   8.120s
```

- [x] 2B Log Replication
```
➜  raft git:(master) go test -run 2B
Test (2B): basic agreement ...
  ... Passed --   1.1  5   52    3
Test (2B): agreement despite follower disconnection ...
  ... Passed --   6.1  3  117    8
Test (2B): no agreement if too many followers disconnect ...
  ... Passed --   3.8  5  186    3
Test (2B): concurrent Start()s ...
  ... Passed --   0.8  3   14    6
Test (2B): rejoin of partitioned leader ...
  ... Passed --   5.0  3  152    4
Test (2B): leader backs up quickly over incorrect follower logs ...
  ... Passed --  34.9  5 2611  105
Test (2B): RPC counts aren't too high ...
  ... Passed --   2.4  3   47   12
PASS
ok      raftKVDB/raft   54.192s
```
