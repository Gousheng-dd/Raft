package myrpc

import (
	"fmt"
	"net"
	"net/rpc"
	"sync"

	"LRaft/util"
)

type ClientEnds struct {
	rpcClients map[string]*rpc.Client
	mu         sync.Mutex
}

var clientTab ClientEnds

func init() {
	clientTab.rpcClients = make(map[string]*rpc.Client)
}

func NewClient(addr string) *rpc.Client {
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		util.DPrintf("dialing:%s", err.Error())

		return nil
	}

	return client
}

func Call(addr string, serviceMethod string, args interface{}, reply interface{}) error {
	clientTab.mu.Lock()
	client, exist := clientTab.rpcClients[addr]
	if !exist {
		client = NewClient(addr)
		if client == nil {
			clientTab.mu.Unlock()

			return fmt.Errorf("dail tcp %s error", addr)
		}
		clientTab.rpcClients[addr] = client
	}
	clientTab.mu.Unlock()

	return client.Call(serviceMethod, args, reply)
}

func RegisterNewServer(rcvr interface{}, port string) error {
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		util.DPrintf("listen error:", err)

		return fmt.Errorf("listen error:" + err.Error())
	}
	rpc.Register(rcvr)
	rpc.Accept(listener)

	return nil
}
