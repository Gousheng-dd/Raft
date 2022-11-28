package main

import (
	"LRaft/constdef"
	kvstorage "LRaft/kv_server/kv_storage"
	"LRaft/util"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
)

var kvServer *kvstorage.KVServer

func main() {
	servers := []string{
		constdef.Server0,
		constdef.Server1,
		constdef.Server2,
		constdef.Server3,
		constdef.Server4,
	}
	idx, err := strconv.Atoi(os.Args[1])
	if err != nil {
		fmt.Println("get cmd index error, err: ", err)
		os.Exit(1)

		return
	}
	util.DPrintf("starting a kv_server of index %d ......", idx)
	kvServer = kvstorage.StartKVServer(servers, idx)
	http.HandleFunc("/get_status", GetRaftState)
	http.HandleFunc("/ping", Ping)
	http.ListenAndServe(":"+constdef.KVStoragePort, nil)
}

func GetRaftState(w http.ResponseWriter, r *http.Request) {
	util.DPrintf("[Get http request GetRaftState]")
	raftState := kvServer.GetRaftState()
	bytes, err := json.Marshal(raftState)
	if err != nil {
		fmt.Fprintln(w, "get raft state but json format fail: ", err.Error())

		return
	}
	fmt.Fprintln(w, string(bytes))
}

func Ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "pong")

	return
}
