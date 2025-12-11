package transfer

import (
	"backup-helper/internal/utils"
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// StartRemoteReceiverViaSSH starts backup-helper receiver on remote host via SSH
// If port > 0, uses that port; if port == 0, auto-finds available port
// Returns the port number where receiver is listening and the output path
func StartRemoteReceiverViaSSH(
	sshHost string,
	port int, // If > 0, use this port; if 0, auto-find
	remoteOutput string,
	estimatedSize int64,
	enableHandshake bool,
	handshakeKey string,
) (int, string, *exec.Cmd, func() error, error) {

	// Build remote backup-helper command
	remoteCmd := []string{"backup-helper", "--download"}

	if port > 0 {
		remoteCmd = append(remoteCmd, fmt.Sprintf("--stream-port=%d", port))
	} else {
		remoteCmd = append(remoteCmd, "--stream-port=0") // Auto-find
	}

	if remoteOutput != "" {
		remoteCmd = append(remoteCmd, "--output", remoteOutput)
	}

	if estimatedSize > 0 {
		remoteCmd = append(remoteCmd, "--estimated-size", utils.FormatBytes(estimatedSize))
	}

	if enableHandshake {
		remoteCmd = append(remoteCmd, "--enable-handshake")
		if handshakeKey != "" {
			remoteCmd = append(remoteCmd, "--stream-key", handshakeKey)
		}
	}

	// Disable rate limiting on remote receiver, rate limiting is handled on sender side
	remoteCmd = append(remoteCmd, "--io-limit=-1")

	// Execute SSH command (rely on system SSH config)
	cmd := exec.Command("ssh", sshHost, strings.Join(remoteCmd, " "))

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 0, "", nil, nil, fmt.Errorf("failed to create stdout pipe: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 0, "", nil, nil, fmt.Errorf("failed to create stderr pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		return 0, "", nil, nil, fmt.Errorf("failed to start SSH: %v", err)
	}

	// Parse output to find port and output path (from stderr)
	actualPort, outputPath, err := parseReceiverInfo(stderr, port, remoteOutput)
	if err != nil {
		cmd.Process.Kill()
		return 0, "", nil, nil, fmt.Errorf("failed to parse receiver info: %v", err)
	}

	// IMPORTANT: Start goroutines to consume stdout/stderr to prevent buffer blocking
	// After parsing the port, the remote process will continue to output progress info
	// to stderr. If we don't consume it, the buffer will fill up and block the remote process.
	go func() {
		// Discard stdout (remote backup-helper doesn't output to stdout in download mode)
		io.Copy(io.Discard, stdout)
	}()

	go func() {
		// Discard stderr after port parsing (we don't need progress info in SSH mode)
		// This prevents the stderr buffer from filling up and blocking the remote process
		io.Copy(io.Discard, stderr)
	}()

	cleanupFunc := func() error {
		// Send SIGTERM to remote backup-helper process
		killCmd := exec.Command("ssh", sshHost, "pkill -TERM backup-helper")
		killCmd.Run()
		return cmd.Process.Kill()
	}

	return actualPort, outputPath, cmd, cleanupFunc, nil
}

// parseReceiverInfo parses both port and output path from backup-helper receiver output
// Looks for: "[backup-helper] Listening on <IP>:<port>"
// Note: "Saved to: <path>" comes after transfer completes, so we can't parse it here
// We return the port and use remoteOutput if provided
func parseReceiverInfo(reader io.Reader, expectedPort int, remoteOutput string) (int, string, error) {
	scanner := bufio.NewScanner(reader)
	portPattern := regexp.MustCompile(`Listening on [\d.]+:(\d+)`)

	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var actualPort int

	for {
		select {
		case <-timeout:
			if actualPort == 0 {
				return 0, "", fmt.Errorf("timeout waiting for receiver to start")
			}
			// We got the port, return it with remoteOutput (if provided)
			return actualPort, remoteOutput, nil
		case <-ticker.C:
			if scanner.Scan() {
				line := scanner.Text()
				// Parse port
				if matches := portPattern.FindStringSubmatch(line); matches != nil {
					if port, err := strconv.Atoi(matches[1]); err == nil {
						// If expectedPort was specified, validate it matches
						if expectedPort > 0 && port != expectedPort {
							return 0, "", fmt.Errorf("port mismatch: expected %d, got %d", expectedPort, port)
						}
						actualPort = port
						// Got the port, return immediately
						return actualPort, remoteOutput, nil
					}
				}
			} else {
				if err := scanner.Err(); err != nil {
					if actualPort == 0 {
						return 0, "", err
					}
					// We got the port, return it with remoteOutput
					return actualPort, remoteOutput, nil
				}
				// EOF, wait a bit more
				time.Sleep(200 * time.Millisecond)
			}
		}
	}
}

// parseReceiverPort parses the port from backup-helper receiver output
// Looks for: "[backup-helper] Listening on <IP>:<port>"
// If expectedPort > 0, validates it matches
// DEPRECATED: Use parseReceiverInfo instead
func parseReceiverPort(reader io.Reader, expectedPort int) (int, error) {
	port, _, err := parseReceiverInfo(reader, expectedPort, "")
	return port, err
}
