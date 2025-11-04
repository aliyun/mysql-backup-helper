package utils

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

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
// Returns the actual listening port and local IP for display.
func StartStreamSender(port int, enableHandshake bool, handshakeKey string, totalSize int64) (io.WriteCloser, *ProgressTracker, func(), int, string, error) {
	var addr string
	var actualPort int

	if port == 0 {
		// Auto-find available port
		var err error
		actualPort, err = GetAvailablePort()
		if err != nil {
			return nil, nil, nil, 0, "", fmt.Errorf("failed to find available port: %v", err)
		}
		addr = fmt.Sprintf(":%d", actualPort)
	} else {
		actualPort = port
		addr = fmt.Sprintf(":%d", port)
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
	tracker := NewProgressTracker(totalSize)

	if !enableHandshake {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Println("[backup-helper] Remote client connected, no handshake required.")
		closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
		progressWriter := NewProgressWriter(conn, tracker)
		return struct {
			io.Writer
			io.Closer
		}{Writer: progressWriter, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Println("[backup-helper] Remote client connected, waiting for handshake...")

		goAway := false
		// set timeout to prevent from hanging
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Write([]byte("Please send handshake\n"))
			goAway = true
		} else {
			line = strings.TrimSpace(line)
			if line == handshakeKey {
				conn.SetReadDeadline(time.Time{}) // cancel timeout
				fmt.Println("[backup-helper] Handshake OK, start streaming backup...")
				closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
				progressWriter := NewProgressWriter(conn, tracker)
				return struct {
					io.Writer
					io.Closer
				}{Writer: progressWriter, Closer: conn}, tracker, closer, actualPort, localIP, nil
			} else {
				conn.Write([]byte("Invalid handshake. Send the correct handshake to begin streaming.\n"))
				goAway = true
			}
		}
		if goAway {
			conn.Close()
			continue
		}
	}
}

// StartStreamReceiver starts a TCP server on the given port for receiving data.
// It accepts connections and returns a ReadCloser for reading data from the remote client.
// If port is 0, it will automatically find an available port.
// Returns the actual listening port and local IP for display.
func StartStreamReceiver(port int, enableHandshake bool, handshakeKey string, totalSize int64) (io.ReadCloser, *ProgressTracker, func(), int, string, error) {
	var addr string
	var actualPort int

	if port == 0 {
		// Auto-find available port
		var err error
		actualPort, err = GetAvailablePort()
		if err != nil {
			return nil, nil, nil, 0, "", fmt.Errorf("failed to find available port: %v", err)
		}
		addr = fmt.Sprintf(":%d", actualPort)
	} else {
		actualPort = port
		addr = fmt.Sprintf(":%d", port)
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

	// Create progress tracker for download mode
	tracker := NewDownloadProgressTracker(totalSize)

	if !enableHandshake {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Println("[backup-helper] Remote client connected, no handshake required.")
		closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
		progressReader := NewProgressReader(conn, tracker, 64*1024)
		return struct {
			io.Reader
			io.Closer
		}{Reader: progressReader, Closer: conn}, tracker, closer, actualPort, localIP, nil
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Println("[backup-helper] Remote client connected, waiting for handshake...")

		goAway := false
		// set timeout to prevent from hanging
		conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		reader := bufio.NewReader(conn)
		line, err := reader.ReadString('\n')
		if err != nil {
			conn.Write([]byte("Please send handshake\n"))
			goAway = true
		} else {
			line = strings.TrimSpace(line)
			if line == handshakeKey {
				conn.SetReadDeadline(time.Time{}) // cancel timeout
				fmt.Println("[backup-helper] Handshake OK, start receiving backup...")
				closer := func() { tracker.Complete(); conn.Close(); ln.Close() }
				progressReader := NewProgressReader(conn, tracker, 64*1024)
				return struct {
					io.Reader
					io.Closer
				}{Reader: progressReader, Closer: conn}, tracker, closer, actualPort, localIP, nil
			} else {
				conn.Write([]byte("Invalid handshake. Send the correct handshake to begin streaming.\n"))
				goAway = true
			}
		}
		if goAway {
			conn.Close()
			continue
		}
	}
}
