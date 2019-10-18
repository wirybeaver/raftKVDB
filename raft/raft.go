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
	"math/rand"
	"sync"
	"time"
)
import "raftKVDB/labrpc"

// import "bytes"
// import "encoding/gob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make().
//
type ApplyMsg struct {
	Index       int
	Command     interface{}
	UseSnapshot bool   // ignore for lab2; only used in lab3
	Snapshot    []byte // ignore for lab2; only used in lab3
}

type LogEntry struct {
	Term    int
	Command interface{}
}

const (
	FOLLOWER = iota
	CANDIDATE
	LEADER
)

const (
	HEARTBEATINTERVAL    = 100
	ELECTIONTIMEOUTFIXED = 400
	// would scale out to 400, cf. the function randomizeTimeout
	ELECTIONTIMEOUTRAND  = 100
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

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := gob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := gob.NewDecoder(r)
	// d.Decode(&rf.xxx)
	// d.Decode(&rf.yyy)
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
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
	CurrentTerm int  // currentTerm, for candidate to update itself
	VoteGranted bool // true means candidate received vote
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	// Your code here (2A, 2B).
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.VoteGranted = false
	if args.Term < rf.CurrentTerm {
		reply.CurrentTerm = rf.CurrentTerm
		return
	}

	reply.CurrentTerm = args.Term
	if args.Term > rf.CurrentTerm {
		rf.CurrentTerm = args.Term
		rf.goBackToFollower()
	}

	// Suspicious Point. I figure the second condition is necessary to avoid the loss of previous grant reply
	if rf.VotedFor == -1 || rf.VotedFor == args.CandidateID {
		idx := len(rf.Logs) - 1
		localLastLogTerm := rf.Logs[idx].Term
		localLastLogIndex := rf.logIdxLocal2Global(idx)
		if args.LastLogTerm > localLastLogTerm || args.Term == localLastLogTerm && args.LastLogIndex >= localLastLogIndex {
			//rf.state = FOLLOWER
			rf.VotedFor = args.CandidateID
			reply.VoteGranted = true
			rf.resetTimerCh <- struct{}{}
		}
	}
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
	reply := &RequestVoteReply{}
	ok := rf.sendRequestVote(sendTo, args, reply)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if !ok || rf.state!=CANDIDATE || reply.CurrentTerm < rf.CurrentTerm {
		return
	}
	if reply.CurrentTerm > rf.CurrentTerm {
		rf.goBackToFollower()
		rf.resetTimerCh <- struct{}{}
		return
	}

	// it indicated ok == true && reply.CurrentTerm == rf.CurrentTerm && rf.state==CANDIDATE
	rf.VotedFor++
	if rf.VotedFor > len(rf.peers)/2 {
		rf.state = LEADER
		// todo
		go rf.heartBeatDaemon()
		rf.resetTimerCh <- struct{}{}
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
	rf.mu.Lock()
	defer rf.mu.Unlock()

	if args.Term < rf.CurrentTerm {
		reply.Term = rf.CurrentTerm
		reply.Success = false
		return
	}

	reply.Term = args.Term
	if rf.CurrentTerm < args.Term {
		rf.CurrentTerm = args.Term
	}

	// Suspicious Point. I figure a peer MUST rewind back to the FOLLOWER state if it receives
	// Append Queries with term >= rf.CurrentTerm
	rf.goBackToFollower()
	rf.resetTimerCh <- struct{}{}

	lastLogIndex := rf.logIdxLocal2Global(len(rf.Logs) - 1)
	if args.PrevLogIndex > lastLogIndex || rf.Logs[args.PrevLogIndex].Term != args.PrevLogTerm {
		reply.Success = false
		if lastLogIndex < args.PrevLogIndex {
			reply.ConflictIndex = lastLogIndex + 1
		} else {
			reply.ConflictIndex = args.PrevLogIndex
		}
		return
	}

	reply.Success = true
	i := rf.logIdxGlobal2Local(args.PrevLogIndex + 1)
	j := 0
	for ; i < len(rf.Logs) && j < len(args.Entries); i, j = i+1, j+1 {
		if rf.Logs[i].Term != args.Entries[j].Term {
			break
		}
	}

	if j < len(args.Entries) {
		rf.Logs = append(rf.Logs[:i], args.Entries[j:]...)
	}

	if args.LeaderCommit > rf.commitIndex {
		// Suspicious Point, last of new entries?.
		rf.commitIndex = min(args.LeaderCommit, rf.logIdxLocal2Global(len(rf.Logs)-1))
		rf.commitCh <- struct{}{}
	}
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
	isLeader := true

	// Your code here (2B).

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
	// first log index is 1, thus we need a dummy log with index 0
	rf.Logs = make([]LogEntry, 1)
	rf.Logs[0] = LogEntry{
		Term:    0,
		Command: nil,
	}
	rf.lastApplied = 0
	rf.commitIndex = 0
	rf.nextIndex = make([]int, len(peers))
	rf.matchIndex = make([]int, len(peers))

	rf.state = FOLLOWER
	rf.timer = time.NewTimer(rf.randomizeTimeout())
	rf.seed = rand.NewSource(int64(rf.me))
	rf.resetTimerCh = make(chan struct{})
	rf.commitCh = make(chan struct{}, 100)
	rf.applyCh = applyCh

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

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
	voteReq := &RequestVoteArgs{}
	rf.CurrentTerm++
	rf.state = CANDIDATE
	rf.VotedFor = rf.me
	rf.voteCount++
	localIdx := len(rf.Logs)-1
	voteReq.CandidateID = rf.me
	voteReq.LastLogIndex = rf.logIdxLocal2Global(localIdx)
	voteReq.LastLogTerm = rf.Logs[localIdx].Term
	voteReq.Term = rf.CurrentTerm
	peerNum := len(rf.peers)

	for i:=0; i<peerNum; i++ {
		if rf.me != i {
			continue
		}
		go rf.sendRequestVoteAndProcess(voteReq, i)
	}
}

func (rf *Raft) heartBeatDaemon(){

}

func (rf *Raft) broadCastAppendLogs() {

}

func (rf *Raft) applyLogEntryDaemon() {

}

// helper function below
func min(a int, b int) int {
	if a < b {
		return a
	} else {
		return b
	}
}

func (rf *Raft) logIdxLocal2Global(localIdx int) int {
	return localIdx
}

func (rf *Raft) logIdxGlobal2Local(globalIdx int) int {
	return globalIdx
}

func (rf *Raft) resetTimer() {
	if !rf.timer.Stop() {
		<-rf.timer.C
	}
	rf.timer.Reset(rf.randomizeTimeout())
}

func (rf *Raft) randomizeTimeout() time.Duration {
	return time.Millisecond * time.Duration(ELECTIONTIMEOUTFIXED+rand.New(rf.seed).Intn(ELECTIONTIMEOUTRAND)*4)
}

// util function, must be called within critical section
// sending info to the Channel resetTimer often follows this function
func (rf *Raft) goBackToFollower(){
	rf.state = FOLLOWER
	rf.VotedFor = -1
}
