package utils

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
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

// StartStreamClient connects to a remote TCP server and returns a WriteCloser for pushing data.
// Similar to `nc host port`, this function actively connects to the remote server.
// Returns the remote address for display.
func StartStreamClient(host string, port int, enableHandshake bool, handshakeKey string, totalSize int64) (io.WriteCloser, *ProgressTracker, func(), string, error) {
	if host == "" {
		return nil, nil, nil, "", fmt.Errorf("stream-host cannot be empty")
	}
	if port <= 0 {
		return nil, nil, nil, "", fmt.Errorf("stream-port must be specified when using --stream-host")
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	fmt.Printf("[backup-helper] Connecting to %s...\n", addr)

	// Create progress tracker
	tracker := NewProgressTracker(totalSize)

	conn, err := net.DialTimeout("tcp", addr, 30*time.Second)
	if err != nil {
		return nil, nil, nil, "", fmt.Errorf("failed to connect to %s: %v", addr, err)
	}

	fmt.Printf("[backup-helper] Connected to %s\n", addr)

	if !enableHandshake {
		closer := func() { tracker.Complete(); conn.Close() }
		progressWriter := NewProgressWriter(conn, tracker)
		return struct {
			io.Writer
			io.Closer
		}{Writer: progressWriter, Closer: conn}, tracker, closer, addr, nil
	}

	// Send handshake
	handshakeMsg := handshakeKey + "\n"
	_, err = conn.Write([]byte(handshakeMsg))
	if err != nil {
		conn.Close()
		return nil, nil, nil, "", fmt.Errorf("failed to send handshake to %s: %v", addr, err)
	}

	// Wait for handshake response
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		return nil, nil, nil, "", fmt.Errorf("failed to receive handshake response from %s: %v", addr, err)
	}

	response = strings.TrimSpace(response)
	if response != "OK" && !strings.Contains(response, "OK") {
		conn.Close()
		return nil, nil, nil, "", fmt.Errorf("handshake failed: received '%s' from %s", response, addr)
	}

	conn.SetReadDeadline(time.Time{}) // cancel timeout
	fmt.Printf("[backup-helper] Handshake OK, start streaming backup to %s...\n", addr)

	closer := func() { tracker.Complete(); conn.Close() }
	progressWriter := NewProgressWriter(conn, tracker)
	return struct {
		io.Writer
		io.Closer
	}{Writer: progressWriter, Closer: conn}, tracker, closer, addr, nil
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

	fmt.Fprintf(os.Stderr, "[backup-helper] Listening on %s:%d\n", localIP, actualPort)
	fmt.Fprintf(os.Stderr, "[backup-helper] Waiting for remote connection...\n")

	// Create progress tracker for download mode
	tracker := NewDownloadProgressTracker(totalSize)

	if !enableHandshake {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, nil, 0, "", fmt.Errorf("failed to accept connection on port %d: %v", actualPort, err)
		}
		fmt.Fprintf(os.Stderr, "[backup-helper] Remote client connected, no handshake required.\n")
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
		fmt.Fprintf(os.Stderr, "[backup-helper] Remote client connected, waiting for handshake...\n")

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
				// Send OK response for handshake
				conn.Write([]byte("OK\n"))
				fmt.Fprintf(os.Stderr, "[backup-helper] Handshake OK, start receiving backup...\n")
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
