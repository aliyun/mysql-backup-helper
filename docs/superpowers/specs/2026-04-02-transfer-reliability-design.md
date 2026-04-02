# Transfer Reliability Refactor Design

Date: 2026-04-02  
Topic: `internal/transfer/*` reliability and diagnosability for hang/broken-pipe issues

## 1. Goals

### Primary goals
- Eliminate silent hangs in stream transfer paths.
- Make broken-pipe and connection failures actionable with clear root-cause signals.
- Preserve compatibility with existing remote receiver usage:
  - `echo "<stream_key>" | nc <ip> <port> | xbstream ...`

### Explicit non-goals (phase 1)
- No automatic retry of mid-stream transfer.
- No resumable transfer protocol.
- No protocol change that breaks existing `nc|xbstream` scripts.

## 2. Scope

In scope:
- `internal/transfer/tcp.go`
- `internal/transfer/ssh.go` (for startup/receiver parsing reliability signals)
- New transfer reliability internals under `internal/transfer/`.

Out of scope:
- Backup extraction internals (`internal/extract/*`) except for consuming clearer transfer errors.
- OSS multipart upload behavior changes.

## 3. Architecture

Introduce a shared transfer reliability core:

### 3.1 New internal units
- `session.go`
  - `TransferSession` state machine.
  - Central phase transitions and close ownership.
- `diagnostics.go`
  - Structured transfer event writer.
  - Consistent error code mapping + summary formatter.
- `integrity.go`
  - Byte-counter wrappers for reader/writer.
  - Optional digest support for `backup-helper <-> backup-helper` finalize path.

### 3.2 Session state model
- `Init`
- `Listening` or `Connecting`
- `Handshaking`
- `Streaming`
- `Finalizing`
- Terminal: `Done` or `Failed`

Each transition records timestamp, peer address, and contextual metadata.

### 3.3 Compatibility strategy
- Keep public transfer entrypoints unchanged:
  - `StartStreamSender`
  - `StartStreamClient`
  - `StartStreamReceiver`
  - `StartStreamClientReader`
- Existing CLI flags remain valid.
- For `backup-helper -> nc|xbstream`, no wire-protocol additions are required.
- For `backup-helper <-> backup-helper`, optional finalize metadata exchange can be enabled when both sides support it.

## 4. Data Flow (Problem Path First)

Target path: `backup` stream sender to remote `nc|xbstream` receiver.

1. Create session ID and initialize `TransferSession`.
2. Connect/listen phase with bounded timeout and event logging.
3. Handshake phase (if enabled) with explicit success/failure logging.
4. Streaming phase wraps connection writer with:
- byte accounting,
- last-progress timestamp,
- write-side idle detection,
- classified transport error capture.
5. Finalization records:
- bytes sent,
- upstream producer status (`xtrabackup` pipeline outcome where known),
- close reason and close order.
6. Emit one structured terminal summary line for fast diagnosis.

Success criteria for this flow:
- No indefinite wait in connect/handshake/stream idle states.
- Transfer failure messages always include phase + error code + byte snapshot.

## 5. Error Handling Rules

### 5.1 Timeouts
- Connect timeout: existing configured timeout behavior retained with explicit error code.
- Handshake timeout: hard-fail with `ERR_HANDSHAKE_TIMEOUT`.
- Stream idle timeout: hard-fail with `ERR_STREAM_IDLE_TIMEOUT` when no progress for configured interval.

### 5.2 Error taxonomy
Canonical codes:
- `ERR_CONNECT_TIMEOUT`
- `ERR_HANDSHAKE_TIMEOUT`
- `ERR_HANDSHAKE_MISMATCH`
- `ERR_STREAM_BROKEN_PIPE`
- `ERR_STREAM_CONN_RESET`
- `ERR_STREAM_IDLE_TIMEOUT`
- `ERR_UPSTREAM_EOF_UNEXPECTED`
- `ERR_STREAM_UNKNOWN`

### 5.3 Close semantics
- Session object is sole owner of terminal close sequence.
- All code paths report close reason once.
- Best-effort close failures are logged as secondary details, not root cause.

### 5.4 Broken-pipe diagnostics payload
Every broken-pipe summary must include:
- session ID
- role (`sender`/`receiver`)
- peer endpoint
- active phase at failure
- bytes transferred up to failure
- elapsed time + time since last successful progress
- normalized error code and original error text

## 6. Integrity and Completeness Strategy

Given current stream architecture, safe mid-stream resume is not supported in phase 1.

Completeness is ensured via strict terminal checks:
- Transport must end without fatal transfer error.
- Upstream/downstream process in pipeline must report success where observable.
- Byte counters are captured and logged in terminal summary.

Optional (helper-to-helper only) phase 1 enhancement:
- finalize exchange with byte count + digest when both peers support it.

For `nc|xbstream` compatibility path:
- Do not require digest exchange.
- Treat extraction process exit status and transfer terminal status as required completeness signals.

## 7. Testing and Validation

Validation is split into both required and optional tracks.

### 7.1 Required pre-release gate (must pass)
1. `go test ./...`
2. `make build`
3. Real-world two-terminal streaming smoke tests with actual tools:
- Terminal A (sender): run backup stream command.
- Terminal B (receiver): run `echo "<stream_key>" | nc <ip> <port> | xbstream ...` path.
- Scenarios:
  - normal completion,
  - intentional receiver interruption mid-stream,
  - handshake mismatch,
  - induced idle timeout/stall.
- Evidence captured:
  - sender/receiver logs,
  - terminal summary lines with error codes,
  - receiver-side process exit codes.

Release block rule:
- Any scenario with ambiguous root-cause output or silent hang fails the gate.

### 7.2 Manual runbook (kept in repo/docs)
Provide a concise operator runbook containing:
- exact sender/receiver command templates,
- required prerequisites (`xtrabackup`, `xbstream`, `nc`),
- expected success/failure signatures,
- troubleshooting checklist by error code,
- artifact collection checklist (logs + exit codes + timestamps).

### 7.3 Unit/integration tests (supporting)
Add focused `internal/transfer/*_test.go` coverage for:
- state transitions,
- timeout behavior,
- error classification,
- terminal summary formatting.

These tests support regression safety but do not replace the required real-world gate.

## 8. Implementation Plan Constraints (for next planning phase)

- Keep public function signatures stable unless absolutely required.
- Prefer adding internal helpers over large rewrites in one commit.
- Introduce feature toggles only if needed to avoid behavioral risk.
- Preserve current default CLI behavior for existing users.

## 9. Risks and Mitigations

Risk: additional timeout logic causes false positives on slow links.  
Mitigation: use configurable idle timeout with conservative default and clear override guidance.

Risk: refactor changes close ordering and introduces regressions.  
Mitigation: single close owner + deterministic session terminal handling + tests for repeated close calls.

Risk: mixed protocol peers misinterpret finalize metadata.  
Mitigation: keep metadata exchange optional and enabled only when peer capability is confirmed.

## 10. Acceptance Criteria

The refactor is accepted when:
- No silent hang remains in connect/handshake/stream idle phases.
- Broken-pipe and reset errors are reported with normalized error code + actionable context.
- Existing `nc|xbstream` workflow remains functional without command changes.
- Required real-world gate scenarios pass with unambiguous diagnostics.

