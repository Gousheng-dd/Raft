package main

import (
	"LRaft/constdef"
	"LRaft/model"
	"LRaft/util"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

var ck *Clerk

func main() {
	servers := []string{
		constdef.Server0,
		constdef.Server1,
		constdef.Server2,
		constdef.Server3,
		constdef.Server4,
	}
	ck = MakeClerk(servers)
	http.HandleFunc("/ping", Ping)
	http.HandleFunc("/kvget", KVGet)
	http.HandleFunc("/kvput", KVPut)
	http.HandleFunc("/kvappend", KVAppend)
	http.ListenAndServe(":"+constdef.KVClientPort, nil)
}

func KVGet(w http.ResponseWriter, r *http.Request) {
	params := r.URL.Query()
	key, exist := params["key"]
	if !exist || len(key) == 0 {
		fmt.Fprintln(w, "missing key")

		return
	}
	res := ck.Get(key[0])
	fmt.Fprintln(w, res)
}

func KVPut(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("read body err, %v\n", err)

		return
	}
	var putRequest model.PutAppend
	err = json.Unmarshal(body, &putRequest)
	if err != nil {
		fmt.Fprintln(w, "unmarshal from request error, err:", err.Error())

		return
	}
	ck.PutAppend(putRequest.Key, putRequest.Value, "put")
	fmt.Fprintln(w, "Put {key:", putRequest.Key, ", value:", putRequest.Value, "} successfully")

	return
}

func KVAppend(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		fmt.Printf("read body err, %v\n", err)

		return
	}
	var putRequest model.PutAppend
	err = json.Unmarshal(body, &putRequest)
	if err != nil {
		fmt.Fprintln(w, "unmarshal from request error, err: ", err.Error())

		return
	}
	util.DPrintf("[Get HTTP Request: Append %+v]", putRequest)
	ck.PutAppend(putRequest.Key, putRequest.Value, "append")
	fmt.Fprintln(w, "Append {key:", putRequest.Key, ", value:", putRequest.Value, "} successfully")

	return
}

func Ping(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "pong")

	return
}
