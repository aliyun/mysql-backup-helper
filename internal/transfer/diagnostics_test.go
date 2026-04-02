package transfer

import (
	"errors"
	"syscall"
	"testing"
)

func TestClassifyTransferError_BrokenPipe(t *testing.T) {
	err := errors.New("write tcp 1.1.1.1:1234->2.2.2.2:5678: broken pipe")
	if got := ClassifyTransferError(err); got != ErrStreamBrokenPipe {
		t.Fatalf("expected %s, got %s", ErrStreamBrokenPipe, got)
	}
}

func TestClassifyTransferError_ConnReset(t *testing.T) {
	err := syscall.ECONNRESET
	if got := ClassifyTransferError(err); got != ErrStreamConnReset {
		t.Fatalf("expected %s, got %s", ErrStreamConnReset, got)
	}
}

func TestBuildTerminalSummary(t *testing.T) {
	s := TransferSummary{SessionID: "abc123", Role: "sender", Code: ErrStreamBrokenPipe, Bytes: 1024, Phase: PhaseStreaming}
	out := BuildTerminalSummary(s)
	if out == "" {
		t.Fatal("expected non-empty summary")
	}
}
