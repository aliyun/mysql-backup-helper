package transfer

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type TransferSession struct {
	id         string
	role       string
	endpoint   string
	startedAt  time.Time
	lastProgress atomic.Int64
	bytes      atomic.Int64
	mutex      sync.Mutex
	phase      SessionPhase
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
	s.lastProgress.Store(now.UnixNano())
	return s
}

func (s *TransferSession) Transition(phase SessionPhase) {
	s.mutex.Lock()
	s.phase = phase
	s.mutex.Unlock()
}

func (s *TransferSession) Phase() SessionPhase {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.phase
}

func (s *TransferSession) MarkProgress(n int64) {
	s.bytes.Add(n)
	s.lastProgress.Store(time.Now().UnixNano())
}

func (s *TransferSession) Bytes() int64 {
	return s.bytes.Load()
}

func (s *TransferSession) IsIdleExceeded(timeout time.Duration) bool {
	if timeout <= 0 {
		return false
	}

	last := time.Unix(0, s.lastProgress.Load())
	return time.Since(last) > timeout
}
