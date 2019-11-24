package raftkv

import (
	"bytes"
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
	CmdType string
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

	//DPrintf("Server=%v, Client=%v, GetCmdID=%v", kv.me, args.ClientID, args.CmdID)
	_, isLeader := kv.rf.GetState()
	reply.WrongLeader=!isLeader
	opDone := kv.seenCmd(args.ClientID, args.CmdID)
	DPrintf("Server=%v, isLeader=%v, Client=%v, GetCmdID=%v, isDupCmd=%v", kv.me, isLeader, args.ClientID, args.CmdID, opDone)
	if !opDone && isLeader==true {
		operation := Op{
			CmdType: GET,
			Key: args.Key,
			ClientID: args.ClientID,
			CmdID: args.CmdID,
		}
		//DPrintf("Server=%v, Client=%v, GetCmdID=%v, EnterOp=%v", kv.me, args.ClientID, args.CmdID, operation)
		opDone = kv.enterOperation(operation)
		//DPrintf("Server=%v, Client=%v, GetCmdID=%v, EnterOpSuccess=%v", kv.me, args.ClientID, args.CmdID, opDone)
	}

	if opDone {
		kv.mu.Lock()
		val,ok := kv.kvdb[args.Key]
		//DPrintf("Server=%v, Client=%v, GetCmdID=%v, Key=%v, Contains=%v, Val=%v", kv.me, args.ClientID, args.CmdID, args.Key, ok, val)
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

	//reply.LeaderID = kv.rf.GetLeaderID()

	//DPrintf("Server=%v, Client=%v, GetCmdID=%v, reply=%v", kv.me, args.ClientID, args.CmdID, reply)
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	// Your code here.
	//DPrintf("Server=%v, Client=%v, PutCmdID=%v", kv.me, args.ClientID, args.CmdID)
	_, isLeader := kv.rf.GetState()
	reply.WrongLeader=!isLeader
	opDone := kv.seenCmd(args.ClientID, args.CmdID)
	DPrintf("Server=%v, isLeader=%v, Client=%v, PutCmdID=%v, isNewCmd=%v", kv.me, isLeader, args.ClientID, args.CmdID, !opDone)
	if !opDone && isLeader==true {
		operation := Op{
			Key: args.Key,
			Val: args.Value,
			ClientID: args.ClientID,
			CmdID: args.CmdID,
		}
		if args.Op == PUT {
			operation.CmdType = PUT
		} else {
			operation.CmdType = APPEND
		}
		//DPrintf("Server=%v, Client=%v, PutCmdID=%v, EnterOp=%v", kv.me, args.ClientID, args.CmdID, operation)
		opDone = kv.enterOperation(operation)
		//DPrintf("Server=%v, Client=%v, PutCmdID=%v, EnterOpSuccess=%v", kv.me, args.ClientID, args.CmdID, opDone)
	}

	if opDone {
		reply.Err = OK
	} else {
		reply.Err = ErrFail
	}

	//reply.LeaderID = kv.rf.GetLeaderID()
	//DPrintf("Server=%v, Client=%v, PutCmdID=%v, reply=%v", kv.me, args.ClientID, args.CmdID, reply)
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

	// You may need initialization code here.
	kv.dupMap = make(map[uint64]uint64)
	kv.applyStub = make(map[int]chan DoneMsg)
	kv.kvdb = make(map[string]string)

	go kv.enactDaemon()

	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	return kv
}

func (kv *KVServer) enactDaemon(){
	for {
		select{
		case appliedMsg := <-kv.applyCh :
			kv.dealWithApplyMsg(&appliedMsg)
		}
	}
}

func (kv *KVServer) dealWithApplyMsg (appliedMsg *raft.ApplyMsg) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	if appliedMsg.CommandValid {
		operation := appliedMsg.Command.(Op)
		//DPrintf("Server=%v, applyCh receive ClientID=%v, QueryID=%v, RaftLogID=%v, Op=%v",
		//	kv.me, operation.ClientID, operation.CmdID, appliedMsg.CommandIndex, operation)
		if recordedCmdID, ok := kv.dupMap[operation.ClientID]; !ok || recordedCmdID < operation.CmdID {
			//DPrintf("Server=%v, Raft ClientID=%v, CmdID=%v, is a new command",
			//	kv.me, operation.ClientID, operation.CmdID)
			if operation.CmdType==PUT {
				kv.kvdb[operation.Key] = operation.Val
			} else if operation.CmdType==APPEND {
				kv.kvdb[operation.Key] += operation.Val
			}
			kv.dupMap[operation.ClientID] = operation.CmdID
		}

		if _, ok := kv.applyStub[appliedMsg.CommandIndex]; !ok {
			kv.applyStub[appliedMsg.CommandIndex] = make(chan DoneMsg, 1)
		}
		ch := kv.applyStub[appliedMsg.CommandIndex]

		// double delete dirty data
		select {
			case <- ch:
			default:
		}

		ch <- DoneMsg{
			ClientID : operation.ClientID,
			CmdID : operation.CmdID,
		}

		if kv.maxraftstate!=-1 && kv.rf.GetStateSize() > kv.maxraftstate {
			go kv.rf.Compact(appliedMsg.CommandIndex, kv.encodeSnapShot())
		}

	} else {
		data := appliedMsg.SnapShot
		kv.decodeSnapShot(data)
	}
}

func (kv *KVServer) enterOperation(operation Op) bool{
	raftIdx, _, isLeader :=kv.rf.Start(operation)
	//DPrintf("Server=%v, isLeader=%v, LogReplica, raftLogId=%v, term=%v, CmdID=%v, Op=%v", kv.me, isLeader, raftIdx, term, operation.CmdID, operation)
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
			DPrintf("Server=%v, ClientID=%v, CmdID=%v, SuccessToEnter at raftLogID=%v", kv.me, operation.ClientID, operation.CmdID, raftIdx)
			return true
		} else {
			doneCh <- doneMsg
		}
	case <-time.After(WAITRAFTINTERVAL * time.Millisecond):
	}
	DPrintf("Server=%v, ClientID=%v, CmdID=%v, FailToEnter at raftLogID=%v", kv.me, operation.ClientID, operation.CmdID, raftIdx)
	return false
}

func (kv *KVServer) seenCmd(clientID uint64, cmdID uint64) bool{
	kv.mu.Lock()
	lastCmd,ok := kv.dupMap[clientID]
	kv.mu.Unlock()
	return ok && cmdID<=lastCmd
}

func (kv *KVServer) encodeSnapShot() []byte {
	w := new (bytes.Buffer)
	e := labgob.NewEncoder(w)

	e.Encode(kv.kvdb)
	e.Encode(kv.dupMap)

	return w.Bytes()
}

func (kv *KVServer) decodeSnapShot(data []byte) {
	r := new(bytes.Buffer)
	d := labgob.NewDecoder(r)
	kv.kvdb = make(map[string]string)
	kv.dupMap = make(map[uint64]uint64)
	d.Decode(&kv.kvdb)
	d.Decode(&kv.dupMap)
}

