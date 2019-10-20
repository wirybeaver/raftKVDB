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
 warning: term changed even though there were no failures  ... Passed
 Test (2A): election after network failure ...
   ... Passed
 PASS
 ok      raftKVDB/raft   7.023s
```

- [x] 2B Log Replication
```
➜  raft git:(master) go test -run 2B
 Test (2B): basic agreement ...
   ... Passed
 Test (2B): agreement despite follower disconnection ...
   ... Passed
 Test (2B): no agreement if too many followers disconnect ...
   ... Passed
 Test (2B): concurrent Start()s ...
   ... Passed
 Test (2B): rejoin of partitioned leader ...
   ... Passed
 Test (2B): leader backs up quickly over incorrect follower logs ...
   ... Passed
 Test (2B): RPC counts aren't too high ...
   ... Passed
 PASS
 ok      raftKVDB/raft   41.406s
```
