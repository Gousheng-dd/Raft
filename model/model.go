package model

type Err string

// model define for raft
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type Entry struct {
	Term    int
	Command interface{}
}

// Append Entries RPC structure
type AppendEntriesArgs struct {
	Term         int
	LeaderId     int
	PrevLogIndex int
	PrevLogTerm  int
	Entries      []Entry
	LeaderCommit int
}

type AppendEntriesReply struct {
	Term             int
	Success          bool
	ConflictingIndex int // optimizer func for find the nextIndex
}
type RequestVoteArgs struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

// model define for kv_server and kv_client
type Op struct {
	Operation string // "get" "put" "append"
	Key       string
	Value     string
	ClientId  int64
	RequestId int
}

// Put or Append
type PutAppendArgs struct {
	Key       string
	Value     string
	Op        string // "Put" or "Append"
	ClientId  int64
	RequestId int
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Key       string
	ClientId  int64
	RequestId int
}

type GetReply struct {
	Err   Err
	Value string
}

// http model for client
type PutAppend struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type RaftState struct {
	CurrentTerm int  `json:"current_term"`
	IsLeader    bool `json:"is_leader"`
}
