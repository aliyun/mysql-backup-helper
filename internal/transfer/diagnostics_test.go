package transfer

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func TestClassifyTransferError_Nil(t *testing.T) {
	if got := ClassifyTransferError(nil); got != "" {
		t.Fatalf("expected empty code, got %s", got)
	}
}

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

func TestClassifyTransferError_Timeout(t *testing.T) {
	timeoutErr := &timeoutError{}

	if got := ClassifyTransferError(timeoutErr); got != ErrConnectTimeout {
		t.Fatalf("expected %s, got %s", ErrConnectTimeout, got)
	}
}

func TestClassifyTransferErrorWithPhase_Timeouts(t *testing.T) {
	timeoutErr := &timeoutError{}

	tests := []struct {
		name  string
		phase string
		want  TransferErrorCode
	}{
		{name: "connecting", phase: "connecting", want: ErrConnectTimeout},
		{name: "handshaking", phase: "handshaking", want: ErrHandshakeTimeout},
		{name: "streaming", phase: "streaming", want: ErrStreamIdleTimeout},
		{name: "default", phase: "", want: ErrStreamUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ClassifyTransferErrorWithPhase(timeoutErr, tt.phase); got != tt.want {
				t.Fatalf("expected %s, got %s", tt.want, got)
			}
		})
	}
}

func TestBuildTerminalSummary(t *testing.T) {
	s := TransferSummary{
		SessionID: "abc123",
		Role:      "sender",
		Endpoint:  "tcp://1.2.3.4:3306",
		Phase:     SessionPhase("streaming"),
		Code:      ErrStreamBrokenPipe,
		Bytes:     1024,
		ElapsedMS: 12,
		IdleMS:    34,
		Cause:     "broken pipe",
	}
	out := BuildTerminalSummary(s)
	if !strings.HasPrefix(out, "[transfer-summary]") {
		t.Fatalf("expected stable prefix, got %q", out)
	}

	expected := `[transfer-summary] session=abc123 role=sender endpoint=tcp://1.2.3.4:3306 phase=streaming code=ERR_STREAM_BROKEN_PIPE bytes=1024 elapsed_ms=12 idle_ms=34 cause="broken pipe"`
	if out != expected {
		t.Fatalf("expected %q, got %q", expected, out)
	}
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }
