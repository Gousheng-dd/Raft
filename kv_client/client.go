package main

import (
	"crypto/rand"
	"math/big"
	mathrand "math/rand"

	"LRaft/constdef"
	"LRaft/kv_server/myrpc"
	"LRaft/model"
)

type Clerk struct {
	servers        []string
	clientId       int64
	requestId      int
	recentLeaderId int
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []string) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	// You'll have to add code here.
	ck.clientId = nrand()
	ck.recentLeaderId = GetRandomServer(len(ck.servers))
	return ck
}

func GetRandomServer(length int) int {
	return mathrand.Intn(length)
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
	ck.requestId++
	requestId := ck.requestId
	server := ck.recentLeaderId
	args := model.GetArgs{Key: key, ClientId: ck.clientId, RequestId: requestId}

	for {
		reply := model.GetReply{}
		err := myrpc.Call(ck.servers[server]+":"+constdef.KVServerPort, "KVServer.Get", &args, &reply)
		if err != nil || reply.Err == constdef.ErrWrongLeader {
			server = (server + 1) % len(ck.servers)
			continue
		}

		if reply.Err == constdef.ErrNoKey {
			return ""
		}
		if reply.Err == constdef.OK {
			ck.recentLeaderId = server
			//ck.requestId++
			return reply.Value
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
	ck.requestId++
	requestId := ck.requestId
	server := ck.recentLeaderId
	for {
		args := model.PutAppendArgs{Key: key, Value: value, Op: op, ClientId: ck.clientId, RequestId: requestId}
		reply := model.PutAppendReply{}

		err := myrpc.Call(ck.servers[server]+":"+constdef.KVServerPort, "KVServer.PutAppend", &args, &reply)
		if err != nil || reply.Err == constdef.ErrWrongLeader {
			server = (server + 1) % len(ck.servers)
			continue
		}
		if reply.Err == constdef.OK {
			ck.recentLeaderId = server
			//ck.requestId++
			return
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, "Put")
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, "Append")
}
