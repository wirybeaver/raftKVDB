# raftKVDB
MIT 6.824

Confusing Points:
1. Step 3 in AppendEntries RPC in Figure 2. Why cannot we truncate log entries after the prevLogIndex when the entry indexed by the prevLogIndex matches?

Reference:
1. [Student Guide issued by MIT TA](https://thesquareplanet.com/blog/students-guide-to-raft/)
2. [raft webpate](https://raft.github.io/)
