package transfer

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

type TransferErrorCode string

type SessionPhase string

const (
	PhaseConnecting  SessionPhase = "connecting"
	PhaseHandshaking SessionPhase = "handshaking"
	PhaseStreaming   SessionPhase = "streaming"
)

const (
	ErrConnectTimeout    TransferErrorCode = "ERR_CONNECT_TIMEOUT"
	ErrHandshakeTimeout  TransferErrorCode = "ERR_HANDSHAKE_TIMEOUT"
	ErrHandshakeMismatch TransferErrorCode = "ERR_HANDSHAKE_MISMATCH"
	ErrStreamBrokenPipe  TransferErrorCode = "ERR_STREAM_BROKEN_PIPE"
	ErrStreamConnReset   TransferErrorCode = "ERR_STREAM_CONN_RESET"
	ErrStreamIdleTimeout TransferErrorCode = "ERR_STREAM_IDLE_TIMEOUT"
	ErrUpstreamEOF       TransferErrorCode = "ERR_UPSTREAM_EOF_UNEXPECTED"
	ErrStreamUnknown     TransferErrorCode = "ERR_STREAM_UNKNOWN"
)

type TransferSummary struct {
	SessionID string
	Role      string
	Endpoint  string
	Phase     SessionPhase
	Code      TransferErrorCode
	Bytes     int64
	ElapsedMS int64
	IdleMS    int64
	Cause     string
}

func ClassifyTransferError(err error) TransferErrorCode {
	if err == nil {
		return ""
	}

	errStr := strings.ToLower(err.Error())

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrConnectTimeout
	}

	if errors.Is(err, syscall.EPIPE) || strings.Contains(errStr, "broken pipe") {
		return ErrStreamBrokenPipe
	}

	if errors.Is(err, syscall.ECONNRESET) || strings.Contains(errStr, "connection reset") {
		return ErrStreamConnReset
	}

	return ErrStreamUnknown
}

func ClassifyTransferErrorWithPhase(err error, phase SessionPhase) TransferErrorCode {
	if err == nil {
		return ""
	}

	errStr := strings.ToLower(err.Error())

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		switch phase {
		case PhaseConnecting:
			return ErrConnectTimeout
		case PhaseHandshaking:
			return ErrHandshakeTimeout
		case PhaseStreaming:
			return ErrStreamIdleTimeout
		default:
			return ErrStreamUnknown
		}
	}

	if errors.Is(err, syscall.EPIPE) || strings.Contains(errStr, "broken pipe") {
		return ErrStreamBrokenPipe
	}

	if errors.Is(err, syscall.ECONNRESET) || strings.Contains(errStr, "connection reset") {
		return ErrStreamConnReset
	}

	return ErrStreamUnknown
}

func BuildTerminalSummary(s TransferSummary) string {
	return fmt.Sprintf("[transfer-summary] session=%s role=%s endpoint=%s phase=%s code=%s bytes=%d elapsed_ms=%d idle_ms=%d cause=%q",
		s.SessionID, s.Role, s.Endpoint, s.Phase, s.Code, s.Bytes, s.ElapsedMS, s.IdleMS, s.Cause)
}
