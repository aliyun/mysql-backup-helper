package transfer

import (
	"testing"
	"time"
)

func TestSessionTransitionSequence(t *testing.T) {
	s := NewTransferSession("sender", "tcp://127.0.0.1:3306")

	if got := s.Phase(); got != PhaseInit {
		t.Fatalf("initial phase mismatch: got %s want %s", got, PhaseInit)
	}

	sequence := []SessionPhase{
		PhaseListening,
		PhaseConnecting,
		PhaseHandshaking,
		PhaseStreaming,
		PhaseFinalizing,
		PhaseDone,
	}

	for _, phase := range sequence {
		s.Transition(phase)
		if got := s.Phase(); got != phase {
			t.Fatalf("phase mismatch after transition: got %s want %s", got, phase)
		}
	}

	s.Transition(PhaseFailed)
	if got := s.Phase(); got != PhaseFailed {
		t.Fatalf("failed phase mismatch: got %s want %s", got, PhaseFailed)
	}
}

func TestSessionIdleTimeout(t *testing.T) {
	s := NewTransferSession("receiver", "tcp://127.0.0.1:3306")

	if s.IsIdleExceeded(0) {
		t.Fatal("expected non-positive timeout to be treated as disabled")
	}
	if s.IsIdleExceeded(-1 * time.Millisecond) {
		t.Fatal("expected negative timeout to be treated as disabled")
	}

	if s.IsIdleExceeded(50 * time.Millisecond) {
		t.Fatal("expected fresh session to be below idle timeout")
	}

	s.MarkProgress(2048)
	if got := s.Bytes(); got != 2048 {
		t.Fatalf("bytes mismatch after progress: got %d want %d", got, 2048)
	}

	time.Sleep(120 * time.Millisecond)

	if !s.IsIdleExceeded(50 * time.Millisecond) {
		t.Fatal("expected idle timeout to be exceeded after no progress")
	}
}
