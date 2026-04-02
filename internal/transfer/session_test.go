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

func TestSessionTransitionRecord(t *testing.T) {
	s := NewTransferSession("sender", "tcp://127.0.0.1:3306")

	s.Transition(PhaseStreaming)

	record, ok := s.LastTransition()
	if !ok {
		t.Fatal("expected transition record to exist")
	}
	if record.Phase != PhaseStreaming {
		t.Fatalf("transition phase mismatch: got %s want %s", record.Phase, PhaseStreaming)
	}
	if record.Endpoint != "tcp://127.0.0.1:3306" {
		t.Fatalf("transition endpoint mismatch: got %s want %s", record.Endpoint, "tcp://127.0.0.1:3306")
	}
	if record.At.IsZero() {
		t.Fatal("expected transition timestamp to be set")
	}
}

func TestSessionTransitionWithMetadata(t *testing.T) {
	s := NewTransferSession("sender", "tcp://127.0.0.1:3306")

	metadata := map[string]string{
		"attempt": "1",
		"role":    "sender",
	}

	s.TransitionWithMetadata(PhaseFinalizing, metadata)

	record, ok := s.LastTransition()
	if !ok {
		t.Fatal("expected transition record to exist")
	}
	if record.Phase != PhaseFinalizing {
		t.Fatalf("transition phase mismatch: got %s want %s", record.Phase, PhaseFinalizing)
	}
	if record.Metadata["attempt"] != "1" || record.Metadata["role"] != "sender" {
		t.Fatalf("transition metadata mismatch: got %#v want %#v", record.Metadata, metadata)
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

func TestSessionCloseOwnership(t *testing.T) {
	s := NewTransferSession("receiver", "tcp://127.0.0.1:3306")

	if !s.TryAcquireCloseOwnership() {
		t.Fatal("expected first close ownership acquisition to succeed")
	}
	if s.TryAcquireCloseOwnership() {
		t.Fatal("expected second close ownership acquisition to fail")
	}
	if !s.HasCloseOwnership() {
		t.Fatal("expected close ownership to be reported after acquisition")
	}
}
