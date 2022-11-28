package kvstorage

import (
	"time"

	"LRaft/constdef"
	"LRaft/model"
	"LRaft/util"
)

func (kv *KVServer) Get(args *model.GetArgs, reply *model.GetReply) error {
	if kv.killed() {
		reply.Err = constdef.ErrWrongLeader
		return nil
	}

	_, ifLeader := kv.rf.GetState()
	if !ifLeader {
		reply.Err = constdef.ErrWrongLeader
		return nil
	}

	op := model.Op{Operation: "get", Key: args.Key, Value: "", ClientId: args.ClientId, RequestId: args.RequestId}

	raftIndex, _, _ := kv.rf.Start(op)
	util.DPrintf("[GET StartToRaft]From Client %d (Request %d) To Server %d, key %v, raftIndex %d", args.ClientId, args.RequestId, kv.me, op.Key, raftIndex)

	// create waitForCh
	kv.mu.Lock()
	chForRaftIndex, exist := kv.waitApplyCh[raftIndex]
	if !exist {
		kv.waitApplyCh[raftIndex] = make(chan model.Op, 1)
		chForRaftIndex = kv.waitApplyCh[raftIndex]
	}
	kv.mu.Unlock()
	// timeout
	select {
	case <-time.After(time.Millisecond * constdef.RaftApplyTimeOut):
		util.DPrintf("[GET TIMEOUT!!!]From Client %d (Request %d) To Server %d, key %v, raftIndex %d", args.ClientId, args.RequestId, kv.me, op.Key, raftIndex)

		_, ifLeader := kv.rf.GetState()
		if kv.ifRequestDuplicate(op.ClientId, op.RequestId) && ifLeader {
			value, exist := kv.ExecuteGetOpOnKVDB(op)
			if exist {
				reply.Err = constdef.OK
				reply.Value = value
			} else {
				reply.Err = constdef.ErrNoKey
				reply.Value = ""
			}
		} else {
			reply.Err = constdef.ErrWrongLeader
		}

	case raftCommitOp := <-chForRaftIndex:
		util.DPrintf("[WaitChanGetRaftApplyMessage<--]Server %d , get Command <-- Index:%d , ClientId %d, RequestId %d, Opreation %v, Key :%v, Value :%v", kv.me, raftIndex, op.ClientId, op.RequestId, op.Operation, op.Key, op.Value)
		if raftCommitOp.ClientId == op.ClientId && raftCommitOp.RequestId == op.RequestId {
			value, exist := kv.ExecuteGetOpOnKVDB(op)
			if exist {
				reply.Err = constdef.OK
				reply.Value = value
			} else {
				reply.Err = constdef.ErrNoKey
				reply.Value = ""
			}
		} else {
			reply.Err = constdef.ErrWrongLeader
		}
	}
	kv.mu.Lock()
	delete(kv.waitApplyCh, raftIndex)
	kv.mu.Unlock()
	return nil
}

func (kv *KVServer) PutAppend(args *model.PutAppendArgs, reply *model.PutAppendReply) error {
	if kv.killed() {
		reply.Err = constdef.ErrWrongLeader
		return nil
	}

	_, ifLeader := kv.rf.GetState()
	if !ifLeader {
		reply.Err = constdef.ErrWrongLeader

		return nil
	}

	op := model.Op{Operation: args.Op, Key: args.Key, Value: args.Value, ClientId: args.ClientId, RequestId: args.RequestId}

	raftIndex, _, _ := kv.rf.Start(op)
	util.DPrintf("[PUTAPPEND StartToRaft]From Client %d (Request %d) To Server %d, key %v, raftIndex %d", args.ClientId, args.RequestId, kv.me, op.Key, raftIndex)

	// create waitForCh
	kv.mu.Lock()
	chForRaftIndex, exist := kv.waitApplyCh[raftIndex]
	if !exist {
		kv.waitApplyCh[raftIndex] = make(chan model.Op, 1)
		chForRaftIndex = kv.waitApplyCh[raftIndex]
	}
	kv.mu.Unlock()

	select {
	case <-time.After(time.Millisecond * constdef.RaftApplyTimeOut):
		util.DPrintf("[TIMEOUT PUTAPPEND !!!!]Server %d , get Command <-- Index:%d , ClientId %d, RequestId %d, Opreation %v, Key :%v, Value :%v", kv.me, raftIndex, op.ClientId, op.RequestId, op.Operation, op.Key, op.Value)
		if kv.ifRequestDuplicate(op.ClientId, op.RequestId) {
			reply.Err = constdef.OK
		} else {
			reply.Err = constdef.ErrWrongLeader
		}

	case raftCommitOp := <-chForRaftIndex:
		util.DPrintf("[WaitChanGetRaftApplyMessage<--]Server %d , get Command <-- Index:%d , ClientId %d, RequestId %d, Opreation %v, Key :%v, Value :%v", kv.me, raftIndex, op.ClientId, op.RequestId, op.Operation, op.Key, op.Value)
		if raftCommitOp.ClientId == op.ClientId && raftCommitOp.RequestId == op.RequestId {
			reply.Err = constdef.OK
		} else {
			reply.Err = constdef.ErrWrongLeader
		}

	}
	kv.mu.Lock()
	delete(kv.waitApplyCh, raftIndex)
	kv.mu.Unlock()
	return nil
}
