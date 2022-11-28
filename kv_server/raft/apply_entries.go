package raft

import (
	"time"

	"LRaft/constdef"
	"LRaft/model"
	"LRaft/util"
)

func (rf *Raft) ApplyEntriesTicker() {
	// put the committed entry to apply on the state machine
	for rf.killed() == false {
		time.Sleep(constdef.ApplyTimeout * time.Millisecond)
		rf.mu.Lock()

		if rf.lastApplied >= rf.commitIndex {
			rf.mu.Unlock()
			continue
		}

		Messages := make([]model.ApplyMsg, 0)
		for rf.lastApplied < rf.commitIndex && rf.lastApplied < rf.getLastIndex() {
			rf.lastApplied += 1
			Messages = append(Messages, model.ApplyMsg{
				CommandValid: true,
				CommandIndex: rf.lastApplied,
				Command:      rf.getLogWithIndex(rf.lastApplied).Command,
			})
		}
		rf.mu.Unlock()

		for _, messages := range Messages {
			rf.applyCh <- messages
		}
	}
}

func (rf *Raft) updateCommitIndex(role int, leaderCommit int) {
	if role != constdef.Leader {
		if leaderCommit > rf.commitIndex {
			lastNewIndex := rf.getLastIndex()
			if leaderCommit >= lastNewIndex {
				rf.commitIndex = lastNewIndex
			} else {
				rf.commitIndex = leaderCommit
			}
		}
		util.DPrintf("[CommitIndex] Fllower %d commitIndex %d", rf.me, rf.commitIndex)
		return
	}

	if role == constdef.Leader {
		for index := rf.getLastIndex(); index >= rf.commitIndex+1; index-- {
			sum := 0
			for i := 0; i < len(rf.peers); i++ {
				if i == rf.me {
					sum += 1
					continue
				}
				if rf.matchIndex[i] >= index {
					sum += 1
				}
			}

			if sum >= len(rf.peers)/2+1 && rf.getLogTermWithIndex(index) == rf.currentTerm {
				rf.commitIndex = index
				break
			}

		}
		util.DPrintf("[CommitIndex] Leader %d(term%d) commitIndex %d", rf.me, rf.currentTerm, rf.commitIndex)
		return
	}

}

func (rf *Raft) getLogWithIndex(globalIndex int) model.Entry {

	return rf.log[globalIndex]
}
