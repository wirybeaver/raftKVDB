package raftkv

import (
	"raftKVDB/labrpc"
	"sync"
)
import "crypto/rand"
import "math/big"


type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	leadId int
	clientID uint64
	nextSeq uint64
	mu sync.Mutex
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.clientID = uint64(nrand())
	ck.leadId = 0
	ck.nextSeq = 0
	return ck
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) Get(key string) string {
	// You will have to modify this function.
	ck.mu.Lock()
	cmdSeq := ck.nextSeq
	ck.nextSeq++
	ck.mu.Unlock()

	for {
		args := &GetArgs{
			Key: key,
			ClientID: ck.clientID,
			CmdID: cmdSeq,
		}

		reply := &GetReply{}
		ck.mu.Lock()
		DPrintf("Client=%v, ToServer=%v, queryID=%v, GetKey=%v\n", ck.clientID, ck.leadId, args.CmdID, args.Key)
		ok := ck.servers[ck.leadId].Call("KVServer.Get", args, reply)
		//DPrintf("Client=%v, ToServer=%v, queryID=%v, reply=%v, isLeader=%v", ck.clientID, ck.leadId, args.CmdID, reply, !reply.WrongLeader)
		//DPrintf("\n")
		ck.mu.Unlock()

		if ok {
			if reply.Err==OK{
				return reply.Value
			} else if reply.Err==ErrNoKey {
				return ""
			}
		}

		if !ok || reply.WrongLeader {
			ck.mu.Lock()
			ck.leadId = (ck.leadId+1)%len(ck.servers)
			ck.mu.Unlock()
		}
	}
}

//
// shared by Put and Append.
//
// you can send an RPC with code like this:
// ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
//
// the types of args and reply (including whether they are pointers)
// must match the declared types of the RPC handler function's
// arguments. and reply must be passed as a pointer.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	// You will have to modify this function.
	ck.mu.Lock()
	cmdSeq := ck.nextSeq
	ck.nextSeq++
	ck.mu.Unlock()
	for {
		args := &PutAppendArgs{
			Key: key,
			Value: value,
			Op: op,
			ClientID: ck.clientID,
			CmdID: cmdSeq,
		}
		reply := &PutAppendReply{}
		DPrintf("Client=%v, ToServer=%v, queryID=%v, PutAppendKey=%v, PutAppendVal=%v\n", ck.clientID, ck.leadId, args.CmdID, args.Key, args.Value)
		ok := ck.servers[ck.leadId].Call("KVServer.PutAppend", args, reply)
		//DPrintf("Client=%v, ToServer=%v, queryID=%v, reply=%v, isLeader=%v", ck.clientID, ck.leadId, args.CmdID, reply, !reply.WrongLeader)
		//DPrintf("\n")
		if ok {
			if reply.Err==OK {
				return
			}
		}
		if !ok || reply.WrongLeader {
			ck.mu.Lock()
			ck.leadId = (ck.leadId+1)%len(ck.servers)
			ck.mu.Unlock()
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
