package stream

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

// performHandshake performs handshake authentication
func performHandshake(conn net.Conn, handshakeKey string) error {
	// set timeout to prevent from hanging
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		conn.Write([]byte("Please send handshake\n"))
		return fmt.Errorf("failed to read handshake: %v", err)
	}

	line = strings.TrimSpace(line)
	if line != handshakeKey {
		conn.Write([]byte("Invalid handshake. Send the correct handshake to begin streaming.\n"))
		return fmt.Errorf("invalid handshake")
	}

	conn.SetReadDeadline(time.Time{}) // cancel timeout
	return nil
}

// writeCloserWrapper wraps a writer with a closer
type writeCloserWrapper struct {
	io.Writer
	io.Closer
}

// readCloserWrapper wraps a reader with a closer
type readCloserWrapper struct {
	io.Reader
	io.Closer
}

// printConnectionInfo prints connection information
func printConnectionInfo(localIP string, actualPort int, outputStream *os.File) {
	if outputStream == nil {
		outputStream = os.Stdout
	}
	fmt.Fprintf(outputStream, "[backup-helper] Listening on %s:%d\n", localIP, actualPort)
	fmt.Fprintf(outputStream, "[backup-helper] Waiting for remote connection...\n")
}
