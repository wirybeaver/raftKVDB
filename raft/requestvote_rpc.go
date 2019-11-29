package raft

import "fmt"

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
	// Your code here (2A, 2B).
	DPrintf("%d<-%d, Start Vote\nargs=%s\nraftState=%s\n",
		rf.me, args.CandidateID, args.str(), rf.str())
	rf.mu.Lock()
	defer rf.mu.Unlock()

	reply.VoteGranted = false
	if args.Term < rf.CurrentTerm {
		reply.Term = rf.CurrentTerm
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

	// The second condition is necessary to avoid the loss of previous grant reply
	oldVotedFor := rf.VotedFor
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
			rf.resetTimerCh <- struct{}{}
		}
	}
	if change {
		rf.persistState()
	}
	DPrintf("%d<-%d, End Vote. VotedFor=%d->%d\nargs=%s\nraftState=%s\n",
		rf.me, args.CandidateID, oldVotedFor, rf.VotedFor, args.str(), rf.str())
}

func (req *RequestVoteArgs) str() string {
	return fmt.Sprintf("VoteRPC ArgsTerm=%d, CandID=%d, LastLogIdx=%d, LastLogTerm=%d", req.Term, req.CandidateID, req.LastLogIndex, req.LastLogTerm)
}

func (reply *RequestVoteReply) str() string {
	return fmt.Sprintf("VoteRPC ReplyTerm=%d, VoteGrandted=%t", reply.Term, reply.VoteGranted)
}
