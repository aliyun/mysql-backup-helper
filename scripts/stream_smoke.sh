#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

BIN="${BIN:-$ROOT_DIR/backup-helper}"
WORK_BASE="${WORK_BASE:-/tmp/backup-helper-smoke-$(date +%Y%m%d%H%M%S)}"
mkdir -p "$WORK_BASE"

require_bin() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "[smoke][ERROR] missing binary: $1" >&2
    exit 1
  }
}

assert_contains() {
  local file="$1"
  local pattern="$2"
  if ! rg -n "$pattern" "$file" >/dev/null 2>&1; then
    echo "[smoke][ERROR] expected pattern '$pattern' in $file" >&2
    return 1
  fi
}

assert_not_zero() {
  local rc="$1"
  local name="$2"
  if [[ "$rc" -eq 0 ]]; then
    echo "[smoke][ERROR] expected non-zero exit code for $name, got 0" >&2
    return 1
  fi
}

assert_zero() {
  local rc="$1"
  local name="$2"
  if [[ "$rc" -ne 0 ]]; then
    echo "[smoke][ERROR] expected zero exit code for $name, got $rc" >&2
    return 1
  fi
}

prepare_valid_xbstream() {
  local out="$1"
  local srcdir="$2"
  rm -rf "$srcdir"
  mkdir -p "$srcdir"
  echo "smoke-$(date +%s)" > "$srcdir/file1.txt"
  head -c 1048576 /dev/urandom > "$srcdir/file2.bin"
  docker run --rm -v "$srcdir":/src -w /src percona/percona-xtrabackup:8.0 sh -lc 'xbstream -c file1.txt file2.bin' > "$out"
}

build_binary() {
  echo "[smoke] building backup-helper"
  go build -a -ldflags="-X 'backup-helper/internal/utils.BuildVersion=smoke'" -o "$BIN" ./cmd/backup-helper
}

run_success_case() {
  local port=39101
  local src="$WORK_BASE/success-src.xb"
  local dst="$WORK_BASE/success-dst.xb"
  local s_log="$WORK_BASE/success-send.log"
  local r_log="$WORK_BASE/success-recv.log"
  local srcdir="$WORK_BASE/success-srcdir"
  local key="smoke-success-key"

  prepare_valid_xbstream "$src" "$srcdir"

  ("$BIN" --download --stream-port "$port" --output "$dst" --enable-handshake --stream-key "$key" --estimated-size 2MB --log-file "$r_log") \
    >"$WORK_BASE/success-recv.out" 2>"$WORK_BASE/success-recv.err" &
  local rpid=$!
  sleep 1

  set +e
  "$BIN" --existed-backup "$src" --mode=stream --stream-host 127.0.0.1 --stream-port "$port" --enable-handshake --stream-key "$key" --estimated-size 2MB --log-file "$s_log" \
    >"$WORK_BASE/success-send.out" 2>"$WORK_BASE/success-send.err"
  local src_rc=$?
  wait "$rpid"
  local rcv_rc=$?
  set -e

  assert_zero "$src_rc" "success sender"
  assert_zero "$rcv_rc" "success receiver"

  local src_sha dst_sha
  src_sha="$(sha256sum "$src" | awk '{print $1}')"
  dst_sha="$(sha256sum "$dst" | awk '{print $1}')"
  if [[ "$src_sha" != "$dst_sha" ]]; then
    echo "[smoke][ERROR] success checksum mismatch: $src_sha != $dst_sha" >&2
    return 1
  fi

  assert_contains "$s_log" "transfer-summary.*phase=done"
  assert_contains "$r_log" "transfer-summary.*phase=done"
  echo "[smoke] success case passed"
}

run_handshake_mismatch_case() {
  local port=39102
  local src="$WORK_BASE/hs-src.xb"
  local dst="$WORK_BASE/hs-dst.xb"
  local s_log="$WORK_BASE/hs-send.log"
  local r_log="$WORK_BASE/hs-recv.log"
  local srcdir="$WORK_BASE/hs-srcdir"

  prepare_valid_xbstream "$src" "$srcdir"

  ("$BIN" --download --stream-port "$port" --output "$dst" --enable-handshake --stream-key good-key --log-file "$r_log") \
    >"$WORK_BASE/hs-recv.out" 2>"$WORK_BASE/hs-recv.err" &
  local rpid=$!
  sleep 1

  set +e
  "$BIN" --existed-backup "$src" --mode=stream --stream-host 127.0.0.1 --stream-port "$port" --enable-handshake --stream-key bad-key --log-file "$s_log" \
    >"$WORK_BASE/hs-send.out" 2>"$WORK_BASE/hs-send.err"
  local src_rc=$?
  wait "$rpid"
  local rcv_rc=$?
  set -e

  assert_not_zero "$src_rc" "handshake-mismatch sender"
  assert_not_zero "$rcv_rc" "handshake-mismatch receiver"
  assert_contains "$s_log" "ERR_HANDSHAKE_MISMATCH"
  assert_contains "$r_log" "ERR_HANDSHAKE_MISMATCH"
  echo "[smoke] handshake mismatch case passed"
}

run_broken_pipe_case() {
  local port=39103
  local src="$WORK_BASE/bp-src.xb"
  local dst="$WORK_BASE/bp-dst.xb"
  local s_log="$WORK_BASE/bp-send.log"
  local r_log="$WORK_BASE/bp-recv.log"
  local srcdir="$WORK_BASE/bp-srcdir"
  local key="smoke-bp-key"

  rm -rf "$srcdir"
  mkdir -p "$srcdir"
  head -c 134217728 /dev/urandom > "$srcdir/big.bin"
  docker run --rm -v "$srcdir":/src -w /src percona/percona-xtrabackup:8.0 sh -lc 'xbstream -c big.bin' > "$src"

  ("$BIN" --download --stream-port "$port" --output "$dst" --enable-handshake --stream-key "$key" --log-file "$r_log") \
    >"$WORK_BASE/bp-recv.out" 2>"$WORK_BASE/bp-recv.err" &
  local rpid=$!
  sleep 1

  ("$BIN" --existed-backup "$src" --mode=stream --stream-host 127.0.0.1 --stream-port "$port" --enable-handshake --stream-key "$key" --io-limit 10MB/s --log-file "$s_log") \
    >"$WORK_BASE/bp-send.out" 2>"$WORK_BASE/bp-send.err" &
  local spid=$!

  sleep 2
  kill "$rpid" >/dev/null 2>&1 || true

  set +e
  wait "$spid"
  local src_rc=$?
  wait "$rpid"
  local rcv_rc=$?
  set -e

  assert_not_zero "$src_rc" "broken-pipe sender"
  # Receiver may be terminated by signal; don't assert exact code.
  if [[ "$rcv_rc" -eq 0 ]]; then
    echo "[smoke][ERROR] expected receiver to be interrupted in broken-pipe case" >&2
    return 1
  fi

  if ! rg -n "ERR_STREAM_CONN_RESET|ERR_STREAM_BROKEN_PIPE" "$s_log" "$WORK_BASE/bp-send.err" >/dev/null 2>&1; then
    echo "[smoke][ERROR] expected broken-pipe/reset code in sender outputs" >&2
    return 1
  fi

  echo "[smoke] broken pipe case passed"
}

run_idle_timeout_case() {
  local port=39104
  local r_log="$WORK_BASE/idle-recv.log"

  set +e
  "$BIN" --download --stream-port "$port" --output "$WORK_BASE/idle-dst.xb" --timeout 2 --enable-handshake --stream-key idle-key --log-file "$r_log" \
    >"$WORK_BASE/idle-recv.out" 2>"$WORK_BASE/idle-recv.err"
  local rcv_rc=$?
  set -e

  assert_not_zero "$rcv_rc" "idle-timeout receiver"
  assert_contains "$r_log" "ERR_CONNECT_TIMEOUT"
  echo "[smoke] idle timeout case passed"
}

main() {
  require_bin go
  require_bin docker
  require_bin rg
  build_binary

  run_success_case
  run_handshake_mismatch_case
  run_broken_pipe_case
  run_idle_timeout_case

  echo "[smoke] all cases passed. Artifacts: $WORK_BASE"
}

main "$@"
