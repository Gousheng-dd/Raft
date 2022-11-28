package kvstorage

import (
	"sync"
	"sync/atomic"

	"LRaft/constdef"
	"LRaft/kv_server/myrpc"
	"LRaft/kv_server/raft"
	"LRaft/model"
	"LRaft/util"
)

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan model.ApplyMsg
	dead    int32 // set by Kill()

	kvDB          map[string]string
	waitApplyCh   map[int]chan model.Op // index(raft) -> chan
	lastRequestId map[int64]int         // clientid -> requestID
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
//
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
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
func StartKVServer(servers []string, me int) *KVServer {
	kv := new(KVServer)
	kv.me = me

	kv.applyCh = make(chan model.ApplyMsg)
	kv.rf = raft.Make(servers, me, kv.applyCh)

	kv.kvDB = make(map[string]string)
	kv.waitApplyCh = make(map[int]chan model.Op)
	kv.lastRequestId = make(map[int64]int)

	util.DPrintf("Start a kv_server of index %d successfully", me)
	// create a rpc listener for kv_client
	go myrpc.RegisterNewServer(kv, constdef.KVServerPort)
	util.DPrintf("Register a rpc listener for kv_client successfully")

	go kv.ReadRaftApplyCommandLoop()

	return kv
}

func (kv *KVServer) ReadRaftApplyCommandLoop() {
	for message := range kv.applyCh {
		// listen to every command applied by its raft ,delivery to relative RPC Handler
		kv.GetCommandFromRaft(message)
	}
}

func (kv *KVServer) GetCommandFromRaft(message model.ApplyMsg) {
	op := message.Command.(model.Op)
	util.DPrintf("[RaftApplyCommand]Server %d , Got Command --> Index:%d , ClientId %d, RequestId %d, Opreation %v, Key :%v, Value :%v", kv.me, message.CommandIndex, op.ClientId, op.RequestId, op.Operation, op.Key, op.Value)

	// State Machine (KVServer solute the duplicate problem)
	// duplicate command will not be exed
	if !kv.ifRequestDuplicate(op.ClientId, op.RequestId) {
		// execute command
		if op.Operation == "put" {
			kv.ExecutePutOpOnKVDB(op)
		}
		if op.Operation == "append" {
			kv.ExecuteAppendOpOnKVDB(op)
		}
	}

	// Send message to the chan of op.ClientId
	kv.SendMessageToWaitChan(op, message.CommandIndex)
}

func (kv *KVServer) SendMessageToWaitChan(op model.Op, raftIndex int) bool {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	ch, exist := kv.waitApplyCh[raftIndex]
	if exist {
		util.DPrintf("[RaftApplyMessageSendToWaitChan-->]Server %d , Send Command --> Index:%d , ClientId %d, RequestId %d, Opreation %v, Key :%v, Value :%v", kv.me, raftIndex, op.ClientId, op.RequestId, op.Operation, op.Key, op.Value)
		ch <- op
	}
	return exist
}

func (kv *KVServer) ExecuteGetOpOnKVDB(op model.Op) (string, bool) {
	kv.mu.Lock()
	value, exist := kv.kvDB[op.Key]
	kv.mu.Unlock()
	if !kv.ifRequestDuplicate(op.ClientId, op.RequestId) {
		kv.mu.Lock()
		kv.lastRequestId[op.ClientId] = op.RequestId
		kv.mu.Unlock()
	}

	if exist {
		util.DPrintf("[KVServerExeGET----]ClientId :%d ,RequestID :%d ,Key : %v, value :%v", op.ClientId, op.RequestId, op.Key, value)
	} else {
		util.DPrintf("[KVServerExeGET----]ClientId :%d ,RequestID :%d ,Key : %v, But No KEY!!!!", op.ClientId, op.RequestId, op.Key)
	}
	return value, exist
}

func (kv *KVServer) ExecutePutOpOnKVDB(op model.Op) {

	kv.mu.Lock()
	kv.kvDB[op.Key] = op.Value
	kv.lastRequestId[op.ClientId] = op.RequestId
	util.DPrintf("[after PUT,kv_storage:%+v]", kv.kvDB)
	kv.mu.Unlock()

	util.DPrintf("[KVServerExePUT----]ClientId :%d ,RequestID :%d ,Key : %v, value : %v", op.ClientId, op.RequestId, op.Key, op.Value)
}

func (kv *KVServer) ExecuteAppendOpOnKVDB(op model.Op) {

	kv.mu.Lock()
	value, exist := kv.kvDB[op.Key]
	if exist {
		kv.kvDB[op.Key] = value + op.Value
	} else {
		kv.kvDB[op.Key] = op.Value
	}
	kv.lastRequestId[op.ClientId] = op.RequestId
	util.DPrintf("[after Append,kv_storage:%+v]", kv.kvDB)
	kv.mu.Unlock()

	util.DPrintf("[KVServerExeAPPEND-----]ClientId :%d ,RequestID :%d ,Key : %v, value : %v", op.ClientId, op.RequestId, op.Key, op.Value)
}

func (kv *KVServer) ifRequestDuplicate(newClientId int64, newRequestId int) bool {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	// return true if message is duplicate
	lastRequestId, ifClientInRecord := kv.lastRequestId[newClientId]
	if !ifClientInRecord {
		return false
	}
	return newRequestId <= lastRequestId
}

func (kv *KVServer) GetRaftState() *model.RaftState {
	currentTerm, isLeader := kv.rf.GetState()
	return &model.RaftState{
		CurrentTerm: currentTerm,
		IsLeader:    isLeader,
	}
}
