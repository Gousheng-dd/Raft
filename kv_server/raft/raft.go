package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	//	"bytes"
	"encoding/gob"
	"sync"
	"sync/atomic"
	"time"

	"LRaft/constdef"
	"LRaft/kv_server/myrpc"
	"LRaft/model"
	"LRaft/util"
)

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu    sync.Mutex // Lock to protect shared access to this peer's state
	peers []string   // RPC address of all peers
	me    int        // this peer's index into peers[]
	dead  int32      // set by Kill()

	currentTerm int
	votedFor    int
	getVoteNum  int
	log         []model.Entry

	commitIndex int
	lastApplied int

	state                 int
	lastResetElectionTime time.Time

	nextIndex  []int
	matchIndex []int

	applyCh chan model.ApplyMsg
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	return rf.currentTerm, rf.state == constdef.Leader
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	if rf.killed() == true {
		return -1, -1, false
	}
	if rf.state != constdef.Leader {
		return -1, -1, false
	} else {
		index := rf.getLastIndex() + 1
		term := rf.currentTerm
		rf.log = append(rf.log, model.Entry{Term: term, Command: command})
		util.DPrintf("[StartCommand] Leader %d get command %v,index %d", rf.me, command, index)
		return index, term, true
	}
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

//
// the service or tester wants to create a Raft server. the ports
// of all the Raft servers (including this one) are in peers[]. this
// server's port is peers[me]. all the servers' peers[] arrays
// have the same order. persister is a place for this server to
// save its persistent state, and also initially holds the most
// recent saved state, if any. applyCh is a channel on which the
// tester or service expects Raft to send ApplyMsg messages.
// Make() must return quickly, so it should start goroutines
// for any long-running work.
//
func Make(peers []string, me int, applyCh chan model.ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.me = me

	rf.mu.Lock()
	rf.state = constdef.Follower
	rf.currentTerm = 0
	rf.getVoteNum = 0
	rf.votedFor = -1
	rf.lastApplied = 0
	rf.commitIndex = 0
	rf.log = []model.Entry{}
	rf.log = append(rf.log, model.Entry{})
	rf.applyCh = applyCh
	rf.mu.Unlock()
	util.DPrintf("Start a raft node of index %d successfully", me)
	// create a rpc listener
	gob.Register(model.Op{})
	go myrpc.RegisterNewServer(rf, constdef.RaftPort)
	util.DPrintf("Register a rpc listener for raft successfully")

	// start ticker goroutine to start elections
	go rf.ElectionTicker()

	go rf.AppendEntriesTicker()

	go rf.ApplyEntriesTicker()

	return rf
}

func (rf *Raft) getLastIndex() int {
	return len(rf.log) - 1
}

func (rf *Raft) getLastTerm() int {
	return rf.log[len(rf.log)-1].Term
}

func (rf *Raft) changeState(howtochange int, resetTime bool) {

	if howtochange == constdef.ToFollower {
		rf.state = constdef.Follower
		rf.votedFor = -1
		rf.getVoteNum = 0
		if resetTime {
			rf.lastResetElectionTime = time.Now()
		}
	}

	if howtochange == constdef.ToCandidate {
		rf.state = constdef.Candidate
		rf.votedFor = rf.me
		rf.getVoteNum = 1
		rf.currentTerm += 1
		rf.JoinElection()
		rf.lastResetElectionTime = time.Now()
	}

	if howtochange == constdef.ToLeader {
		rf.state = constdef.Leader
		rf.votedFor = -1
		rf.getVoteNum = 0

		rf.nextIndex = make([]int, len(rf.peers))
		for i := 0; i < len(rf.peers); i++ {
			rf.nextIndex[i] = rf.getLastIndex() + 1
			util.DPrintf("---------nextIndex for server %d is %d", i, rf.nextIndex[i])
		}

		rf.matchIndex = make([]int, len(rf.peers))
		rf.matchIndex[rf.me] = rf.getLastIndex()
		rf.lastResetElectionTime = time.Now()
		rf.LeaderAppendEntries()
	}
}
