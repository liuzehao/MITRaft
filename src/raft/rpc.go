package raft

type RequestVoteRequest struct {
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}
type RequestVoteResponse struct {
	Term        int
	VoteGranted bool
}
