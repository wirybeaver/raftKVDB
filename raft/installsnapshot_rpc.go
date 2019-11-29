package raft

import "fmt"

// InstallSnapShot RPC
type InstallSnapshotArgs struct {
	Term              int // leader's term
	LeaderID          int
	LastIncludedIndex int
	LastIncludedTerm  int
	Snapshot          []byte
}

type InstallSnapshotReply struct {
	Term int // for leader to update itself
}

func (rf *Raft) InstallSnapShot(args *InstallSnapshotArgs, reply *InstallSnapshotReply) {
	rf.mu.Lock()
	DPrintf("%d<-%d, Start SnapShot\nargs=%s\nraftState=%s\n",
		rf.me, args.LeaderID, args.str(), rf.str())
	if args.Term < rf.CurrentTerm {
		reply.Term = rf.CurrentTerm
		rf.mu.Unlock()
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

	// all servers agree on all logs up to commitIndex per the proof of raft. If args.LastIncludedIndex <=rf.commitIndex, it indicates the snapshot is outdated.
	// But in what scenario this case occurs?
	// In principle, InstallSnapshot RPC should be triggered only when the leader doesn't contain the log with the conflict index reported by the follower.
	// And follower should always receive SnapshotIndex larger than local commitIndex if there's no network issue and FIFO.
	// However, the network is inevitable to be unstable in real world and cause this outdated issue. Let's get into it with more details
	// If a leader find that nextIndex[follower] <= leader's snapshotIndex is true, the leader will send a InstallSnapshot request.
	// But NextIndex[follower] sometimes would be changed incidentally by the reply of a stale AppendEntries request due to flaky network.
	// If the outdated AppendEntries reply's conflict index is <= current leader's snapshotIndex,
	// the leader set nextIndex[follower] to the outdated conflictIndex in the first place
	// Then, the leader would trigger InstallSnapshot during the next period of consistencyCheck.
	if args.LastIncludedIndex <= rf.SnapshotIndex || args.LastIncludedIndex <=rf.commitIndex {
		if change {
			rf.persistState()
		}
		rf.mu.Unlock()
		DPrintf("%d<-%d, End SnapShot, Stale case: leader's snap<=follower's snapshotIndex or commitIndex\nargs=%s\nraftState=%s\n",
			rf.me, args.LeaderID, args.str(), rf.str())
		return
	}

	size :=1
	argsLocalLastIncludeIndex := rf.logIdxGlobal2Local(args.LastIncludedIndex)
	if argsLocalLastIncludeIndex < len(rf.Logs) && rf.Logs[argsLocalLastIncludeIndex].Term == args.LastIncludedTerm {
		size = len(rf.Logs) - argsLocalLastIncludeIndex
	}
	newLog := make([]LogEntry, size)
	newLog[0] = LogEntry{
		GlobalIndex: args.LastIncludedIndex,
		Term: args.LastIncludedTerm,
		Command: nil,
	}
	if size>1 {
		copy(newLog[1:], rf.Logs[argsLocalLastIncludeIndex+1:])
	}

	// It's not a necessity to update lastApplied or commitIndex.
	rf.Logs = newLog
	rf.SnapshotIndex = args.LastIncludedIndex
	state := rf.encodeState()
	rf.persister.SaveStateAndSnapshot(state, args.Snapshot)
	DPrintf("%d<-%d, End SnapShot, Success\nargs=%s\nraftState=%s\n",
		rf.me, args.LeaderID, args.str(), rf.str())
	rf.mu.Unlock()
	rf.notifyAppUseNewSnapShot(args.Snapshot)
}

func (rf *Raft) notifyAppUseNewSnapShot(snapshot []byte) {
	if snapshot==nil || len(snapshot)<1 {
		return
	}
	rf.applyCh <- ApplyMsg{
		CommandValid: false,
		SnapShot: snapshot,
	}
}

func (req *InstallSnapshotArgs) str() string {
	return fmt.Sprintf("InstallRPC ArgsTerm=%d, Leader=%d, LastIncludeIndex=%d, LastIncludeTerm=%d", req.Term, req.LeaderID, req.LastIncludedIndex, req.LastIncludedTerm)
}
