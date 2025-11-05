package stream

import (
	"backup-helper/internal/pkg/progress"
	"fmt"
	"io"
	"net"
)

// Sender handles TCP stream sending
type Sender struct {
	port            int
	enableHandshake bool
	handshakeKey    string
	totalSize       int64
}

// NewSender creates a new stream sender
func NewSender(port int, enableHandshake bool, handshakeKey string, totalSize int64) *Sender {
	return &Sender{
		port:            port,
		enableHandshake: enableHandshake,
		handshakeKey:    handshakeKey,
		totalSize:       totalSize,
	}
}

// Start starts the TCP server and returns a writer for sending data
func (s *Sender) Start() (io.WriteCloser, *progress.Tracker, func(), int, string, error) {
	var addr string
	var actualPort int

	if s.port == 0 {
		// Auto-find available port
		var err error
		actualPort, err = GetAvailablePort()
		if err != nil {
			return nil, nil, nil, 0, "", fmt.Errorf("failed to find available port: %v", err)
		}
		addr = fmt.Sprintf(":%d", actualPort)
	} else {
		actualPort = s.port
		addr = fmt.Sprintf(":%d", s.port)
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

	fmt.Printf("[backup-helper] Listening on %s:%d\n", localIP, actualPort)
	fmt.Printf("[backup-helper] Waiting for remote connection...\n")

	// Create progress tracker
	tracker := progress.NewTracker(s.totalSize)

	if !s.enableHandshake {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Println("[backup-helper] Remote client connected, no handshake required.")
		closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
		progressWriter := progress.NewWriter(conn, tracker)
		return &writeCloserWrapper{Writer: progressWriter, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}

	// With handshake
	for {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Println("[backup-helper] Remote client connected, waiting for handshake...")

		if err := performHandshake(conn, s.handshakeKey); err != nil {
			conn.Close()
			continue
		}

		fmt.Println("[backup-helper] Handshake OK, start streaming backup...")
		closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
		progressWriter := progress.NewWriter(conn, tracker)
		return &writeCloserWrapper{Writer: progressWriter, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}
}
