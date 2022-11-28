package raft

import (
	"math/rand"
	"time"

	"LRaft/constdef"
	"LRaft/kv_server/myrpc"
	"LRaft/model"
	"LRaft/util"
)

// The ticker go routine starts a new election if this peer hasn't received
// heartsbeats recently.
func (rf *Raft) ElectionTicker() {
	for rf.killed() == false {
		nowTime := time.Now()
		timet := getRand(int64(rf.me))
		time.Sleep(time.Duration(timet) * time.Millisecond)
		rf.mu.Lock()
		if rf.lastResetElectionTime.Before(nowTime) && rf.state != constdef.Leader {
			rf.changeState(constdef.ToCandidate, true)
		}
		rf.mu.Unlock()

	}
}

// get a random timeout interval in [MinElectionTimeout,MaxElectionTimeout)
func getRand(server int64) int {
	rand.Seed(time.Now().Unix() + server)
	return rand.Intn(constdef.MaxElectionTimeout-constdef.MinElectionTimeout) + constdef.MinElectionTimeout
}

func (rf *Raft) JoinElection() {
	for index := range rf.peers {
		if index == rf.me {
			continue
		}

		go func(server int) {
			rf.mu.Lock()
			rvArgs := model.RequestVoteArgs{
				Term:         rf.currentTerm,
				CandidateId:  rf.me,
				LastLogIndex: rf.getLastIndex(),
				LastLogTerm:  rf.getLastTerm(),
			}
			rvReply := model.RequestVoteReply{}
			rf.mu.Unlock()
			// waiting code should free lock first.
			err := myrpc.Call(rf.peers[server]+":"+constdef.RaftPort, "Raft.RequestVote", &rvArgs, &rvReply)
			if err != nil {
				util.DPrintf("[Call RequestVote RPC to server %d error, err: %s]", server, err.Error())

				return
			}
			rf.mu.Lock()
			if rf.state != constdef.Candidate || rvArgs.Term < rf.currentTerm {
				rf.mu.Unlock()
				return
			}

			if rvReply.VoteGranted == true && rf.currentTerm == rvArgs.Term {
				rf.getVoteNum += 1
				if rf.getVoteNum >= len(rf.peers)/2+1 {
					util.DPrintf("[LeaderSuccess+++++] %d got votenum: %d, needed >= %d, become leader for term %d", rf.me, rf.getVoteNum, len(rf.peers)/2+1, rf.currentTerm)
					rf.changeState(constdef.ToLeader, true)
					rf.mu.Unlock()
					return
				}
				rf.mu.Unlock()
				return
			}

			if rvReply.Term > rvArgs.Term {
				if rf.currentTerm < rvReply.Term {
					rf.currentTerm = rvReply.Term
				}
				rf.changeState(constdef.ToFollower, false)
				rf.mu.Unlock()
				return
			}

			rf.mu.Unlock()
			return
		}(index)
	}
}

func (rf *Raft) RequestVote(args *model.RequestVoteArgs, reply *model.RequestVoteReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()

	//rule 1 ------------
	if args.Term < rf.currentTerm {
		util.DPrintf("[ElectionReject++++++]Server %d reject %d, MYterm %d, candidate term %d", rf.me, args.CandidateId, rf.currentTerm, args.Term)
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
		return nil
	}

	reply.Term = rf.currentTerm

	if args.Term > rf.currentTerm {
		util.DPrintf("[ElectionToFollower++++++]Server %d(term %d) into follower,candidate %d(term %d) ", rf.me, rf.currentTerm, args.CandidateId, args.Term)
		rf.currentTerm = args.Term
		rf.changeState(constdef.ToFollower, false)
	}

	//rule 2 ------------
	if rf.UpToDate(args.LastLogIndex, args.LastLogTerm) == false {
		util.DPrintf("[ElectionReject+++++++]Server %d reject %d, UpToDate", rf.me, args.CandidateId)
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
		return nil
	}

	// arg.Term == rf.currentTerm
	if rf.votedFor != -1 && rf.votedFor != args.CandidateId && args.Term == reply.Term {
		util.DPrintf("[ElectionReject+++++++]Server %d reject %d, Have voter for %d", rf.me, args.CandidateId, rf.votedFor)
		reply.VoteGranted = false
		reply.Term = rf.currentTerm
		return nil
	}
	rf.votedFor = args.CandidateId
	reply.VoteGranted = true
	rf.currentTerm = args.Term
	reply.Term = rf.currentTerm
	rf.lastResetElectionTime = time.Now()
	util.DPrintf("[ElectionSUCCESS+++++++]Server %d voted for %d!", rf.me, args.CandidateId)
	return nil
}

func (rf *Raft) UpToDate(index int, term int) bool {
	lastIndex := rf.getLastIndex()
	lastTerm := rf.getLastTerm()
	return term > lastTerm || (term == lastTerm && index >= lastIndex)
}
