package constdef

// constance define for raft
const (
	Follower    = 0
	Candidate   = 1
	Leader      = 2
	ToFollower  = 0
	ToCandidate = 1
	ToLeader    = 2

	MaxElectionTimeout = 100
	MinElectionTimeout = 50

	HeartbeatTimeout = 27
	ApplyTimeout     = 28

	RaftPort = "1234"

	KVStoragePort = "8080"
)

// constance define for kv_server and kv_client
const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"

	RaftApplyTimeOut = 500 // ms

	KVServerPort = "5678"

	KVClientPort = "8080"
)

// server rpc address
const (
	Server0 = "server0"
	Server1 = "server1"
	Server2 = "server2"
	Server3 = "server3"
	Server4 = "server4"
)
