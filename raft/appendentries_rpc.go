package raft

import "fmt"

type AppendEntriesArgs struct {
	Term         int        // leader's term
	LeaderID     int        // so follower can redirect clients
	PrevLogIndex int        // index of log entry immediately preceding new ones
	PrevLogTerm  int        // term of prevLogIndex entry
	Entries      []LogEntry // log entries to store(empty for heartbeat; may send more than one for efficiency)
	LeaderCommit int        // leader's commitIndex
}

type AppendEntriesReply struct {
	Term    int  // currentTerm, for leader to update itself
	Success bool // true if follower contained entry matching prevLogIndex and prevLogTerm

	// extra info for heartbeat from follower
	//ConflictTerm int // term of the conflicting entry
	//FirstIndex   int // the first index it stores for ConflictTerm
	ConflictIndex int
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	DPrintf("%d<-%d, Start Append\nargs=%s\nraftState=%s\n",
		rf.me, args.LeaderID, args.str(), rf.str())
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.CurrentTerm {
		reply.Term = rf.CurrentTerm
		reply.Success = false
		return
	}

	change := false
	reply.Term = args.Term
	if rf.CurrentTerm < args.Term {
		rf.CurrentTerm = args.Term
		rf.VotedFor = -1
		rf.goBackToFollower()
		change = true
	} else if rf.state!=FOLLOWER {
		// candidate lose the leader election
		rf.state = FOLLOWER
	}

	rf.resetTimerCh <- struct{}{}

	localLastIdx := len(rf.Logs) - 1
	localPrevIdx := rf.logIdxGlobal2Local(args.PrevLogIndex)

	if args.PrevLogIndex<rf.SnapshotIndex {
		// In principle, the conflictIndex could be set to rf.commitIndex+1, but it would introduce more traffic of Log Replica.
		// The downside to set conflictIndex to be the tail of local log is that would repeat the consistency check
		// And you have to change the code in checkConsistency to avoid out of range problem: localIdx := min(rf.logIdxGlobal2Local(prevIdx), len(rf.Logs)-1)
		// Such out of range may happens if follower has more logs than the leader's due to network partition.
		reply.Success=false
		reply.ConflictIndex = rf.logIdxLocal2Global(localLastIdx) + 1
		DPrintf("%d<-%d, End Append, Conflict Case1: leader's prev<follower's snap\nargs=%s, reply=%s\nraftState=%s\n",
			rf.me, args.LeaderID, args.str(), reply.str(), rf.str())
	} else if localPrevIdx > localLastIdx{
		reply.Success = false
		reply.ConflictIndex = rf.logIdxLocal2Global(localLastIdx) + 1
		DPrintf("%d<-%d, End Append, Conflict Case2: leader's prev>follower's lastLog\nargs=%s, reply=%s\nraftState=%s\n",
			rf.me, args.LeaderID, args.str(), reply.str(), rf.str())
	} else if rf.Logs[localPrevIdx].Term != args.PrevLogTerm {
		// find the head index whose term is the same as PrevLog's term
		var i = localPrevIdx
		var t = rf.Logs[i].Term
		for ; i>0; i-- {
			if rf.Logs[i-1].Term != t {
				break
			}
		}
		reply.Success = false
		reply.ConflictIndex = rf.logIdxLocal2Global(i)
		DPrintf("%d<-%d, End Append, Conflict Case3: leader's prev>follower's lastLog\nargs=%s, reply=%s\nraftState=%s\n",
			rf.me, args.LeaderID, args.str(), reply.str(), rf.str())
	} else {
		reply.Success = true
		i := localPrevIdx+1
		j := 0
		for ; i < len(rf.Logs) && j < len(args.Entries); i, j = i+1, j+1 {
			if rf.Logs[i].Term != args.Entries[j].Term {
				break
			}
		}

		if j < len(args.Entries) {
			rf.Logs = append(rf.Logs[:i], args.Entries[j:]...)
			change = true
		}

		oldCommitIndex := rf.commitIndex
		if args.LeaderCommit > rf.commitIndex {
			// Suspicious Point, last of new entries?.
			if rf.commitIndex == rf.logIdxLocal2Global(len(rf.Logs)-1) {
				DPrintf("Internal Error when reply to AppendRPC")
			}
			rf.commitIndex = min(args.LeaderCommit, rf.logIdxLocal2Global(len(rf.Logs)-1))
			rf.commitCh <- struct{}{}
		}

		DPrintf("%d<-%d, End Append, Success, commitIndex=%d->%d\nargs=%s, reply=%s\nraftState=%s\n",
			rf.me, args.LeaderID, oldCommitIndex, rf.commitIndex, args.str(), reply.str(), rf.str())
	}

	if change {
		rf.persistState()
	}
}


func (req *AppendEntriesArgs) str() string {
	return fmt.Sprintf("AppendRPC ArgsTerm=%d, LeaderID=%d, PrevLogIndex=%d, PrevLogTerm=%d, LeaderCommit=%d, AppendLenth=%d",
		req.Term, req.LeaderID, req.PrevLogIndex, req.PrevLogTerm, req.LeaderCommit, len(req.Entries))
}

func (reply *AppendEntriesReply) str() string {
	return fmt.Sprintf("AppendRPC ReplyTerm=%d, Success=%t, ConflictIdx=%d", reply.Term, reply.Success, reply.ConflictIndex)
}
