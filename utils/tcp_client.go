package utils

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// StartStreamServer starts a TCP server on the given port, accepts connections in a loop, and only returns the connection that sends the correct handshake.
func StartStreamServer(port int, enableHandshake bool, handshakeKey string) (io.WriteCloser, func(), error) {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to listen on port %d: %v", port, err)
	}
	fmt.Printf("[backup-helper] Waiting for remote connection on port %d...\n", port)

	if !enableHandshake {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, fmt.Errorf("failed to accept connection on port %d: %v", port, err)
		}
		fmt.Println("[backup-helper] Remote client connected, no handshake required.")
		closer := func() { conn.Close(); ln.Close() }
		return struct {
			io.Writer
			io.Closer
		}{Writer: conn, Closer: conn}, closer, nil
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			ln.Close()
			return nil, nil, fmt.Errorf("failed to accept connection on port %d: %v", port, err)
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
				closer := func() { conn.Close(); ln.Close() }
				return struct {
					io.Writer
					io.Closer
				}{Writer: conn, Closer: conn}, closer, nil
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
