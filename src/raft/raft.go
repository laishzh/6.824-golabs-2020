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
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"../labrpc"
)

// import "bytes"
// import "../labgob"

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}

type RaftState int

const (
	Follower  RaftState = 0
	Candidate           = 1
	Leader              = 2
)

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	// Your data here (2A, 2B, 2C).
	// Look at the paper's Figure 2 for a description of what
	// state a Raft server must maintain.
	state       RaftState
	currentTerm int
	voteFor     int

	// election timer
	timer *time.Timer
}

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {

	rf.mu.Lock()
	defer rf.mu.Unlock()
	var term int
	var isleader bool
	// Your code here (2A).
	term = rf.currentTerm
	if rf.state == Leader {
		isleader = true
	}

	if rf.state == Follower {
		isleader = false
	}

	if rf.state == Candidate {
		isleader = false
	}

	DPrintf("[%d]GetState, term: %d, isleader: %v", rf.me, term, isleader)
	return term, isleader
}

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
}

//
// example RequestVote RPC arguments structure.
// field names must start with capital letters!
//
type RequestVoteArgs struct {
	// Your data here (2A, 2B).

	// Candidate's term
	Term int
	// Candidate requesting vote
	CandidateID int
}

//
// example RequestVote RPC reply structure.
// field names must start with capital letters!
//
type RequestVoteReply struct {
	// Your data here (2A).

	// Reply its currentTerm, for candidate to update itself
	Term int
	// True means candidate received vote
	VoteGranted bool
}

//
// example RequestVote RPC handler.
//
func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	DPrintf("[%d]RequestVote", rf.me)
	// Your code here (2A, 2B).

	DPrintf("[%d]Received VoteReq from [%d], curTerm(%d) vs candTerm(%d)", rf.me, args.CandidateID, rf.currentTerm, args.Term)
	myTerm := rf.currentTerm
	reply.Term = myTerm
	reply.VoteGranted = false
	if args.Term > myTerm {
		rf.mu.Lock()
		rf.state = Follower
		rf.currentTerm = args.Term
		rf.mu.Unlock()
		rf.ResetElectionTimeout()
		reply.VoteGranted = true

	}

	if reply.VoteGranted {
		DPrintf("[%d] votes to [%d]", rf.me, args.CandidateID)
	} else {
		DPrintf("[%d] doesn't vote to [%d]", rf.me, args.CandidateID)
	}

	return
}

//
// example code to send a RequestVote RPC to a server.
// server is the index of the target server in rf.peers[].
// expects RPC arguments in args.
// fills in *reply with RPC reply, so caller should
// pass &reply.
// the types of the args and reply passed to Call() must be
// the same as the types of the arguments declared in the
// handler function (including whether they are pointers).
//
// The labrpc package simulates a lossy network, in which servers
// may be unreachable, and in which requests and replies may be lost.
// Call() sends a request and waits for a reply. If a reply arrives
// within a timeout interval, Call() returns true; otherwise
// Call() returns false. Thus Call() may not return for a while.
// A false return can be caused by a dead server, a live server that
// can't be reached, a lost request, or a lost reply.
//
// Call() is guaranteed to return (perhaps after a delay) *except* if the
// handler function on the server side does not return.  Thus there
// is no need to implement your own timeouts around Call().
//
// look at the comments in ../labrpc/labrpc.go for more details.
//
// if you're having trouble getting RPC to work, check that you've
// capitalized all field names in structs passed over RPC, and
// that the caller passes the address of the reply struct with &, not
// the struct itself.
//
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	DPrintf("[%d]sendRequestVote to [%d]", rf.me, server)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)
	return ok
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
	DPrintf("[%d]Raft.Start", rf.me)
	index := -1
	term := -1
	isLeader := true

	// Your code here (2B).

	return index, term, isLeader
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
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

// StartElection
func (rf *Raft) StartElection() {
	DPrintf("[%d]Start an election.", rf.me)
	defer DPrintf("[%d]Election ends.", rf.me)
	rf.mu.Lock()
	voteArgs := RequestVoteArgs{
		Term:        rf.currentTerm,
		CandidateID: rf.me,
	}
	rf.mu.Unlock()

	total := len(rf.peers)
	voteGranted := 0
	voteReceived := 0
	for i := 0; i < total; i++ {
		if i == rf.me {
			continue
		}
		var voteReply RequestVoteReply
		b := rf.sendRequestVote(i, &voteArgs, &voteReply)
		if b {
			voteReceived++
			if voteReply.VoteGranted {
				voteGranted++
			}
		} else {
			DPrintf("[%d]fail to get resp from [%d]", rf.me, i)
		}
		// if voteGranted >= total/2 {
		// 	DPrintf("[%d]Got %d/%d votes, break", rf.me, voteGranted+1, total)
		// 	break
		// }
	}

	if voteGranted >= total/2 {
		rf.mu.Lock()
		rf.state = Leader
		rf.mu.Unlock()
		DPrintf("[%d]become leader", rf.me)
	}
}

func (rf *Raft) ResetElectionTimeout() {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rd := rand.Intn(150) + 150
	rf.timer.Reset(time.Duration(rd) * time.Millisecond)
	DPrintf("[%d]ResetTimtout %d ms", rf.me, rd)

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
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	// Your initialization code here (2A, 2B, 2C).

	rand.Seed(time.Now().UnixNano())

	rf.state = Follower
	rf.currentTerm = 0

	rd := 150 + rand.Intn(150)
	d := time.Duration(time.Duration(rd) * time.Millisecond)
	rf.timer = time.NewTimer(d)

	go func(rf *Raft) {
		for {
			<-rf.timer.C
			DPrintf("[%d]timer timeout!", rf.me)

			rf.mu.Lock()
			if rf.state == Follower {
				rf.state = Candidate
				rf.currentTerm++
				rf.mu.Unlock()
				rf.StartElection()
			} else {
				rf.mu.Unlock()
			}
			rf.ResetElectionTimeout()

		}
	}(rf)

	DPrintf("[%d]Make raft", rf.me)

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}
