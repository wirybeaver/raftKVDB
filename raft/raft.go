package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"bytes"
	"encoding/gob"
	"fmt"

	//"log/syslog"
	"math/rand"
	"sync"
	"time"
)
import "raftKVDB/labrpc"

// import "bytes"
// import "labgob"



//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandIndex       int
	Command     interface{}
	CommandValid bool   // ignore for lab2; only used in lab3
	SnapShot []byte
}

type LogEntry struct {
	GlobalIndex int
	Term    int
	Command interface{}
}

const (
	FOLLOWER = iota
	CANDIDATE
	LEADER
)

const (
	CONFLICTINTERVAL     = 10
	HEARTBEATINTERVAL    = 100
	ELECTIONTIMEOUTFIXED = 400
	ELECTIONTIMEOUTRAND  = 400
)

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.

	CurrentTerm int        // Persisted before responding to RPCs
	VotedFor    int        // Persisted before responding to RPCs
	Logs        []LogEntry // Persisted before responding to RPCs
	SnapshotIndex int      // Used for snapshot

	commitIndex int        // Volatile state on all servers
	lastApplied int        // Volatile state on all servers
	nextIndex   []int      // Leader only, reinitialized after election
	matchIndex  []int      // Leader only, reinitialized after election

	// extra field used for the implementation
	state int         // follower, candidate or leader
	voteCount int     // used for count votes
	timer *time.Timer // election timer
	seed  rand.Source

	commitCh     chan struct{} // for commitIndex update
	resetTimerCh chan struct{} // for reset election timer
	applyCh      chan ApplyMsg // channel to send ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	var term int
	var isleader bool
	// Your code here (2A).

	rf.mu.Lock()
	defer rf.mu.Unlock()

	term = rf.CurrentTerm
	isleader = rf.state == LEADER

	return term, isleader
}

func (rf *Raft) GetStateSize() int {
	return rf.persister.RaftStateSize()
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persistState() {
	// Your code here (2C).
	// Example:
	rf.persister.SaveRaftState(rf.encodeState())
}

func (rf *Raft) dePersistState() {
	rf.decodeState(rf.persister.ReadRaftState())
}

func (rf *Raft) encodeState() []byte {
	w := new(bytes.Buffer)
	e := gob.NewEncoder(w)
	e.Encode(rf.CurrentTerm)
	e.Encode(rf.VotedFor)
	e.Encode(rf.SnapshotIndex)
	e.Encode(rf.Logs)
	return w.Bytes()
}

//
// restore previously persisted state.
//
func (rf *Raft) decodeState(data []byte) {
	// Your code here (2C).
	// Example:
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := gob.NewDecoder(r)
	var currentTerm int
	var votedFor int
	var snapShotIndex int
	var logs []LogEntry
	if d.Decode(&currentTerm)!=nil ||d.Decode(&votedFor)!=nil || d.Decode(&snapShotIndex)!=nil || d.Decode(&logs)!=nil{
		DPrintf("Error: server %d fail to read Persisted state", rf.me)
	} else {
		rf.CurrentTerm = currentTerm
		rf.VotedFor = votedFor
		rf.SnapshotIndex = snapShotIndex
		rf.Logs = logs
	}
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int // candidate's term
	CandidateID  int // candidate requesting vote
	LastLogIndex int // index of candidate's last log entry
	LastLogTerm  int // term of candidate's last log entry
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).
	Term int  // currentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	//DPrintf("[%d] recvVote-Start from candidate=[%d]\n req=[%v]\n rf=[%v]", rf.me, args.CandidateID, args.str(), rf.str())
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.VoteGranted = false
	if args.Term < rf.CurrentTerm {
		reply.Term = rf.CurrentTerm
		//DPrintf("[%d] recvVote-StaleRequest from candidate=[%d]\n req=[%v]\n reply=[]%v\n rf=[%v]", rf.me, args.CandidateID, args.str(), reply.str(), rf.str())
		return
	}

	change := false
	reply.Term = args.Term
	if args.Term > rf.CurrentTerm {
		rf.CurrentTerm = args.Term
		rf.VotedFor = -1
		rf.goBackToFollower()
		change=true
	}

	// Suspicious Point. I figure the second condition is necessary to avoid the loss of previous grant reply
	if rf.VotedFor == -1 || rf.VotedFor == args.CandidateID {
		idx := len(rf.Logs) - 1
		localLastLogTerm := rf.Logs[idx].Term
		localLastLogIndex := rf.logIdxLocal2Global(idx)
		if args.LastLogTerm > localLastLogTerm || args.LastLogTerm == localLastLogTerm && args.LastLogIndex >= localLastLogIndex {
			//rf.state = FOLLOWER
			if rf.VotedFor == -1 {
				rf.VotedFor = args.CandidateID
				change = true
			}
			reply.VoteGranted = true
			//DPrintf("[%d] recvVote-GrantVote from candidate=[%d]\n req=[%v]\n reply=[]%v\n rf=[%v]", rf.me, args.CandidateID, args.str(), reply.str(), rf.str())
			rf.resetTimerCh <- struct{}{}
		}
		//else {
		//  DPrintf("[%d] recvVote-NoVote-UpToDate from candidate=[%d]\n req=[%v]\n reply=[]%v\n rf=[%v]", rf.me, args.CandidateID, args.str(), reply.str(), rf.str())
		//}
	}
	if change {
		rf.persistState()
	}
	//else{
	//  DPrintf("[%d] recvVote-NoVote-1 from candidate=[%d]\n req=[%v]\n reply=[]%v\n rf=[%v]", rf.me, args.CandidateID, args.str(), reply.str(), rf.str())
	//}
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
}

func (rf *Raft) sendRequestVoteAndProcess (args *RequestVoteArgs, sendTo int) {
	//DPrintf("Cand [%d] sendVote to peer=[%d]\n req=[%v]\n rf=[%v]", rf.me, sendTo, args.str(), rf.str())
	reply := &RequestVoteReply{}
	ok := rf.sendRequestVote(sendTo, args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if !ok || rf.state!=CANDIDATE || reply.Term < rf.CurrentTerm {
		return
	}
	if reply.Term > rf.CurrentTerm {
		rf.CurrentTerm = reply.Term
		rf.VotedFor = -1
		rf.goBackToFollower()
		rf.resetTimerCh <- struct{}{}
		rf.persistState()
		return
	}

	// it indicated ok == true && reply.CurrentTerm == rf.CurrentTerm && rf.state==CANDIDATE
	if reply.VoteGranted {
		rf.voteCount++
		if rf.voteCount > len(rf.peers)/2 {
			// leader initialization
			rf.state = LEADER
			//rf.voteCount = 0
			for i:= range rf.nextIndex{
				localIdx := len(rf.Logs)-1
				rf.nextIndex[i] = rf.logIdxLocal2Global(localIdx)+1
				rf.matchIndex[i] = 0
			}
			rf.matchIndex[rf.me] = rf.nextIndex[rf.me]-1
			go rf.heartBeatDaemon()
			//DPrintf("Candidate=%d Became Leader to peer=[%d]\n req=[%v]\n reply=[%v]\n rf=[%v]", rf.me, sendTo, args.str(), reply.str(), rf.str())
			rf.resetTimerCh <- struct{}{}
		}
	}
}

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

		tmp := rf.commitIndex
		if args.LeaderCommit > rf.commitIndex {
			// Suspicious Point, last of new entries?.
			if rf.commitIndex == rf.logIdxLocal2Global(len(rf.Logs)-1) {
				DPrintf("Internal Error when reply to AppendRPC")
			}
			rf.commitIndex = min(args.LeaderCommit, rf.logIdxLocal2Global(len(rf.Logs)-1))
			rf.commitCh <- struct{}{}
		}

		DPrintf("%d<-%d, End Append, Success, commitIndex=%d->%d\nargs=%s, reply=%s\nraftState=%s\n",
			rf.me, args.LeaderID, tmp, rf.commitIndex, args.str(), reply.str(), rf.str())
	}

	if change {
		rf.persistState()
	}
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

func (rf *Raft) sendAppendEntriesAndProcess(args *AppendEntriesArgs, server int) {
	reply := &AppendEntriesReply{}
	ok := rf.sendAppendEntries(server, args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if !ok || rf.state!=LEADER || reply.Term < rf.CurrentTerm {
		return
	}
	if rf.CurrentTerm < reply.Term {
		rf.CurrentTerm = reply.Term
		rf.VotedFor = -1
		rf.goBackToFollower()
		rf.resetTimerCh <- struct{}{}
		rf.persistState()
		return
	}
	tmp1 := rf.commitIndex
	tmp2 := rf.nextIndex[server]
	if reply.Success {
		N := args.PrevLogIndex+len(args.Entries)
		if N>rf.matchIndex[server] {
			rf.matchIndex[server] = N
			rf.nextIndex[server] = N+1
			idx := rf.logIdxGlobal2Local(N)
			count := 0
			if N>rf.commitIndex && rf.Logs[idx].Term==rf.CurrentTerm {
				for _, index := range rf.matchIndex {
					if index>=N {
						count++
					}
				}
			}
			if count > len(rf.peers)/2 {
				rf.commitIndex = N
				rf.commitCh <- struct{}{}
			}
		}
	} else {
		rf.nextIndex[server] = max(1, reply.ConflictIndex)
		go rf.backupOverIncorrectFollowerLogs(server)
	}
	DPrintf("%d->%d, After sendAppend, nextIndex[%d]=%d->%d, commitID=%d->%d\nargs=%s, reply=%s\nraftState:%s\n", rf.me, server, server, tmp2, rf.nextIndex[server], tmp1, rf.commitIndex, args.str(), reply.str(), rf.str())
}

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
		//DPrintf("[%d] recvInstallSnap-StaleReq from leader=[%d]\n rf=[%v]\n req=[%v]\n reply=[%v]", rf.me, args.LeaderID, rf.str(), args.str(), reply.str())
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

	if args.LastIncludedIndex <= rf.SnapshotIndex {
		if change {
			rf.persistState()
		}
		rf.mu.Unlock()
		DPrintf("%d<-%d, End SnapShot, Stale case: leader's snap<=follower's snap\nargs=%s\nraftState=%s\n",
			rf.me, args.LeaderID, args.str(), rf.str())
		return
	}

	if args.LastIncludedIndex <= rf.commitIndex {
		if change {
			rf.persistState()
		}
		rf.mu.Unlock()
		DPrintf("%d<-%d, End SnapShot, Stale case: leader's snap<=follower's lastApplied\nargs=%s\nraftState=%s\n",
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
	//
	//if args.LastIncludedIndex > rf.lastApplied {
	//	rf.lastApplied = args.LastIncludedIndex
	//}
	//
	//if args.LastIncludedIndex > rf.commitIndex {
	//	rf.commitIndex = args.LastIncludedIndex
	//}

	rf.Logs = newLog
	rf.SnapshotIndex = args.LastIncludedIndex

	DPrintf("%d<-%d, End SnapShot, Success\nargs=%s\nraftState=%s\n",
		rf.me, args.LeaderID, args.str(), rf.str())

	state := rf.encodeState()
	rf.persister.SaveStateAndSnapshot(state, args.Snapshot)
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

func (rf *Raft) sendSnapShot(args *InstallSnapshotArgs, reply *InstallSnapshotReply, server int) bool {
	ok := rf.peers[server].Call("Raft.InstallSnapShot", args, reply)
	return ok
}

func (rf *Raft) sendSnapShotAndProcess(args *InstallSnapshotArgs, server int) {
	reply := &InstallSnapshotReply{}
	ok := rf.sendSnapShot(args, reply, server)

	rf.mu.Lock()
	defer rf.mu.Unlock()

	if !ok || rf.state!=LEADER || rf.CurrentTerm > reply.Term {
		return
	}

	if rf.CurrentTerm < reply.Term {
		rf.CurrentTerm = reply.Term
		rf.VotedFor = -1
		rf.goBackToFollower()
		rf.resetTimerCh <- struct{}{}
		rf.persistState()
		return
	}
	tmp:=rf.nextIndex[server]
	if args.LastIncludedIndex >= rf.nextIndex[server] {
		rf.nextIndex[server]=args.LastIncludedIndex+1
		rf.matchIndex[server]=args.LastIncludedIndex
	}
	DPrintf("%d->%d, After sendSnapshot, nextIndex[%d]=%d->%d\nargs=%s, reply.Term=%d\nRaftState:%s\n", rf.me, server, server, tmp, rf.nextIndex[server], args.str(), reply.Term, rf.str())
}

func (rf *Raft) Compact(cmdIndex int, snapshot [] byte) {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	//if rf.state==LEADER {
		DPrintf("raftMe=%d, Start compact, ArgsLastIncludeIdx=%d\nRaftState:%s\n", rf.me, cmdIndex, rf.str())
	//}


	if cmdIndex <= rf.SnapshotIndex {
		DPrintf("raftMe=%d, receive Stale lastIncludeIndex=%d\nRaftState=%s\n", rf.me, cmdIndex, rf.str())
		return
	}

	localLastIncludeIndex := rf.logIdxGlobal2Local(cmdIndex)

	if localLastIncludeIndex >= len(rf.Logs) {
		DPrintf("Internal Error, the log doesn't have a log for given log index")
		return
	}
	size := 1 + max(0, len(rf.Logs)-1-localLastIncludeIndex)
	newLog := make([]LogEntry, size)
	newLog[0] = LogEntry{
		GlobalIndex: cmdIndex,
		Term: rf.Logs[localLastIncludeIndex].Term,
		Command: nil,
	}

	if size > 1 {
		copy(newLog[1:], rf.Logs[localLastIncludeIndex+1:])
	}

	rf.Logs = newLog
	rf.SnapshotIndex = cmdIndex
	raftState := rf.encodeState()

	DPrintf("raftMe=%d, End compact, ArgsLastIncludeIndex=%d\nRaftState:%s\n", rf.me, cmdIndex, rf.str())


	rf.persister.SaveStateAndSnapshot(raftState, snapshot)
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	index := -1
	term := -1
	isLeader := false

	// Your code here (2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state==LEADER {
		log := LogEntry{rf.SnapshotIndex+len(rf.Logs), rf.CurrentTerm, command}
		rf.Logs = append(rf.Logs, log)
		index = rf.logIdxLocal2Global(len(rf.Logs)-1)
		term = rf.CurrentTerm
		isLeader = true
		rf.nextIndex[rf.me] = index+1
		rf.matchIndex[rf.me] = index
		go rf.fastBroadCastNewCommand()
		rf.persistState()
	}

	return index, term, isLeader
}

//
// the tester calls Kill() when a Raft instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (rf *Raft) Kill() {
	// Your code here, if desired.
	rf.timer.Stop()
	rf.state=FOLLOWER
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).

	rf.CurrentTerm = 0
	rf.VotedFor = -1
	rf.SnapshotIndex = 0
	// first log index is 1, thus we need a dummy log with index 0
	rf.Logs = make([]LogEntry, 1)
	rf.Logs[0] = LogEntry{
		Term:    0,
		Command: nil,
	}
	rf.lastApplied = 0
	rf.commitIndex = 0
	rf.nextIndex = make([]int, len(peers))
	for i := range rf.nextIndex {
		rf.nextIndex[i]=1
	}
	rf.matchIndex = make([]int, len(peers))

	rf.seed = rand.NewSource(int64(rf.me))
	rf.state = FOLLOWER
	rf.timer = time.NewTimer(0)
	rf.resetTimerCh = make(chan struct{})
	rf.commitCh = make(chan struct{}, 100)
	rf.applyCh = applyCh

	// initialize from state persisted before a crash
	rf.dePersistState()
	rf.notifyAppUseNewSnapShot(rf.persister.ReadSnapshot())

	go rf.electionDaemon()      // kick off election
	go rf.applyLogEntryDaemon() // distinguished thread to apply log up through commitIdx

	return rf
}

func (rf *Raft) electionDaemon() {
	for {
		select {
		case <-rf.resetTimerCh:
			rf.resetTimer()
		case <-rf.timer.C:
			go rf.broadCastVote()
			rf.timer.Reset(rf.randomizeTimeout())
		}

	}
}

func (rf *Raft) broadCastVote() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state == LEADER {
		return
	}
	DPrintf("[%d] Became Candidate", rf.me)
	rf.CurrentTerm++
	rf.state = CANDIDATE
	rf.VotedFor = rf.me
	rf.voteCount=1
	localIdx := len(rf.Logs)-1

	voteReq := &RequestVoteArgs{
		Term: rf.CurrentTerm,
		CandidateID: rf.me,
		LastLogTerm: rf.Logs[localIdx].Term,
		LastLogIndex: rf.logIdxLocal2Global(localIdx),
	}

	for i:=0; i<len(rf.peers); i++ {
		if rf.me == i {
			continue
		}
		go rf.sendRequestVoteAndProcess(voteReq, i)
	}

	rf.persistState()
}

func (rf *Raft) heartBeatDaemon(){
	for {
		if rf.state != LEADER {
			return
		}
		for i:=0; i<len(rf.peers); i++ {
			if i!= rf.me {
				go rf.checkConsistency(i)
			}
		}
		time.Sleep(HEARTBEATINTERVAL*time.Millisecond)
	}
}

func (rf *Raft) backupOverIncorrectFollowerLogs(peer int) {
	time.Sleep(time.Millisecond*CONFLICTINTERVAL)
	rf.checkConsistency(peer)
}

func (rf *Raft) fastBroadCastNewCommand(){
	for i := range rf.peers {
		if i!=rf.me {
			rf.checkConsistency(i)
		}
	}
}

func (rf *Raft) checkConsistency(to int) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.state != LEADER {
		return
	}
	prevIdx := rf.nextIndex[to]-1
	if prevIdx<rf.SnapshotIndex {
		args := &InstallSnapshotArgs{
			Term: rf.CurrentTerm,
			LeaderID: rf.me,
			LastIncludedIndex: rf.SnapshotIndex,
			LastIncludedTerm: rf.Logs[0].Term,
			Snapshot: rf.persister.ReadSnapshot(),
		}
		DPrintf("raftMe=%d, Before sendSnapshot to peer=%d, nextIndex[%d]=%d, args=%s\nRaftState:%s\n", rf.me, to, to, rf.nextIndex[to], args.str(), rf.str())
		go rf.sendSnapShotAndProcess(args, to)
	} else {
		localIdx := rf.logIdxGlobal2Local(prevIdx)
		request := &AppendEntriesArgs{
			Term : rf.CurrentTerm,
			LeaderID: rf.me,
			PrevLogIndex: prevIdx,
			PrevLogTerm: rf.Logs[localIdx].Term,
			Entries: nil,
			LeaderCommit: rf.commitIndex,
		}
		request.Entries = append(request.Entries, rf.Logs[localIdx+1:]...)
		DPrintf("raftMe=%d, Before sendAppend to peer=%d, nextIndex[%d]=%d, args=%s\nRaftState:%s\n", rf.me, to, to, rf.nextIndex[to], request.str(), rf.str())
		go rf.sendAppendEntriesAndProcess(request, to)
	}
}

func (rf *Raft) applyLogEntryDaemon() {
	for {
		<- rf.commitCh
		rf.mu.Lock()
		start, end := rf.lastApplied+1, rf.commitIndex
		//rf.mu.Unlock()
		for i := start; i<=end; i++ {
			if i<=rf.SnapshotIndex {
				i = rf.SnapshotIndex
			} else {
				idx := rf.logIdxGlobal2Local(i)
				rf.applyCh <- ApplyMsg{
					Command: rf.Logs[idx].Command,
					CommandIndex: i,
					CommandValid: true,
				}
				rf.lastApplied=i
			}
		}
		//rf.mu.Lock()
		rf.mu.Unlock()
	}
}

// helper function below
func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func max(a int, b int) int {
	if a > b {
		return a
	} else {
		return b
	}
}

func (rf *Raft) logIdxLocal2Global(localIdx int) int {
	return localIdx+rf.SnapshotIndex
}

func (rf *Raft) logIdxGlobal2Local(globalIdx int) int {
	return globalIdx-rf.SnapshotIndex
}

func (rf *Raft) resetTimer() {
	if !rf.timer.Stop() {
		<-rf.timer.C
	}
	rf.timer.Reset(rf.randomizeTimeout())
}

func (rf *Raft) randomizeTimeout() time.Duration {
	return time.Millisecond * time.Duration(ELECTIONTIMEOUTFIXED+rand.New(rf.seed).Intn(ELECTIONTIMEOUTRAND))
}

// util function, must be called within critical section
// sending info to the Channel resetTimer often follows this function
func (rf *Raft) goBackToFollower(){
	rf.state = FOLLOWER
	rf.voteCount = 0
}

func (rf *Raft) str() string {
	ans := fmt.Sprintf("raftID=%d, Term=%d, state=%d, SnapshotIdx=%d, commitIdx=%d, lastApplies=%d\nLogs=%v\n",
		rf.me, rf.CurrentTerm, rf.state, rf.SnapshotIndex, rf.commitIndex,
		rf.lastApplied, rf.Logs)
	if rf.state==LEADER {
		ans += fmt.Sprintf("nextIndexs=%v\n", rf.nextIndex)
	}
	return ans
}

func (req *RequestVoteArgs) str() string {
	return fmt.Sprintf("VoteRPC ArgsTerm=%d, CandID=%d, LastLogIdx=%d, LastLogTerm=%d", req.Term, req.CandidateID, req.LastLogIndex, req.LastLogTerm)
}

func (reply *RequestVoteReply) str() string {
	return fmt.Sprintf("VoteRPC ReplyTerm=%d, VoteGrandted=%t", reply.Term, reply.VoteGranted)
}

func (req *AppendEntriesArgs) str() string {
	return fmt.Sprintf("AppendRPC ArgsTerm=%d, LeaderID=%d, PrevLogIndex=%d, PrevLogTerm=%d, LeaderCommit=%d, AppendLenth=%d",
		req.Term, req.LeaderID, req.PrevLogIndex, req.PrevLogTerm, req.LeaderCommit, len(req.Entries))
}

func (reply *AppendEntriesReply) str() string {
	return fmt.Sprintf("AppendRPC ReplyTerm=%d, Success=%t, ConflictIdx=%d", reply.Term, reply.Success, reply.ConflictIndex)
}

func (req *InstallSnapshotArgs) str() string {
	return fmt.Sprintf("InstallRPC ArgsTerm=%d, Leader=%d, LastIncludeIndex=%d, LastIncludeTerm=%d", req.Term, req.LeaderID, req.LastIncludedIndex, req.LastIncludedTerm)
}