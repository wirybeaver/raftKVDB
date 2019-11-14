package raftkv

const (
	OK       = "OK"
	ErrNoKey = "ErrNoKey"
	ErrFail  = "ErrFail"
)

const (
	GET = "Get"
	PUT = "Put"
	APPEND = "Append"
)

const WAITRAFTINTERVAL = 400

type Err string

// Put or Append
type PutAppendArgs struct {
	Key   string
	Value string
	Op    string // "Put" or "Append"
	// You'll have to add definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	ClientID uint64
	CmdID uint64
}

type PutAppendReply struct {
	WrongLeader bool
	Err         Err
}

type GetArgs struct {
	Key string
	// You'll have to add definitions here.
	ClientID uint64
	CmdID uint64
}

type GetReply struct {
	WrongLeader bool
	Err         Err
	Value       string
}
