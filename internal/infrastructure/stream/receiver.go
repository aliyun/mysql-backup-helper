package stream

import (
	"backup-helper/internal/pkg/progress"
	"fmt"
	"io"
	"net"
	"os"
)

// Receiver handles TCP stream receiving
type Receiver struct {
	port            int
	enableHandshake bool
	handshakeKey    string
	totalSize       int64
}

// NewReceiver creates a new stream receiver
func NewReceiver(port int, enableHandshake bool, handshakeKey string, totalSize int64) *Receiver {
	return &Receiver{
		port:            port,
		enableHandshake: enableHandshake,
		handshakeKey:    handshakeKey,
		totalSize:       totalSize,
	}
}

// Start starts the TCP server and returns a reader for receiving data
func (r *Receiver) Start() (io.ReadCloser, *progress.Tracker, func(), int, string, error) {
	var addr string
	var actualPort int

	if r.port == 0 {
		// Auto-find available port
		var err error
		actualPort, err = GetAvailablePort()
		if err != nil {
			return nil, nil, nil, 0, "", fmt.Errorf("failed to find available port: %v", err)
		}
		addr = fmt.Sprintf(":%d", actualPort)
	} else {
		actualPort = r.port
		addr = fmt.Sprintf(":%d", r.port)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, nil, nil, 0, "", fmt.Errorf("failed to listen on port %d: %v", actualPort, err)
	}

	// Get local IP and display connection info
	localIP, err := GetLocalIP()
	if err != nil {
		localIP = "127.0.0.1" // fallback to localhost
	}

	fmt.Fprintf(os.Stderr, "[backup-helper] Listening on %s:%d\n", localIP, actualPort)
	fmt.Fprintf(os.Stderr, "[backup-helper] Waiting for remote connection...\n")

	// Create progress tracker for download mode
	tracker := progress.NewDownloadTracker(r.totalSize)

	if !r.enableHandshake {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Fprintf(os.Stderr, "[backup-helper] Remote client connected, no handshake required.\n")
		closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
		progressReader := progress.NewReader(conn, tracker, 64*1024)
		return &readCloserWrapper{Reader: progressReader, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}

	// With handshake
	for {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Fprintf(os.Stderr, "[backup-helper] Remote client connected, waiting for handshake...\n")

		if err := performHandshake(conn, r.handshakeKey); err != nil {
			conn.Close()
			continue
		}

		fmt.Fprintf(os.Stderr, "[backup-helper] Handshake OK, start receiving backup...\n")
		closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
		progressReader := progress.NewReader(conn, tracker, 64*1024)
		return &readCloserWrapper{Reader: progressReader, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}
}
