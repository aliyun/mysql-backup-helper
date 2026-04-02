package transfer

import (
	"errors"
	"syscall"
	"testing"
	"strings"
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
	s := TransferSummary{SessionID: "abc123", Role: "sender", Code: ErrStreamBrokenPipe, Bytes: 1024, Phase: "streaming"}
	out := BuildTerminalSummary(s)
	if !strings.HasPrefix(out, "[transfer-summary]") {
		t.Fatalf("expected stable prefix, got %q", out)
	}

	requiredFields := []string{
		"session=",
		"role=",
		"endpoint=",
		"phase=",
		"code=",
		"bytes=",
		"elapsed_ms=",
		"idle_ms=",
		"cause=",
	}
	for _, field := range requiredFields {
		if !strings.Contains(out, field) {
			t.Fatalf("expected summary to contain %q, got %q", field, out)
		}
	}
}
