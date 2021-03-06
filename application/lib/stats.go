package lib

import (
	"log"
	"os"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/refraction-networking/gotapdance/protobuf"
)

// We would use uint64, but we want to atomically subtract sometimes
type Stats struct {
	logger      *log.Logger
	activeConns int64 // incremented on add, decremented on remove, not reset
	newConns    int64 // new connections since last stats.reset()
	newErrConns int64 // new connections that had some sort of error since last reset()

	activeRegistrations    int64 // Current number of active registrations we have
	newLocalRegistrations  int64 // Current registrations that were picked up from this detector (also included in newRegistrations)
	newApiRegistrations    int64 // Current registrations that we heard about from the API (also included in newRegistrations)
	newRegistrations       int64 // Added registrations since last reset()
	newMissedRegistrations int64 // number of "missed" registrations (as seen by a connection with no registration)
	newErrRegistrations    int64 // number of registrations that had some kinda error
	newDupRegistrations    int64 // number of duplicate registrations (doesn't uniquify, so might have some double counting)


	newLivenessPass int64 // Liveness tests that passed (non-live phantom) since reset()
	newLivenessFail int64 // Liveness tests that failed (live phantom) since reset()

	genMutex    *sync.Mutex      // Lock for generations map
	generations map[uint32]int64 // Map from ClientConf generation to number of registrations we saw using it

	newBytesUp   int64 // TODO: need to redo halfPipe to make this not really jumpy
	newBytesDown int64 // ditto
}

var statInstance Stats
var statsOnce sync.Once

// Returns our singleton stats
func Stat() *Stats {
	statsOnce.Do(initStats)
	return &statInstance
}

func initStats() {
	logger := log.New(os.Stdout, "[STATS] ", log.Ldate|log.Lmicroseconds)
	statInstance = Stats{
		logger:      logger,
		generations: make(map[uint32]int64),
		genMutex:    &sync.Mutex{},
	}

	// Periodic PrintStats()
	ticker := time.NewTicker(5 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				statInstance.PrintStats()
			}
		}
	}()
}

func (s *Stats) Reset() {
	atomic.StoreInt64(&s.newConns, 0)
	atomic.StoreInt64(&s.newRegistrations, 0)
	atomic.StoreInt64(&s.newLocalRegistrations, 0)
	atomic.StoreInt64(&s.newApiRegistrations, 0)
	atomic.StoreInt64(&s.newMissedRegistrations, 0)
	atomic.StoreInt64(&s.newErrRegistrations, 0)
	atomic.StoreInt64(&s.newDupRegistrations, 0)
	atomic.StoreInt64(&s.newLivenessPass, 0)
	atomic.StoreInt64(&s.newLivenessFail, 0)
	atomic.StoreInt64(&s.newBytesUp, 0)
	atomic.StoreInt64(&s.newBytesDown, 0)
}

func (s *Stats) PrintStats() {
	s.logger.Printf("Conns: %d cur %d new %d err Regs: %d cur %d new (%d local %d API) %d miss %d err %d dup LiveT: %d valid %d live Byte: %d up %d down",
		atomic.LoadInt64(&s.activeConns), atomic.LoadInt64(&s.newConns), atomic.LoadInt64(&s.newErrConns),
		atomic.LoadInt64(&s.activeRegistrations),
		atomic.LoadInt64(&s.newRegistrations),
		atomic.LoadInt64(&s.newLocalRegistrations), atomic.LoadInt64(&s.newApiRegistrations),
		atomic.LoadInt64(&s.newMissedRegistrations),
		atomic.LoadInt64(&s.newErrRegistrations), atomic.LoadInt64(&s.newDupRegistrations),
		atomic.LoadInt64(&s.newLivenessPass), atomic.LoadInt64(&s.newLivenessFail),
		atomic.LoadInt64(&s.newBytesUp), atomic.LoadInt64(&s.newBytesDown))
	s.Reset()
}

func (s *Stats) AddConn() {
	atomic.AddInt64(&s.activeConns, 1)
	atomic.AddInt64(&s.newConns, 1)
}

func (s *Stats) CloseConn() {
	atomic.AddInt64(&s.activeConns, -1)
}

func (s *Stats) ConnErr() {
	atomic.AddInt64(&s.activeConns, -1)
	atomic.AddInt64(&s.newErrConns, 1)
}

func (s *Stats) AddReg(generation uint32, source *pb.RegistrationSource) {
	atomic.AddInt64(&s.activeRegistrations, 1)
	atomic.AddInt64(&s.newRegistrations, 1)

	if *source == pb.RegistrationSource_Detector {
		//atomic.AddInt64(&s.activeLocalRegistrations, 1) // Actually an absolute is not super useful.
		atomic.AddInt64(&s.newLocalRegistrations, 1)
	} else {
		//atomic.AddInt64(&s.activeApiRegistrations, 1)
		atomic.AddInt64(&s.newApiRegistrations, 1)
	}
	s.genMutex.Lock()
	s.generations[generation] += 1
	s.genMutex.Unlock()
}

func (s *Stats) AddDupReg() {
	atomic.AddInt64(&s.newDupRegistrations, 1)
}

func (s *Stats) AddErrReg() {
	atomic.AddInt64(&s.newErrRegistrations, 1)
}

func (s *Stats) ExpireReg(generation uint32, source *pb.RegistrationSource) {
	atomic.AddInt64(&s.activeRegistrations, -1)

	/*
		if *source == pb.RegistrationSource_Detector {
			atomic.AddInt64(&s.activeLocalRegistrations, -1)
		} else {
			atomic.AddInt64(&s.activeApiRegistrations, -1)
		}*/
	s.genMutex.Lock()
	s.generations[generation] -= 1
	s.genMutex.Unlock()
}

func (s *Stats) AddMissedReg() {
	atomic.AddInt64(&s.newMissedRegistrations, 1)
}

func (s *Stats) AddLivenessPass() {
	atomic.AddInt64(&s.newLivenessPass, 1)
}

func (s *Stats) AddLivenessFail() {
	atomic.AddInt64(&s.newLivenessFail, 1)
}

func (s *Stats) AddBytesUp(n int64) {
	atomic.AddInt64(&s.newBytesUp, n)
}

func (s *Stats) AddBytesDown(n int64) {
	atomic.AddInt64(&s.newBytesDown, n)
}

func (s *Stats) AddBytes(n int64, dir string) {
	if dir == "Up" {
		s.AddBytesUp(n)
	} else {
		s.AddBytesDown(n)
	}
}
