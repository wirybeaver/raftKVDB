# raftKVDB
MIT 6.824

Confusing Points:
1. Step 3 in AppendEntries RPC in Figure 2. Why cannot we truncate log entries after the prevLogIndex when the entry indexed by the prevLogIndex matches?

Reference:
1. [Student Guide issued by MIT TA](https://thesquareplanet.com/blog/students-guide-to-raft/)
2. [raft webpage](https://raft.github.io/)
3. [Course Schedule: Spring 2017](http://nil.csail.mit.edu/6.824/2017/schedule.html)
4. [Slides from Prinston](https://www.cs.princeton.edu/courses/archive/fall16/cos418/index.html)
