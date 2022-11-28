package raft

import (
	"LRaft/constdef"
	"LRaft/kv_server/myrpc"
	"LRaft/model"
	"LRaft/util"
	"time"
)

func (rf *Raft) AppendEntriesTicker() {
	for rf.killed() == false {
		time.Sleep(constdef.HeartbeatTimeout * time.Millisecond)
		rf.mu.Lock()
		if rf.state == constdef.Leader {
			rf.mu.Unlock()
			rf.LeaderAppendEntries()
		} else {
			rf.mu.Unlock()
		}
	}
}

func (rf *Raft) LeaderAppendEntries() {
	// send to every server to replicate logs to them
	for index := range rf.peers {
		if index == rf.me {
			continue
		}
		// parallel replicate logs to sever
		go func(server int) {
			rf.mu.Lock()
			if rf.state != constdef.Leader {
				rf.mu.Unlock()
				return
			}

			aeArgs := model.AppendEntriesArgs{}

			if rf.getLastIndex() >= rf.nextIndex[server] {
				util.DPrintf("[LeaderAppendEntries]Leader %d (term %d) to server %d, index %d --- %d", rf.me, rf.currentTerm, server, rf.nextIndex[server], rf.getLastIndex())
				entriesNeeded := make([]model.Entry, 0)
				entriesNeeded = append(entriesNeeded, rf.log[rf.nextIndex[server]:]...)
				prevLogIndex, prevLogTerm := rf.getPrevLogInfo(server)
				aeArgs = model.AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: prevLogIndex,
					PrevLogTerm:  prevLogTerm,
					Entries:      entriesNeeded,
					LeaderCommit: rf.commitIndex,
				}
			} else {
				// send heart beat
				prevLogIndex, prevLogTerm := rf.getPrevLogInfo(server)
				aeArgs = model.AppendEntriesArgs{
					Term:         rf.currentTerm,
					LeaderId:     rf.me,
					PrevLogIndex: prevLogIndex,
					PrevLogTerm:  prevLogTerm,
					Entries:      []model.Entry{},
					LeaderCommit: rf.commitIndex,
				}
				util.DPrintf("[LeaderSendHeartBeat]Leader %d (term %d) to server %d,nextIndex %d, matchIndex %d, lastIndex %d", rf.me, rf.currentTerm, server, rf.nextIndex[server], rf.matchIndex[server], rf.getLastIndex())
			}
			aeReply := model.AppendEntriesReply{}
			rf.mu.Unlock()

			err := myrpc.Call(rf.peers[server]+":"+constdef.RaftPort, "Raft.AppendEntries", &aeArgs, &aeReply)

			if err != nil {
				util.DPrintf("[Call AppendEntries RPC to server %d error, err: %s]", server, err.Error())

				return
			}
			rf.mu.Lock()
			defer rf.mu.Unlock()

			if rf.state != constdef.Leader {
				return
			}

			if aeReply.Term > rf.currentTerm {
				rf.currentTerm = aeReply.Term
				rf.changeState(constdef.ToFollower, true)
				return
			}

			util.DPrintf("[AppendEntiresRPCGetReturn] Leader %d (term %d) ,from Server %d, prevLogIndex %d", rf.me, rf.currentTerm, server, aeArgs.PrevLogIndex)

			if aeReply.Success {
				util.DPrintf("[AppendEntiresRPC SUCCESS] Leader %d (term %d) ,from Server %d, prevLogIndex %d", rf.me, rf.currentTerm, server, aeArgs.PrevLogIndex)
				rf.matchIndex[server] = aeArgs.PrevLogIndex + len(aeArgs.Entries)
				rf.nextIndex[server] = rf.matchIndex[server] + 1
				rf.updateCommitIndex(constdef.Leader, 0)
			}

			if !aeReply.Success {
				if aeReply.ConflictingIndex != -1 {
					util.DPrintf("[AppendEntiresRPC CONFLICT] Leader %d (term %d) ,from Server %d, prevLogIndex %d, Confilicting %d", rf.me, rf.currentTerm, server, aeArgs.PrevLogIndex, aeReply.ConflictingIndex)
					rf.nextIndex[server] = aeReply.ConflictingIndex
				}
			}

		}(index)

	}
}

func (rf *Raft) AppendEntries(args *model.AppendEntriesArgs, reply *model.AppendEntriesReply) error {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	util.DPrintf("[GetHeartBeat]Sever %d, from Leader %d(term %d), mylastindex %d, leader.preindex %d", rf.me, args.LeaderId, args.Term, rf.getLastIndex(), args.PrevLogIndex)

	// rule 1
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		reply.Success = false
		reply.ConflictingIndex = -1
		return nil
	}

	rf.currentTerm = args.Term
	reply.Term = rf.currentTerm
	reply.Success = true
	reply.ConflictingIndex = -1

	if rf.state != constdef.Follower {
		rf.changeState(constdef.ToFollower, true)
	} else {
		rf.lastResetElectionTime = time.Now()
	}

	// confilict
	if rf.getLastIndex() < args.PrevLogIndex {
		reply.Success = false
		reply.ConflictingIndex = rf.getLastIndex()
		util.DPrintf("[AppendEntries ERROR1]Sever %d ,prevLogIndex %d,Term %d, rf.getLastIndex %d, rf.getLastTerm %d, Confilicing %d", rf.me, args.PrevLogIndex, args.PrevLogTerm, rf.getLastIndex(), rf.getLastTerm(), reply.ConflictingIndex)
		return nil
	} else {
		// term 不同，即对应 index 处可能为其他 leader 同步的 log，但这些 log 并没有被 commit（应被覆盖）
		if rf.getLogTermWithIndex(args.PrevLogIndex) != args.PrevLogTerm {
			reply.Success = false
			tempTerm := rf.getLogTermWithIndex(args.PrevLogIndex)
			for index := args.PrevLogIndex; index >= 0; index-- {
				if rf.getLogTermWithIndex(index) != tempTerm {
					reply.ConflictingIndex = index + 1
					util.DPrintf("[AppendEntries ERROR2]Sever %d ,prevLogIndex %d,Term %d, rf.getLastIndex %d, rf.getLastTerm %d, Confilicing %d", rf.me, args.PrevLogIndex, args.PrevLogTerm, rf.getLastIndex(), rf.getLastTerm(), reply.ConflictingIndex)
					break
				}
			}
			return nil
		}
	}

	//rule 3 & rule 4
	rf.log = append(rf.log[:args.PrevLogIndex+1], args.Entries...)

	//rule 5
	if args.LeaderCommit > rf.commitIndex {
		rf.updateCommitIndex(constdef.Follower, args.LeaderCommit)
	}
	util.DPrintf("[FinishHeartBeat]Server %d, from leader %d(term %d), me.lastIndex %d", rf.me, args.LeaderId, args.Term, rf.getLastIndex())
	return nil
}

func (rf *Raft) getPrevLogInfo(server int) (int, int) {
	newEntryBeginIndex := rf.nextIndex[server] - 1
	return newEntryBeginIndex, rf.getLogTermWithIndex(newEntryBeginIndex)
}

func (rf *Raft) getLogTermWithIndex(globalIndex int) int {
	return rf.log[globalIndex].Term
}
