package transfer

import (
	"backup-helper/internal/log"
	"backup-helper/internal/progress"
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
)

const (
	defaultTCPTimeoutSeconds = 60
	maxTCPTimeoutSeconds     = 3600
	handshakeTimeout         = 10 * time.Second
)

type sessionTrackingWriter struct {
	counter *CountingWriter
	session *TransferSession
}

func newSessionTrackingWriter(writer io.Writer, session *TransferSession) *sessionTrackingWriter {
	return &sessionTrackingWriter{
		counter: NewCountingWriter(writer),
		session: session,
	}
}

func (w *sessionTrackingWriter) Write(p []byte) (int, error) {
	n, err := w.counter.Write(p)
	if n > 0 {
		w.session.MarkProgress(int64(n))
	}
	return n, err
}

func (w *sessionTrackingWriter) BytesWritten() int64 {
	return w.counter.BytesWritten()
}

type sessionTrackingReader struct {
	counter *CountingReader
	session *TransferSession
}

func newSessionTrackingReader(reader io.Reader, session *TransferSession) *sessionTrackingReader {
	return &sessionTrackingReader{
		counter: NewCountingReader(reader),
		session: session,
	}
}

func (r *sessionTrackingReader) Read(p []byte) (int, error) {
	n, err := r.counter.Read(p)
	if n > 0 {
		r.session.MarkProgress(int64(n))
	}
	return n, err
}

func (r *sessionTrackingReader) BytesRead() int64 {
	return r.counter.BytesRead()
}

func normalizeTCPTimeoutSeconds(timeoutSeconds int) int {
	if timeoutSeconds <= 0 {
		return defaultTCPTimeoutSeconds
	}
	if timeoutSeconds > maxTCPTimeoutSeconds {
		return maxTCPTimeoutSeconds
	}
	return timeoutSeconds
}

func mapHandshakeErrToCode(err error) TransferErrorCode {
	if err == nil {
		return ""
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return ErrHandshakeTimeout
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrHandshakeTimeout
	}

	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") || strings.Contains(errStr, "i/o timeout") {
		return ErrHandshakeTimeout
	}

	if strings.Contains(errStr, "handshake failed") || strings.Contains(errStr, "invalid handshake") {
		return ErrHandshakeMismatch
	}

	if code := ClassifyTransferErrorWithPhase(err, PhaseHandshaking); code != ErrStreamUnknown {
		return code
	}

	return ErrStreamUnknown
}

func emitTCPTerminalSummary(logCtx *log.LogContext, session *TransferSession, bytes int64, code TransferErrorCode, cause string) {
	if session == nil {
		return
	}

	summary := BuildTerminalSummary(TransferSummary{
		SessionID: session.id,
		Role:      session.role,
		Endpoint:  session.endpoint,
		Phase:     session.Phase(),
		Code:      code,
		Bytes:     bytes,
		ElapsedMS: int64(time.Since(session.startedAt) / time.Millisecond),
		Cause:     cause,
	})

	fmt.Fprintln(os.Stderr, summary)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "%s", summary)
	}
}

func emitTCPFailure(logCtx *log.LogContext, session *TransferSession, bytes int64, code TransferErrorCode, err error) {
	if session == nil {
		return
	}

	session.Transition(PhaseFailed)
	cause := ""
	if err != nil {
		cause = err.Error()
	}
	emitTCPTerminalSummary(logCtx, session, bytes, code, cause)
}

func closeTCPStream(session *TransferSession, tracker *progress.ProgressTracker, conn net.Conn, ln net.Listener, logCtx *log.LogContext, bytes int64, progressErr error) {
	if session != nil && !session.TryAcquireCloseOwnership() {
		return
	}

	code := TransferErrorCode("")
	cause := ""
	if progressErr != nil && progressErr != io.EOF {
		code = ClassifyTransferErrorWithPhase(progressErr, PhaseStreaming)
		cause = progressErr.Error()
		if session != nil {
			session.Transition(PhaseFailed)
		}
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Transfer interrupted: %v", progressErr)
		}
	} else {
		if session != nil {
			session.Transition(PhaseFinalizing)
		}
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Transfer completed")
		}
	}

	if tracker != nil {
		tracker.Complete()
	}
	if conn != nil {
		_ = conn.Close()
	}
	if ln != nil {
		_ = ln.Close()
	}

	if session != nil && code == "" {
		session.Transition(PhaseDone)
	}

	emitTCPTerminalSummary(logCtx, session, bytes, code, cause)
}

func acceptListenerConnection(ln net.Listener, role string) (net.Conn, *TransferSession, error) {
	conn, err := ln.Accept()
	if err != nil {
		return nil, nil, err
	}

	session := NewTransferSession(role, conn.RemoteAddr().String())
	return conn, session, nil
}

// GetAvailablePort finds an available port by binding to port 0 and getting the assigned port
func GetAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port, nil
}

// GetLocalIP gets the local IP address (preferring non-loopback)
func GetLocalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

// StartStreamSender starts a TCP server on the given port for sending data.
// It accepts connections and returns a WriteCloser for writing data to the remote client.
// If port is 0, it will automatically find an available port.
// timeoutSeconds: connection timeout in seconds (0 means use default 60s, max 3600s)
// Returns the actual listening port and local IP for display.
func StartStreamSender(port int, enableHandshake bool, handshakeKey string, totalSize int64, isCompressed bool, timeoutSeconds int, logCtx *log.LogContext) (io.WriteCloser, *progress.ProgressTracker, func(), int, string, error) {
	var addr string
	var actualPort int

	if port == 0 {
		var err error
		actualPort, err = GetAvailablePort()
		if err != nil {
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Failed to find available port: %v", err)
			}
			return nil, nil, nil, 0, "", fmt.Errorf("failed to find available port: %v", err)
		}
		addr = fmt.Sprintf(":%d", actualPort)
	} else {
		actualPort = port
		addr = fmt.Sprintf(":%d", port)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to listen on port %d: %v", actualPort, err)
		}
		return nil, nil, nil, 0, "", fmt.Errorf("failed to listen on port %d: %v", actualPort, err)
	}

	localIP, err := GetLocalIP()
	if err != nil {
		localIP = "127.0.0.1"
	}

	listenEndpoint := fmt.Sprintf("%s:%d", localIP, actualPort)
	listenSession := NewTransferSession("sender", listenEndpoint)
	listenSession.Transition(PhaseListening)

	timeoutSeconds = normalizeTCPTimeoutSeconds(timeoutSeconds)
	fmt.Printf("[backup-helper] Listening on %s:%d\n", localIP, actualPort)
	fmt.Printf("[backup-helper] Waiting for remote connection (timeout: %ds)...\n", timeoutSeconds)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Listening on %s:%d", localIP, actualPort)
		logCtx.WriteLog("TCP", "Waiting for remote connection (timeout: %ds)", timeoutSeconds)
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	_ = ln.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))

	tracker := progress.NewProgressTrackerWithCompression(totalSize, isCompressed)

	if !enableHandshake {
		conn, session, err := acceptListenerConnection(ln, "sender")
		if err != nil {
			ln.Close()
			code := ClassifyTransferErrorWithPhase(err, listenSession.Phase())
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if logCtx != nil {
					logCtx.WriteLog("TCP", "Connection timeout after %ds: %v", timeoutSeconds, err)
				}
				emitTCPFailure(logCtx, listenSession, 0, code, err)
				return nil, nil, nil, 0, "", fmt.Errorf("connection timeout after %ds on port %d: %v", timeoutSeconds, actualPort, err)
			}
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Failed to accept connection: %v", err)
			}
			emitTCPFailure(logCtx, listenSession, 0, code, err)
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}

		fmt.Println("[backup-helper] Remote client connected, no handshake required.")
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Remote client connected, no handshake required")
			logCtx.WriteLog("TCP", "Transfer started")
		}

		session.Transition(PhaseStreaming)
		trackedWriter := newSessionTrackingWriter(conn, session)
		progressWriter := progress.NewProgressWriter(trackedWriter, tracker)
		closer := func() {
			closeTCPStream(session, tracker, conn, ln, logCtx, trackedWriter.BytesWritten(), progressWriter.GetError())
		}
		return struct {
			io.Writer
			io.Closer
		}{Writer: progressWriter, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}

	for {
		conn, session, err := acceptListenerConnection(ln, "sender")
		if err != nil {
			ln.Close()
			code := ClassifyTransferErrorWithPhase(err, listenSession.Phase())
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if logCtx != nil {
					logCtx.WriteLog("TCP", "Connection timeout after %ds: %v", timeoutSeconds, err)
				}
				emitTCPFailure(logCtx, listenSession, 0, code, err)
				return nil, nil, nil, 0, "", fmt.Errorf("connection timeout after %ds on port %d: %v", timeoutSeconds, actualPort, err)
			}
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Failed to accept connection: %v", err)
			}
			emitTCPFailure(logCtx, listenSession, 0, code, err)
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}

		fmt.Println("[backup-helper] Remote client connected, waiting for handshake...")
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Remote client connected, waiting for handshake")
		}

		session.Transition(PhaseHandshaking)
		_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			_, _ = conn.Write([]byte("Please send handshake\n"))
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Handshake failed: %v", err)
			}
			emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
			_ = conn.Close()
			continue
		}

		line = strings.TrimSpace(line)
		if line != handshakeKey {
			_, _ = conn.Write([]byte("Invalid handshake. Send the correct handshake to begin streaming.\n"))
			mismatchErr := fmt.Errorf("handshake failed: received '%s' from %s, expected '%s'", line, conn.RemoteAddr().String(), handshakeKey)
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Handshake failed: received '%s'", line)
			}
			emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(mismatchErr), mismatchErr)
			_ = conn.Close()
			continue
		}

		_ = conn.SetReadDeadline(time.Time{})
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Handshake OK, transfer started")
		}

		session.Transition(PhaseStreaming)
		trackedWriter := newSessionTrackingWriter(conn, session)
		progressWriter := progress.NewProgressWriter(trackedWriter, tracker)
		closer := func() {
			closeTCPStream(session, tracker, conn, ln, logCtx, trackedWriter.BytesWritten(), progressWriter.GetError())
		}
		return struct {
			io.Writer
			io.Closer
		}{Writer: progressWriter, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}
}

// StartStreamClient connects to a remote TCP server and returns a WriteCloser for pushing data.
// Similar to `nc host port`, this function actively connects to the remote server.
// Returns the remote address for display.
func StartStreamClient(host string, port int, enableHandshake bool, handshakeKey string, totalSize int64, isCompressed bool, logCtx *log.LogContext) (io.WriteCloser, *progress.ProgressTracker, func(), string, error) {
	if host == "" {
		return nil, nil, nil, "", fmt.Errorf("stream-host cannot be empty")
	}
	if port <= 0 {
		return nil, nil, nil, "", fmt.Errorf("stream-port must be specified when using --stream-host")
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	session := NewTransferSession("sender", addr)
	fmt.Printf("[backup-helper] Connecting to %s...\n", addr)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Connecting to %s", addr)
	}

	tracker := progress.NewProgressTrackerWithCompression(totalSize, isCompressed)

	session.Transition(PhaseConnecting)
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to connect to %s: %v", addr, err)
		}
		emitTCPFailure(logCtx, session, 0, ClassifyTransferErrorWithPhase(err, session.Phase()), err)
		return nil, nil, nil, "", fmt.Errorf("failed to connect to %s: %v", addr, err)
	}

	fmt.Printf("[backup-helper] Connected to %s\n", addr)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Connected to %s", addr)
	}

	if !enableHandshake {
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Transfer started (no handshake)")
		}

		session.Transition(PhaseStreaming)
		trackedWriter := newSessionTrackingWriter(conn, session)
		progressWriter := progress.NewProgressWriter(trackedWriter, tracker)
		closer := func() {
			closeTCPStream(session, tracker, conn, nil, logCtx, trackedWriter.BytesWritten(), progressWriter.GetError())
		}
		return struct {
			io.Writer
			io.Closer
		}{Writer: progressWriter, Closer: conn}, tracker, closer, addr, nil
	}

	session.Transition(PhaseHandshaking)
	handshakeMsg := handshakeKey + "\n"
	_, err = conn.Write([]byte(handshakeMsg))
	if err != nil {
		_ = conn.Close()
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to send handshake: %v", err)
		}
		emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
		return nil, nil, nil, "", fmt.Errorf("failed to send handshake to %s: %v", addr, err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to receive handshake response: %v", err)
		}
		emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
		return nil, nil, nil, "", fmt.Errorf("failed to receive handshake response from %s: %v", addr, err)
	}

	response = strings.TrimSpace(response)
	if response != "OK" && !strings.Contains(response, "OK") {
		err = fmt.Errorf("handshake failed: received '%s' from %s", response, addr)
		_ = conn.Close()
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Handshake failed: received '%s'", response)
		}
		emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
		return nil, nil, nil, "", err
	}

	_ = conn.SetReadDeadline(time.Time{})
	fmt.Printf("[backup-helper] Handshake OK, start streaming backup to %s...\n", addr)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Handshake OK, transfer started")
	}

	session.Transition(PhaseStreaming)
	trackedWriter := newSessionTrackingWriter(conn, session)
	progressWriter := progress.NewProgressWriter(trackedWriter, tracker)
	closer := func() {
		closeTCPStream(session, tracker, conn, nil, logCtx, trackedWriter.BytesWritten(), progressWriter.GetError())
	}
	return struct {
		io.Writer
		io.Closer
	}{Writer: progressWriter, Closer: conn}, tracker, closer, addr, nil
}

// StartStreamClientReader starts a TCP client connection to the given host:port for reading data.
// It actively connects to the remote server and returns a ReadCloser for reading data.
// If handshake is enabled, it will wait for the server's handshake message and respond with "OK".
// Returns the remote address for display.
func StartStreamClientReader(host string, port int, enableHandshake bool, handshakeKey string, totalSize int64, isCompressed bool, logCtx *log.LogContext) (io.ReadCloser, *progress.ProgressTracker, func(), string, error) {
	if host == "" {
		return nil, nil, nil, "", fmt.Errorf("stream-host cannot be empty")
	}
	if port <= 0 {
		return nil, nil, nil, "", fmt.Errorf("stream-port must be specified when using --stream-host")
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	session := NewTransferSession("receiver", addr)
	fmt.Printf("[backup-helper] Connecting to %s...\n", addr)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Connecting to %s", addr)
	}

	tracker := progress.NewDownloadProgressTracker(totalSize)

	session.Transition(PhaseConnecting)
	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to connect to %s: %v", addr, err)
		}
		emitTCPFailure(logCtx, session, 0, ClassifyTransferErrorWithPhase(err, session.Phase()), err)
		return nil, nil, nil, "", fmt.Errorf("failed to connect to %s: %v", addr, err)
	}

	fmt.Printf("[backup-helper] Connected to %s\n", addr)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Connected to %s", addr)
	}

	if !enableHandshake {
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Transfer started (no handshake)")
		}

		session.Transition(PhaseStreaming)
		trackedReader := newSessionTrackingReader(conn, session)
		progressReader := progress.NewProgressReader(trackedReader, tracker, 64*1024)
		closer := func() {
			closeTCPStream(session, tracker, conn, nil, logCtx, trackedReader.BytesRead(), progressReader.GetError())
		}
		return struct {
			io.Reader
			io.Closer
		}{Reader: progressReader, Closer: conn}, tracker, closer, addr, nil
	}

	session.Transition(PhaseHandshaking)
	_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
	reader := bufio.NewReader(conn)
	handshakeMsg, err := reader.ReadString('\n')
	if err != nil {
		_ = conn.Close()
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to receive handshake: %v", err)
		}
		emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
		return nil, nil, nil, "", fmt.Errorf("failed to receive handshake from %s: %v", addr, err)
	}

	handshakeMsg = strings.TrimSpace(handshakeMsg)
	if handshakeMsg != handshakeKey {
		err = fmt.Errorf("handshake failed: received '%s' from %s, expected '%s'", handshakeMsg, addr, handshakeKey)
		_ = conn.Close()
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Handshake failed: received '%s', expected '%s'", handshakeMsg, handshakeKey)
		}
		emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
		return nil, nil, nil, "", err
	}

	_, err = conn.Write([]byte("OK\n"))
	if err != nil {
		_ = conn.Close()
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to send handshake response: %v", err)
		}
		emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
		return nil, nil, nil, "", fmt.Errorf("failed to send handshake response to %s: %v", addr, err)
	}

	_ = conn.SetReadDeadline(time.Time{})
	fmt.Printf("[backup-helper] Handshake OK, start receiving backup from %s...\n", addr)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Handshake OK, transfer started")
	}

	session.Transition(PhaseStreaming)
	trackedReader := newSessionTrackingReader(reader, session)
	progressReader := progress.NewProgressReader(trackedReader, tracker, 64*1024)
	closer := func() {
		closeTCPStream(session, tracker, conn, nil, logCtx, trackedReader.BytesRead(), progressReader.GetError())
	}
	return struct {
		io.Reader
		io.Closer
	}{Reader: progressReader, Closer: conn}, tracker, closer, addr, nil
}

// StartStreamReceiver starts a TCP server on the given port for receiving data.
// It accepts connections and returns a ReadCloser for reading data from the remote client.
// If port is 0, it will automatically find an available port.
// timeoutSeconds: connection timeout in seconds (0 means use default 60s, max 3600s)
// Returns the actual listening port and local IP for display.
func StartStreamReceiver(port int, enableHandshake bool, handshakeKey string, totalSize int64, isCompressed bool, timeoutSeconds int, logCtx *log.LogContext) (io.ReadCloser, *progress.ProgressTracker, func(), int, string, error) {
	var addr string
	var actualPort int

	if port == 0 {
		var err error
		actualPort, err = GetAvailablePort()
		if err != nil {
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Failed to find available port: %v", err)
			}
			return nil, nil, nil, 0, "", fmt.Errorf("failed to find available port: %v", err)
		}
		addr = fmt.Sprintf(":%d", actualPort)
	} else {
		actualPort = port
		addr = fmt.Sprintf(":%d", port)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Failed to listen on port %d: %v", actualPort, err)
		}
		return nil, nil, nil, 0, "", fmt.Errorf("failed to listen on port %d: %v", actualPort, err)
	}

	localIP, err := GetLocalIP()
	if err != nil {
		localIP = "127.0.0.1"
	}

	listenEndpoint := fmt.Sprintf("%s:%d", localIP, actualPort)
	listenSession := NewTransferSession("receiver", listenEndpoint)
	listenSession.Transition(PhaseListening)

	timeoutSeconds = normalizeTCPTimeoutSeconds(timeoutSeconds)
	fmt.Fprintf(os.Stderr, "[backup-helper] Listening on %s:%d\n", localIP, actualPort)
	fmt.Fprintf(os.Stderr, "[backup-helper] Waiting for remote connection (timeout: %ds)...\n", timeoutSeconds)
	if logCtx != nil {
		logCtx.WriteLog("TCP", "Listening on %s:%d", localIP, actualPort)
		logCtx.WriteLog("TCP", "Waiting for remote connection (timeout: %ds)", timeoutSeconds)
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	_ = ln.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))

	tracker := progress.NewDownloadProgressTrackerWithCompression(totalSize, isCompressed)

	if !enableHandshake {
		conn, session, err := acceptListenerConnection(ln, "receiver")
		if err != nil {
			ln.Close()
			code := ClassifyTransferErrorWithPhase(err, listenSession.Phase())
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if logCtx != nil {
					logCtx.WriteLog("TCP", "Connection timeout after %ds: %v", timeoutSeconds, err)
				}
				emitTCPFailure(logCtx, listenSession, 0, code, err)
				return nil, nil, nil, 0, "", fmt.Errorf("connection timeout after %ds on port %d: %v", timeoutSeconds, actualPort, err)
			}
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Failed to accept connection: %v", err)
			}
			emitTCPFailure(logCtx, listenSession, 0, code, err)
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}

		fmt.Fprintf(os.Stderr, "[backup-helper] Remote client connected, no handshake required.\n")
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Remote client connected, no handshake required")
			logCtx.WriteLog("TCP", "Transfer started")
		}

		session.Transition(PhaseStreaming)
		trackedReader := newSessionTrackingReader(conn, session)
		progressReader := progress.NewProgressReader(trackedReader, tracker, 64*1024)
		closer := func() {
			closeTCPStream(session, tracker, conn, ln, logCtx, trackedReader.BytesRead(), progressReader.GetError())
		}
		return struct {
			io.Reader
			io.Closer
		}{Reader: progressReader, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}

	for {
		conn, session, err := acceptListenerConnection(ln, "receiver")
		if err != nil {
			ln.Close()
			code := ClassifyTransferErrorWithPhase(err, listenSession.Phase())
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				if logCtx != nil {
					logCtx.WriteLog("TCP", "Connection timeout after %ds: %v", timeoutSeconds, err)
				}
				emitTCPFailure(logCtx, listenSession, 0, code, err)
				return nil, nil, nil, 0, "", fmt.Errorf("connection timeout after %ds on port %d: %v", timeoutSeconds, actualPort, err)
			}
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Failed to accept connection: %v", err)
			}
			emitTCPFailure(logCtx, listenSession, 0, code, err)
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}

		fmt.Fprintf(os.Stderr, "[backup-helper] Remote client connected, waiting for handshake...\n")
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Remote client connected, waiting for handshake")
		}

		session.Transition(PhaseHandshaking)
		_ = conn.SetReadDeadline(time.Now().Add(handshakeTimeout))
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			_, _ = conn.Write([]byte("Please send handshake\n"))
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Handshake failed: %v", err)
			}
			emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
			_ = conn.Close()
			continue
		}

		line = strings.TrimSpace(line)
		if line != handshakeKey {
			_, _ = conn.Write([]byte("Invalid handshake. Send the correct handshake to begin streaming.\n"))
			mismatchErr := fmt.Errorf("handshake failed: received '%s' from %s, expected '%s'", line, conn.RemoteAddr().String(), handshakeKey)
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Handshake failed: received '%s'", line)
			}
			emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(mismatchErr), mismatchErr)
			_ = conn.Close()
			continue
		}

		_, err = conn.Write([]byte("OK\n"))
		if err != nil {
			if logCtx != nil {
				logCtx.WriteLog("TCP", "Failed to send handshake response: %v", err)
			}
			emitTCPFailure(logCtx, session, 0, mapHandshakeErrToCode(err), err)
			_ = conn.Close()
			continue
		}

		_ = conn.SetReadDeadline(time.Time{})
		fmt.Fprintf(os.Stderr, "[backup-helper] Handshake OK, start receiving backup...\n")
		if logCtx != nil {
			logCtx.WriteLog("TCP", "Handshake OK, transfer started")
		}

		session.Transition(PhaseStreaming)
		trackedReader := newSessionTrackingReader(reader, session)
		progressReader := progress.NewProgressReader(trackedReader, tracker, 64*1024)
		closer := func() {
			closeTCPStream(session, tracker, conn, ln, logCtx, trackedReader.BytesRead(), progressReader.GetError())
		}
		return struct {
			io.Reader
			io.Closer
		}{Reader: progressReader, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}
}
