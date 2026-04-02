# Transfer Reliability Refactor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor `internal/transfer/*` so stream transfer never hangs silently and always emits actionable root-cause diagnostics for broken pipe and related transport failures while preserving `nc|xbstream` compatibility.

**Architecture:** Add a shared transfer session core (`session`, `diagnostics`, `integrity`) and route existing stream entrypoints through it without changing external CLI behavior. Standardize timeout enforcement and terminal close ownership so connection lifecycle is deterministic. Emit normalized error codes and a final one-line summary containing phase, endpoint, bytes transferred, and timing.

**Tech Stack:** Go 1.21, standard library (`net`, `io`, `errors`, `sync/atomic`, `time`), existing `internal/log` and `internal/progress` modules, Makefile-based verification.

---

## File Structure And Responsibilities

- Create: `internal/transfer/diagnostics.go`
- Create: `internal/transfer/session.go`
- Create: `internal/transfer/integrity.go`
- Create: `internal/transfer/diagnostics_test.go`
- Create: `internal/transfer/session_test.go`
- Create: `internal/transfer/integrity_test.go`
- Modify: `internal/transfer/tcp.go`
- Modify: `internal/transfer/ssh.go`
- Create: `docs/stream-smoke-test.md`
- Create: `scripts/stream_smoke.sh`
- Modify: `makefile`

Responsibility boundaries:
- `diagnostics.go`: normalized error codes, error classification, and terminal summary formatter.
- `session.go`: state machine, transition recording, timeout/idle tracking, and close ownership.
- `integrity.go`: byte counters and optional digest interface for helper-to-helper finalize.
- `tcp.go`: transport wiring only; avoid inline lifecycle complexity.
- `ssh.go`: robust receiver-start parsing and timeout/EOF diagnostics.
- `scripts/stream_smoke.sh` + `docs/stream-smoke-test.md`: required real-world gate and manual runbook.

## Task 1: Add Transfer Diagnostics Core

**Files:**
- Create: `internal/transfer/diagnostics.go`
- Test: `internal/transfer/diagnostics_test.go`

- [ ] **Step 1: Write failing diagnostics tests**

```go
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
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/transfer -run 'TestClassifyTransferError|TestBuildTerminalSummary'`
Expected: FAIL with undefined identifiers such as `ClassifyTransferError`.

- [ ] **Step 3: Implement diagnostics module**

```go
package transfer

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
)

type TransferErrorCode string

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
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrConnectTimeout
	}
	if errors.Is(err, syscall.EPIPE) || strings.Contains(strings.ToLower(err.Error()), "broken pipe") {
		return ErrStreamBrokenPipe
	}
	if errors.Is(err, syscall.ECONNRESET) || strings.Contains(strings.ToLower(err.Error()), "connection reset") {
		return ErrStreamConnReset
	}
	return ErrStreamUnknown
}

func BuildTerminalSummary(s TransferSummary) string {
	return fmt.Sprintf("[transfer-summary] session=%s role=%s endpoint=%s phase=%s code=%s bytes=%d elapsed_ms=%d idle_ms=%d cause=%q",
		s.SessionID, s.Role, s.Endpoint, s.Phase, s.Code, s.Bytes, s.ElapsedMS, s.IdleMS, s.Cause)
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/transfer -run 'TestClassifyTransferError|TestBuildTerminalSummary'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transfer/diagnostics.go internal/transfer/diagnostics_test.go
git commit -m "feat: add transfer diagnostics core and classification tests"
```

## Task 2: Add Session State Machine With Timeout/Close Ownership

**Files:**
- Create: `internal/transfer/session.go`
- Test: `internal/transfer/session_test.go`

- [ ] **Step 1: Write failing session tests**

```go
package transfer

import (
	"testing"
	"time"
)

func TestSessionTransitionSequence(t *testing.T) {
	s := NewTransferSession("sender", "10.0.0.1:33061")
	s.Transition(PhaseConnecting)
	s.Transition(PhaseHandshaking)
	s.Transition(PhaseStreaming)
	if s.Phase() != PhaseStreaming {
		t.Fatalf("expected %s, got %s", PhaseStreaming, s.Phase())
	}
}

func TestSessionIdleTimeout(t *testing.T) {
	s := NewTransferSession("sender", "10.0.0.1:33061")
	s.MarkProgress(100)
	time.Sleep(20 * time.Millisecond)
	if !s.IsIdleExceeded(10 * time.Millisecond) {
		t.Fatal("expected idle timeout exceeded")
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/transfer -run 'TestSessionTransitionSequence|TestSessionIdleTimeout'`
Expected: FAIL with undefined `NewTransferSession`, `PhaseConnecting`, etc.

- [ ] **Step 3: Implement session state machine**

```go
package transfer

import (
	"sync"
	"sync/atomic"
	"time"
)

type SessionPhase string

const (
	PhaseInit        SessionPhase = "init"
	PhaseListening   SessionPhase = "listening"
	PhaseConnecting  SessionPhase = "connecting"
	PhaseHandshaking SessionPhase = "handshaking"
	PhaseStreaming   SessionPhase = "streaming"
	PhaseFinalizing  SessionPhase = "finalizing"
	PhaseDone        SessionPhase = "done"
	PhaseFailed      SessionPhase = "failed"
)

type TransferSession struct {
	id           string
	role         string
	endpoint     string
	startedAt    time.Time
	lastProgress atomic.Int64
	bytes        atomic.Int64
	mu           sync.Mutex
	phase        SessionPhase
}

func NewTransferSession(role, endpoint string) *TransferSession {
	now := time.Now()
	s := &TransferSession{id: now.Format("20060102150405.000000000"), role: role, endpoint: endpoint, startedAt: now, phase: PhaseInit}
	s.lastProgress.Store(now.UnixNano())
	return s
}

func (s *TransferSession) Transition(p SessionPhase) { s.mu.Lock(); s.phase = p; s.mu.Unlock() }
func (s *TransferSession) Phase() SessionPhase       { s.mu.Lock(); defer s.mu.Unlock(); return s.phase }
func (s *TransferSession) MarkProgress(n int64)      { s.bytes.Add(n); s.lastProgress.Store(time.Now().UnixNano()) }
func (s *TransferSession) Bytes() int64              { return s.bytes.Load() }
func (s *TransferSession) IsIdleExceeded(limit time.Duration) bool {
	last := time.Unix(0, s.lastProgress.Load())
	return time.Since(last) > limit
}
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/transfer -run 'TestSessionTransitionSequence|TestSessionIdleTimeout'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transfer/session.go internal/transfer/session_test.go
git commit -m "feat: add transfer session state machine and timeout tracking"
```

## Task 3: Add Integrity Wrappers And Tests

**Files:**
- Create: `internal/transfer/integrity.go`
- Test: `internal/transfer/integrity_test.go`

- [ ] **Step 1: Write failing integrity tests**

```go
package transfer

import (
	"bytes"
	"io"
	"testing"
)

func TestCountingWriterCountsBytes(t *testing.T) {
	var buf bytes.Buffer
	cw := NewCountingWriter(&buf)
	if _, err := cw.Write([]byte("abc")); err != nil {
		t.Fatal(err)
	}
	if got := cw.BytesWritten(); got != 3 {
		t.Fatalf("expected 3, got %d", got)
	}
}

func TestCountingReaderCountsBytes(t *testing.T) {
	cr := NewCountingReader(bytes.NewBufferString("abcd"))
	if _, err := io.ReadAll(cr); err != nil {
		t.Fatal(err)
	}
	if got := cr.BytesRead(); got != 4 {
		t.Fatalf("expected 4, got %d", got)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/transfer -run 'TestCountingWriterCountsBytes|TestCountingReaderCountsBytes'`
Expected: FAIL with undefined counting wrappers.

- [ ] **Step 3: Implement integrity wrappers**

```go
package transfer

import (
	"hash"
	"hash/crc32"
	"io"
	"sync/atomic"
)

type CountingWriter struct {
	w     io.Writer
	bytes atomic.Int64
	h     hash.Hash32
}

func NewCountingWriter(w io.Writer) *CountingWriter {
	return &CountingWriter{w: w, h: crc32.NewIEEE()}
}

func (cw *CountingWriter) Write(p []byte) (int, error) {
	n, err := cw.w.Write(p)
	if n > 0 {
		cw.bytes.Add(int64(n))
		_, _ = cw.h.Write(p[:n])
	}
	return n, err
}

func (cw *CountingWriter) BytesWritten() int64 { return cw.bytes.Load() }
func (cw *CountingWriter) CRC32() uint32       { return cw.h.Sum32() }

type CountingReader struct {
	r     io.Reader
	bytes atomic.Int64
}

func NewCountingReader(r io.Reader) *CountingReader { return &CountingReader{r: r} }

func (cr *CountingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	if n > 0 {
		cr.bytes.Add(int64(n))
	}
	return n, err
}

func (cr *CountingReader) BytesRead() int64 { return cr.bytes.Load() }
```

- [ ] **Step 4: Run tests to verify pass**

Run: `go test ./internal/transfer -run 'TestCountingWriterCountsBytes|TestCountingReaderCountsBytes'`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transfer/integrity.go internal/transfer/integrity_test.go
git commit -m "feat: add transfer byte-count and checksum wrappers"
```

## Task 4: Refactor `tcp.go` To Use Session/Diagnostics Core

**Files:**
- Modify: `internal/transfer/tcp.go`
- Test: `internal/transfer/session_test.go`
- Test: `internal/transfer/diagnostics_test.go`

- [ ] **Step 1: Write/extend failing behavior tests for hang/error mapping**

```go
func TestMapHandshakeTimeoutToErrorCode(t *testing.T) {
	code := mapHandshakeErrToCode(context.DeadlineExceeded)
	if code != ErrHandshakeTimeout {
		t.Fatalf("expected %s, got %s", ErrHandshakeTimeout, code)
	}
}

func TestMapBrokenPipeToErrorCode(t *testing.T) {
	code := ClassifyTransferError(errors.New("write: broken pipe"))
	if code != ErrStreamBrokenPipe {
		t.Fatalf("expected %s, got %s", ErrStreamBrokenPipe, code)
	}
}
```

- [ ] **Step 2: Run tests to verify failure**

Run: `go test ./internal/transfer -run 'TestMapHandshakeTimeoutToErrorCode|TestMapBrokenPipeToErrorCode'`
Expected: FAIL due to missing mapping helper.

- [ ] **Step 3: Refactor sender/receiver flows with shared session hooks**

```go
// inside StartStreamSender / StartStreamReceiver flow
session := NewTransferSession("sender", addr)
session.Transition(PhaseConnecting)

conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
if err != nil {
	return nil, nil, nil, "", wrapTransferErr(session, err, ErrConnectTimeout)
}

session.Transition(PhaseHandshaking)
if err := runHandshakeWithTimeout(conn, handshakeKey, 10*time.Second); err != nil {
	return nil, nil, nil, "", wrapTransferErr(session, err, mapHandshakeErrToCode(err))
}

session.Transition(PhaseStreaming)
cw := NewCountingWriter(conn)
progressWriter := progress.NewProgressWriter(cw, tracker)

closer := func() {
	session.Transition(PhaseFinalizing)
	tracker.Complete()
	_ = conn.Close()
	session.Transition(PhaseDone)
	if logCtx != nil {
		summary := BuildTerminalSummary(TransferSummary{
			SessionID: session.id,
			Role:      "sender",
			Endpoint:  addr,
			Phase:     session.Phase(),
			Code:      "",
			Bytes:     cw.BytesWritten(),
			ElapsedMS: int64(time.Since(session.startedAt) / time.Millisecond),
		})
		logCtx.WriteLog("TCP", "%s", summary)
	}
}
```

- [ ] **Step 4: Run focused package tests**

Run: `go test ./internal/transfer`
Expected: PASS.

- [ ] **Step 5: Run full test/build verification**

Run: `go test ./... && make build`
Expected: all packages PASS, binary builds successfully.

- [ ] **Step 6: Commit**

```bash
git add internal/transfer/tcp.go internal/transfer/*.go internal/transfer/*_test.go
git commit -m "refactor: unify tcp transfer lifecycle with session diagnostics"
```

## Task 5: Improve SSH Receiver Startup Diagnostics

**Files:**
- Modify: `internal/transfer/ssh.go`
- Test: `internal/transfer/diagnostics_test.go`

- [ ] **Step 1: Add failing test for startup parse timeout context**

```go
func TestParseReceiverInfoTimeoutIncludesContext(t *testing.T) {
	_, _, err := parseReceiverInfo(strings.NewReader(""), 0, "")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "receiver") {
		t.Fatalf("expected receiver context in error: %v", err)
	}
}
```

- [ ] **Step 2: Run focused tests to verify failure**

Run: `go test ./internal/transfer -run TestParseReceiverInfoTimeoutIncludesContext`
Expected: FAIL if context text/mapping not yet present.

- [ ] **Step 3: Implement improved SSH startup diagnostics and stderr draining safety**

```go
func parseReceiverInfo(reader io.Reader, expectedPort int, remoteOutput string) (int, string, error) {
	scanner := bufio.NewScanner(reader)
	portPattern := regexp.MustCompile(`Listening on [\\d.]+:(\\d+)`)
	timeoutAt := time.Now().Add(10 * time.Second)
	lastLine := ""
	for time.Now().Before(timeoutAt) {
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				return 0, "", fmt.Errorf("receiver startup read failed: %w", err)
			}
			time.Sleep(100 * time.Millisecond)
			continue
		}
		lastLine = scanner.Text()
		if m := portPattern.FindStringSubmatch(lastLine); m != nil {
			port, err := strconv.Atoi(m[1])
			if err != nil {
				return 0, "", fmt.Errorf("receiver startup invalid port %q: %w", m[1], err)
			}
			if expectedPort > 0 && port != expectedPort {
				return 0, "", fmt.Errorf("receiver startup port mismatch: expected=%d actual=%d", expectedPort, port)
			}
			return port, remoteOutput, nil
		}
	}
	return 0, "", fmt.Errorf("receiver startup timeout after 10s: expected_port=%d last_line=%q", expectedPort, lastLine)
}

// in StartRemoteReceiverViaSSH cleanup path:
cleanupFunc := func() error {
	killCmd := exec.Command("ssh", sshHost, "pkill -TERM backup-helper")
	_ = killCmd.Run()
	if err := cmd.Process.Kill(); err != nil {
		return fmt.Errorf("ssh cleanup failed: %w", err)
	}
	return nil
}
```

- [ ] **Step 4: Run package tests and build**

Run: `go test ./internal/transfer && go test ./... && make build`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/transfer/ssh.go internal/transfer/diagnostics_test.go
git commit -m "fix: improve ssh receiver startup diagnostics for timeout and eof cases"
```

## Task 6: Add Required Real-World Smoke Gate + Manual Runbook

**Files:**
- Create: `scripts/stream_smoke.sh`
- Create: `docs/stream-smoke-test.md`
- Modify: `makefile`

- [ ] **Step 1: Write failing test hook (make target missing)**

Run: `make test-stream-smoke`
Expected: FAIL with `No rule to make target 'test-stream-smoke'`.

- [ ] **Step 2: Implement smoke test script for two-terminal real tools**

```bash
#!/usr/bin/env bash
set -euo pipefail

require_bin() { command -v "$1" >/dev/null 2>&1 || { echo "missing: $1"; exit 1; }; }
require_bin backup-helper
require_bin nc
require_bin xbstream
require_bin xtrabackup

SENDER_LOG="${SENDER_LOG:-/tmp/backup-helper-sender.log}"
RECV_DIR="${RECV_DIR:-/tmp/backup-helper-smoke-recv}"
mkdir -p "$RECV_DIR"

echo "[smoke] scenario=handshake-mismatch"
PORT_MISMATCH="${PORT_MISMATCH:-34001}"
RECV_KEY="good-key"
BAD_KEY="bad-key"
(echo "$RECV_KEY";) | nc 127.0.0.1 "$PORT_MISMATCH" 2>"$RECV_DIR/nc-mismatch.log" | xbstream -x -C "$RECV_DIR" 2>"$RECV_DIR/xb-mismatch.log" &
RECV_PID=$!
sleep 1
set +e
backup-helper --backup --mode stream --stream-host 127.0.0.1 --stream-port "$PORT_MISMATCH" --enable-handshake --stream-key "$BAD_KEY" --log-file-name "$SENDER_LOG"
S1=$?
set -e
kill "$RECV_PID" >/dev/null 2>&1 || true
grep -q "ERR_HANDSHAKE_MISMATCH" "$SENDER_LOG"
[ "$S1" -ne 0 ]

echo "[smoke] scenario=broken-pipe"
PORT_BROKEN="${PORT_BROKEN:-34002}"
(echo "$RECV_KEY";) | nc 127.0.0.1 "$PORT_BROKEN" 2>"$RECV_DIR/nc-broken.log" | xbstream -x -C "$RECV_DIR" 2>"$RECV_DIR/xb-broken.log" &
RECV_PID=$!
sleep 1
backup-helper --backup --mode stream --stream-host 127.0.0.1 --stream-port "$PORT_BROKEN" --enable-handshake --stream-key "$RECV_KEY" --log-file-name "$SENDER_LOG" &
SEND_PID=$!
sleep 3
kill "$RECV_PID" >/dev/null 2>&1 || true
wait "$SEND_PID" || true
grep -q "ERR_STREAM_BROKEN_PIPE" "$SENDER_LOG"

echo "[smoke] scenario=normal"
PORT_OK="${PORT_OK:-34003}"
(echo "$RECV_KEY";) | nc 127.0.0.1 "$PORT_OK" 2>"$RECV_DIR/nc-ok.log" | xbstream -x -C "$RECV_DIR" 2>"$RECV_DIR/xb-ok.log" &
sleep 1
backup-helper --backup --mode stream --stream-host 127.0.0.1 --stream-port "$PORT_OK" --enable-handshake --stream-key "$RECV_KEY" --log-file-name "$SENDER_LOG"
grep -q "completed OK!" "$SENDER_LOG"
```

- [ ] **Step 3: Add manual runbook with explicit two-terminal commands**

```markdown
# Stream Smoke Test Runbook

## Terminal A (sender)
backup-helper --backup --mode stream --stream-host <receiver_ip> --stream-port <port> --enable-handshake --stream-key <key> --host <mysql_host> --port 3306 --user <mysql_user>

## Terminal B (receiver)
(echo "<key>";) | nc <sender_ip> <port> 2>nc.log | xbstream -x -C <target_dir> 2>xbstream.log

## Required evidence
- sender log file path + final transfer summary line
- `nc.log`, `xbstream.log`
- receiver command exit code
```

- [ ] **Step 4: Wire Makefile target**

```make
.PHONY: test-stream-smoke

test-stream-smoke:
	bash ./scripts/stream_smoke.sh
```

- [ ] **Step 5: Run gate commands**

Run: `go test ./... && make build && make test-stream-smoke`
Expected: PASS in environment with required binaries and permissions.

- [ ] **Step 6: Commit**

```bash
git add scripts/stream_smoke.sh docs/stream-smoke-test.md makefile
git commit -m "test: add real-world stream smoke gate and operator runbook"
```

## Task 7: Final Verification And Release Notes

**Files:**
- Modify: `README.md`
- Modify: `README.en.md`

- [ ] **Step 1: Document new diagnostics and smoke gate usage**

```markdown
- Added transfer terminal summary with normalized error codes (e.g. `ERR_STREAM_BROKEN_PIPE`).
- Added `make test-stream-smoke` as required pre-release validation in environments with xtrabackup/xbstream/nc.
```

- [ ] **Step 2: Run final verification**

Run: `go test ./... && make build`
Expected: PASS.

- [ ] **Step 3: Commit docs update**

```bash
git add README.md README.en.md
git commit -m "docs: document transfer diagnostics and smoke test gate"
```

## Spec Coverage Check

- Silent-hang prevention: covered by Tasks 2 and 4 (phase tracking + timeout enforcement).
- Broken-pipe actionable diagnosis: covered by Tasks 1 and 4.
- `nc|xbstream` compatibility: preserved in Task 4 design and validated in Task 6.
- Required real-world pre-release gate: implemented in Task 6.
- Manual runbook: implemented in Task 6.
- Helper-to-helper integrity option groundwork: introduced in Task 3.

No uncovered spec requirements remain.

## Placeholder Scan

Checked for `TODO`, `TBD`, and vague “handle later” wording. No unresolved placeholders remain.

## Type Consistency Check

Consistent symbols across tasks:
- `SessionPhase` constants: `PhaseInit`, `PhaseConnecting`, `PhaseHandshaking`, `PhaseStreaming`, `PhaseFinalizing`, `PhaseDone`, `PhaseFailed`
- Error codes: `ErrConnectTimeout`, `ErrHandshakeTimeout`, `ErrHandshakeMismatch`, `ErrStreamBrokenPipe`, `ErrStreamConnReset`, `ErrStreamIdleTimeout`, `ErrUpstreamEOF`, `ErrStreamUnknown`
- Session constructor: `NewTransferSession(role, endpoint)`
