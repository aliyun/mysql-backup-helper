package transfer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type TransitionRecord struct {
	Phase    SessionPhase
	At       time.Time
	Endpoint string
	Metadata map[string]string
}

type TransferSession struct {
	id                          string
	role                        string
	endpoint                    string
	startedAt                   time.Time
	lastProgressSinceStartNanos atomic.Int64
	bytes                       atomic.Int64
	closeOwnership              atomic.Uint32
	mutex                       sync.Mutex
	phase                       SessionPhase
	transitions                 []TransitionRecord
}

func NewTransferSession(role, endpoint string) *TransferSession {
	now := time.Now()
	s := &TransferSession{
		id:        fmt.Sprintf("%d", now.UnixNano()),
		role:      role,
		endpoint:  endpoint,
		startedAt: now,
		phase:     PhaseInit,
	}
	s.lastProgressSinceStartNanos.Store(0)
	return s
}

func (s *TransferSession) Transition(phase SessionPhase) {
	s.TransitionWithMetadata(phase, nil)
}

func (s *TransferSession) TransitionWithMetadata(phase SessionPhase, metadata map[string]string) {
	var metadataCopy map[string]string
	if metadata != nil {
		metadataCopy = make(map[string]string, len(metadata))
		for k, v := range metadata {
			metadataCopy[k] = v
		}
	}

	record := TransitionRecord{
		Phase:    phase,
		At:       time.Now(),
		Endpoint: s.endpoint,
		Metadata: metadataCopy,
	}

	s.mutex.Lock()
	s.phase = phase
	s.transitions = append(s.transitions, record)
	s.mutex.Unlock()
}

func (s *TransferSession) Phase() SessionPhase {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.phase
}

func (s *TransferSession) LastTransition() (TransitionRecord, bool) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if len(s.transitions) == 0 {
		return TransitionRecord{}, false
	}

	return s.transitions[len(s.transitions)-1], true
}

func (s *TransferSession) MarkProgress(n int64) {
	s.bytes.Add(n)
	s.lastProgressSinceStartNanos.Store(time.Since(s.startedAt).Nanoseconds())
}

func (s *TransferSession) Bytes() int64 {
	return s.bytes.Load()
}

func (s *TransferSession) TryAcquireCloseOwnership() bool {
	return s.closeOwnership.CompareAndSwap(0, 1)
}

func (s *TransferSession) HasCloseOwnership() bool {
	return s.closeOwnership.Load() == 1
}

func (s *TransferSession) IsIdleExceeded(timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}

	return time.Since(s.startedAt).Nanoseconds()-s.lastProgressSinceStartNanos.Load() > timeout.Nanoseconds()
}
