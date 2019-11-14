package raftkv

import (
	"log"
	"raftKVDB/labgob"
	"raftKVDB/labrpc"
	"raftKVDB/raft"
	"sync"
	"time"
)

const Debug = 0

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}


type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	CmdType int
	Key string
	Val string
	ClientID uint64
	CmdID uint64
}

type DoneMsg struct {
	ClientID uint64
	CmdID uint64
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	dupMap map[uint64]uint64
	applyStub map[int]chan DoneMsg
	kvdb map[string]string
}


func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	// Your code here.

	_, isLeader := kv.rf.GetState()
	reply.WrongLeader=!isLeader

	opDone := kv.seenCmd(args.ClientID, args.CmdID)
	if !opDone && isLeader {
		operation := Op{
			CmdType: GET,
			Key: args.Key,
			ClientID: args.ClientID,
			CmdID: args.CmdID,
		}

		opDone = kv.enterOperation(operation)
	}

	if opDone {
		kv.mu.Lock()
		val,ok := kv.kvdb[args.Key]
		kv.mu.Unlock()
		if ok{
			reply.Err = OK
			reply.Value = val
		} else {
			reply.Err = ErrNoKey
		}
	} else {
		reply.Err = ErrFail
	}
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. you are not required to do anything
// in Kill(), but it might be convenient to (for example)
// turn off debug output from this instance.
//
func (kv *KVServer) Kill() {
	kv.rf.Kill()
	// Your code here, if desired.
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	labgob.Register(Op{})

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.

	return kv
}

func (kv *KVServer) enactDaemon(){
	for {
		select{
		case appliedMsg := <-kv.applyCh :
			if appliedMsg.CommandValid {
				operation := appliedMsg.Command.(Op)
				kv.mu.Lock()

				if recordedCmdID, ok := kv.dupMap[operation.ClientID]; !ok || recordedCmdID < operation.CmdID {
					if operation.CmdType==PUTAPPEND {
						kv.kvdb[operation.Key] = operation.Val
					}
					kv.dupMap[operation.ClientID] = operation.ClientID
				}

				if _, ok := kv.applyStub[appliedMsg.CommandIndex]; !ok {
					kv.applyStub[appliedMsg.CommandIndex] = make(chan DoneMsg, 1)
				}
				kv.applyStub[appliedMsg.CommandIndex] <- DoneMsg{
					ClientID : operation.ClientID,
					CmdID : operation.CmdID,
				}
				kv.mu.Unlock()
			}
		}
	}
}

func (kv *KVServer) enterOperation(operation Op) bool{
	raftIdx, _, isLeader :=kv.rf.Start(operation)
	if !isLeader {
		return false
	}
	kv.mu.Lock()
	if _, ok := kv.applyStub[raftIdx]; !ok {
		kv.applyStub[raftIdx]=make(chan DoneMsg, 1)
	}
	doneCh := kv.applyStub[raftIdx]
	kv.mu.Unlock()

	select {
	case doneMsg := <-doneCh:
		if doneMsg.ClientID == operation.ClientID && doneMsg.CmdID == operation.CmdID {
			close(doneCh)
			kv.mu.Lock()
			delete(kv.applyStub, raftIdx)
			kv.mu.Unlock()
			return true
		} else {
			doneCh <- doneMsg
		}
	case <-time.After(WAITRAFTINTERVAL * time.Millisecond):
	}
	return false
}

func (kv *KVServer) seenCmd(clientID uint64, cmdID uint64) bool{
	kv.mu.Lock()
	lastCmd,ok := kv.dupMap[clientID]
	kv.mu.Unlock()
	return ok && cmdID<=lastCmd
}
