package check

import (
	"backup-helper/internal/config"
	"backup-helper/internal/mysql"
	"backup-helper/internal/utils"
	"database/sql"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
)

// CheckResult represents the result of a single check
type CheckResult struct {
	Status      string // "OK", "WARNING", "ERROR", "INFO", "RECOMMEND"
	Item        string
	Value       string
	Recommended string
	Message     string
}

// SystemResources contains system resource information
type SystemResources struct {
	CPUCores        int
	TotalMemory     int64 // bytes
	AvailableMemory int64 // bytes
	NetworkInfo     string
}

// CheckDependencies checks all required and optional tools
func CheckDependencies(cfg *config.Config, compressType string) []CheckResult {
	var results []CheckResult

	// Check xtrabackup (required)
	xtrabackupPath, xbstreamPath, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, true)
	if err != nil {
		results = append(results, CheckResult{
			Status:  "ERROR",
			Item:    "xtrabackup",
			Value:   "not found",
			Message: fmt.Sprintf("xtrabackup not found. %s", err.Error()),
		})
		// If xtrabackup not found, xbstream check will also fail, but we continue
	} else {
		// Get xtrabackup version
		cmd := exec.Command(xtrabackupPath, "--version")
		out, err := cmd.CombinedOutput()
		versionStr := "unknown"
		if err == nil {
			// Extract version from output
			output := string(out)
			lines := strings.Split(output, "\n")
			for _, line := range lines {
				if strings.Contains(line, "version") || strings.Contains(line, "Version") {
					versionStr = strings.TrimSpace(line)
					break
				}
			}
		}
		results = append(results, CheckResult{
			Status:  "OK",
			Item:    "xtrabackup",
			Value:   fmt.Sprintf("found at %s (%s)", xtrabackupPath, versionStr),
			Message: "",
		})

		// Check xbstream (required for backup)
		if xbstreamPath != "" {
			results = append(results, CheckResult{
				Status:  "OK",
				Item:    "xbstream",
				Value:   fmt.Sprintf("found at %s", xbstreamPath),
				Message: "",
			})
		}
	}

	// Check zstd (optional, if compressType includes zstd)
	if compressType == "zstd" {
		zstdPath, err := exec.LookPath("zstd")
		if err != nil {
			results = append(results, CheckResult{
				Status:  "WARNING",
				Item:    "zstd",
				Value:   "not found",
				Message: "zstd not found in PATH, install from https://github.com/facebook/zstd",
			})
		} else {
			results = append(results, CheckResult{
				Status:  "OK",
				Item:    "zstd",
				Value:   fmt.Sprintf("found at %s", zstdPath),
				Message: "",
			})
		}
	}

	// Check qpress (optional, if compressType includes qp)
	if compressType == "qp" {
		qpressPath, err := exec.LookPath("qpress")
		if err != nil {
			results = append(results, CheckResult{
				Status:  "WARNING",
				Item:    "qpress",
				Value:   "not found",
				Message: "qpress not found in PATH, install from https://github.com/mariadb-corporation/qpress",
			})
		} else {
			results = append(results, CheckResult{
				Status:  "OK",
				Item:    "qpress",
				Value:   fmt.Sprintf("found at %s", qpressPath),
				Message: "",
			})
		}
	}

	return results
}

// ValidateDefaultsFile validates if the defaults-file is correct
// It checks if the file exists, is readable, contains MySQL sections, and
// if a database connection is available, verifies that the datadir in the
// config file matches the actual datadir used by MySQL server.
func ValidateDefaultsFile(defaultsFile string, db *sql.DB) CheckResult {
	if defaultsFile == "" {
		return CheckResult{
			Status:  "WARNING",
			Item:    "defaults-file",
			Value:   "not found",
			Message: "Could not auto-detect MySQL config file. You may need to specify it manually using --defaults-file.",
		}
	}

	// Check if file exists and is readable
	if info, err := os.Stat(defaultsFile); err != nil || info.IsDir() {
		return CheckResult{
			Status:  "ERROR",
			Item:    "defaults-file",
			Value:   defaultsFile,
			Message: fmt.Sprintf("File does not exist or is not readable: %v", err),
		}
	}

	// Try to validate by checking if the file contains MySQL configuration sections
	content, err := os.ReadFile(defaultsFile)
	if err != nil {
		return CheckResult{
			Status:  "WARNING",
			Item:    "defaults-file",
			Value:   defaultsFile,
			Message: fmt.Sprintf("Could not read file for validation: %v", err),
		}
	}

	contentStr := string(content)
	hasMySQLSection := strings.Contains(contentStr, "[mysqld]") ||
		strings.Contains(contentStr, "[mysql]") ||
		strings.Contains(contentStr, "[client]")

	if !hasMySQLSection {
		return CheckResult{
			Status:  "WARNING",
			Item:    "defaults-file",
			Value:   defaultsFile,
			Message: "File does not appear to contain MySQL configuration sections. Please verify this is the correct config file.",
		}
	}

	// If we have a database connection, try to verify by checking if datadir matches
	if db != nil {
		actualDatadir := mysql.GetMySQLVariable(db, "datadir")
		if actualDatadir != "" {
			// Parse config file to find datadir setting
			configDatadir := parseDatadirFromConfig(contentStr)
			if configDatadir != "" {
				// Normalize paths for comparison
				actualDatadirNorm := filepath.Clean(actualDatadir)
				configDatadirNorm := filepath.Clean(configDatadir)

				if actualDatadirNorm != configDatadirNorm {
					return CheckResult{
						Status:  "ERROR",
						Item:    "defaults-file",
						Value:   defaultsFile,
						Message: fmt.Sprintf("CRITICAL: Config file datadir (%s) does not match MySQL server datadir (%s). This config file is likely NOT the one MySQL is using. Please specify the correct --defaults-file or remove it to let xtrabackup use default behavior.", configDatadir, actualDatadir),
					}
				}
			}
		}
	}

	return CheckResult{
		Status:  "OK",
		Item:    "defaults-file",
		Value:   defaultsFile,
		Message: "Config file found and appears valid",
	}
}

// parseDatadirFromConfig extracts datadir value from MySQL config file content
func parseDatadirFromConfig(content string) string {
	lines := strings.Split(content, "\n")
	inMysqldSection := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}

		// Check for section headers
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section := strings.Trim(line, "[]")
			inMysqldSection = (section == "mysqld" || section == "mysqld_safe")
			continue
		}

		// Look for datadir in [mysqld] or [mysqld_safe] section
		if inMysqldSection && strings.HasPrefix(strings.ToLower(line), "datadir") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				datadir := strings.TrimSpace(parts[1])
				// Remove quotes if present
				datadir = strings.Trim(datadir, "\"'")
				return datadir
			}
		}
	}

	return ""
}

// CheckMySQLCompatibility performs MySQL compatibility checks
func CheckMySQLCompatibility(db *sql.DB, cfg *config.Config) []CheckResult {
	var results []CheckResult

	if db == nil {
		return results
	}

	// Collect MySQL variables (silent mode to avoid duplicate output)
	options := mysql.CollectVariableFromMySQLServerSilent(db, true)

	// Check MySQL version
	if version, ok := options["version"]; ok && version != "" {
		// Parse version manually (same logic as checker.go)
		header := strings.Split(version, "-")[0]
		vers := strings.Split(header, ".")
		var v Version
		if len(vers) == 3 {
			major, _ := strconv.Atoi(vers[0])
			minor, _ := strconv.Atoi(vers[1])
			micro, _ := strconv.Atoi(vers[2])
			v = Version{major, minor, micro}
		}
		cfg.MysqlVersion = config.Version{
			Major: v.major,
			Minor: v.minor,
		}
		status := "OK"
		message := ""
		if v.major == 5 && v.minor == 7 {
			message = "MySQL 5.7"
		} else if v.major == 8 && v.minor == 0 && v.micro <= 36 {
			message = "MySQL 8.0"
		} else {
			status = "WARNING"
			message = "Version may be newer than supported versions"
		}
		results = append(results, CheckResult{
			Status:  status,
			Item:    "MySQL version",
			Value:   version,
			Message: message,
		})
	}

	// Check xtrabackup version compatibility
	mysqlVer := cfg.MysqlVersion
	xtrabackupPath, _, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, false)
	if err == nil {
		cmd := exec.Command(xtrabackupPath, "--version")
		out, err := cmd.CombinedOutput()
		if err == nil {
			versionStr := string(out)
			re := regexp.MustCompile(`([0-9]+)\.([0-9]+)\.([0-9]+)(?:-([0-9]+))?`)
			match := re.FindStringSubmatch(versionStr)
			if len(match) >= 4 {
				var xtrabackupVerParts [4]int
				xtrabackupVerParts[0], _ = strconv.Atoi(match[1])
				xtrabackupVerParts[1], _ = strconv.Atoi(match[2])
				xtrabackupVerParts[2], _ = strconv.Atoi(match[3])
				if len(match) >= 5 && match[4] != "" {
					xtrabackupVerParts[3], _ = strconv.Atoi(match[4])
				}

				status := "OK"
				message := ""
				if mysqlVer.Major == 5 && mysqlVer.Minor == 7 {
					// MySQL 5.7 only supports xtrabackup 2.4
					if xtrabackupVerParts[0] == 2 && xtrabackupVerParts[1] == 4 {
						message = "MySQL 5.7 with xtrabackup 2.4, compatible"
					} else {
						status = "ERROR"
						message = fmt.Sprintf("MySQL 5.7 only supports xtrabackup 2.4, but detected %d.%d. Please use xtrabackup 2.4 for MySQL 5.7", xtrabackupVerParts[0], xtrabackupVerParts[1])
					}
				} else if mysqlVer.Major == 8 && mysqlVer.Minor == 0 {
					// MySQL 8.0 only supports xtrabackup 8.0
					if xtrabackupVerParts[0] == 8 && xtrabackupVerParts[1] == 0 {
						message = "MySQL 8.0 with xtrabackup 8.0, compatible"
						if XtrabackupVersionGreaterOrEqual(xtrabackupVerParts, [4]int{8, 0, 34, 29}) {
							message += " (Note: xtrabackup 8.0.34-29+, default zstd compression may cause recovery issues)"
						}
					} else {
						status = "ERROR"
						message = fmt.Sprintf("MySQL 8.0 only supports xtrabackup 8.0, but detected %d.%d. Please use xtrabackup 8.0 for MySQL 8.0", xtrabackupVerParts[0], xtrabackupVerParts[1])
					}
				} else if mysqlVer.Major == 0 && mysqlVer.Minor == 0 {
					// MySQL version not detected or parsing failed
					status = "ERROR"
					message = fmt.Sprintf("MySQL version not detected or parsing failed. Detected xtrabackup %d.%d, cannot verify compatibility", xtrabackupVerParts[0], xtrabackupVerParts[1])
				} else {
					// Other MySQL versions (e.g., 5.6, 8.1, 8.4, etc.) are not supported
					status = "ERROR"
					message = fmt.Sprintf("MySQL %d.%d is not supported. Only MySQL 5.7 and 8.0 are supported", mysqlVer.Major, mysqlVer.Minor)
				}

				results = append(results, CheckResult{
					Status:  status,
					Item:    "xtrabackup compatibility",
					Value:   fmt.Sprintf("%d.%d.%d", xtrabackupVerParts[0], xtrabackupVerParts[1], xtrabackupVerParts[2]),
					Message: message,
				})
			}
		}
	}

	// Calculate data size
	datadir, err := mysql.GetDatadirFromMySQL(db)
	if err == nil {
		totalSize, err := mysql.CalculateBackupSize(datadir)
		if err == nil {
			results = append(results, CheckResult{
				Status:  "OK",
				Item:    "Estimated backup size",
				Value:   formatBytesForCheck(totalSize),
				Message: fmt.Sprintf("Based on datadir: %s", datadir),
			})
		}
	}

	// Check replication parameters
	// Required: gtid_mode=ON, enforce_gtid_consistency=ON, log_bin=ON or 1
	var repErrors []string
	var repValues []string

	// Check gtid_mode
	// MySQL gtid_mode valid values: OFF, OFF_PERMISSIVE, ON_PERMISSIVE, ON (case-sensitive in MySQL, but we accept case-insensitive)
	// We require ON (accepts ON, on, On, etc., but not 1 or other values)
	if gtidMode, ok := options["gtid_mode"]; ok && gtidMode != "" {
		repValues = append(repValues, fmt.Sprintf("gtid_mode=%s", gtidMode))
		gtidModeUpper := strings.ToUpper(gtidMode)
		if gtidModeUpper != "ON" {
			repErrors = append(repErrors, fmt.Sprintf("gtid_mode must be ON (case-insensitive), but got %s", gtidMode))
		}
	} else {
		repErrors = append(repErrors, "gtid_mode is not set")
	}

	// Check enforce_gtid_consistency
	// MySQL enforce_gtid_consistency valid values: OFF, ON, WARN (case-sensitive in MySQL, but we accept case-insensitive)
	// We require ON (accepts ON, on, On, etc., but not 1 or other values)
	if enforceGTID, ok := options["enforce_gtid_consistency"]; ok && enforceGTID != "" {
		repValues = append(repValues, fmt.Sprintf("enforce_gtid_consistency=%s", enforceGTID))
		enforceGTIDUpper := strings.ToUpper(enforceGTID)
		if enforceGTIDUpper != "ON" {
			repErrors = append(repErrors, fmt.Sprintf("enforce_gtid_consistency must be ON (case-insensitive), but got %s", enforceGTID))
		}
	} else {
		repErrors = append(repErrors, "enforce_gtid_consistency is not set")
	}

	// Check log_bin
	// MySQL log_bin valid values: ON, OFF, or a filename/path (case-sensitive in MySQL, but we accept case-insensitive for ON/OFF)
	// We require log_bin to be enabled (ON or a filename/path, but not OFF, 0, or empty)
	if logBin, ok := options["log_bin"]; ok && logBin != "" {
		repValues = append(repValues, fmt.Sprintf("log_bin=%s", logBin))
		logBinUpper := strings.ToUpper(logBin)
		// log_bin can be ON or a filename/path (any non-empty value that's not OFF means it's enabled)
		if logBinUpper == "OFF" || logBin == "0" {
			repErrors = append(repErrors, fmt.Sprintf("log_bin must be enabled (ON or a filename), but got %s", logBin))
		}
		// If log_bin is ON (case-insensitive) or any other non-empty value (filename/path), it's considered enabled
	} else {
		repErrors = append(repErrors, "log_bin is not set")
	}

	// Check server_id (optional, but log it)
	if serverID, ok := options["server_id"]; ok && serverID != "" {
		repValues = append(repValues, fmt.Sprintf("server_id=%s", serverID))
	}

	// Report replication parameters check result
	status := "OK"
	message := ""
	if len(repErrors) > 0 {
		status = "ERROR"
		message = strings.Join(repErrors, "; ")
	}

	value := strings.Join(repValues, ", ")
	if value == "" {
		value = "not set"
	}

	results = append(results, CheckResult{
		Status:  status,
		Item:    "Replication parameters",
		Value:   value,
		Message: message,
	})

	// Validate defaults-file (only if explicitly set in config)
	// We do NOT auto-detect to avoid using wrong config file (e.g., from another MySQL instance)
	// User should explicitly specify --defaults-file if they want to use it
	if cfg.DefaultsFile != "" {
		result := ValidateDefaultsFile(cfg.DefaultsFile, db)
		results = append(results, result)
	}

	return results
}

// CheckForBackupMode performs checks specific to backup mode
func CheckForBackupMode(cfg *config.Config, compressType string, db *sql.DB, streamHost string, streamPort int) []CheckResult {
	var results []CheckResult

	// Check dependencies (xtrabackup, xbstream, compression tools)
	depResults := CheckDependencies(cfg, compressType)
	results = append(results, depResults...)

	// Check MySQL connection and compatibility (required for backup)
	if db == nil {
		results = append(results, CheckResult{
			Status:  "ERROR",
			Item:    "MySQL connection",
			Value:   "not available",
			Message: "MySQL connection is required for backup mode. Please provide --host, --port, --user, and --password.",
		})
	} else {
		// Check MySQL compatibility
		mysqlResults := CheckMySQLCompatibility(db, cfg)
		results = append(results, mysqlResults...)
	}

	// Check TCP connectivity if stream-port or stream-host+stream-port is specified
	if streamPort > 0 {
		if streamHost != "" {
			// Check remote connectivity
			connectivityResults := CheckTCPConnectivity(streamHost, streamPort)
			results = append(results, connectivityResults...)
		} else {
			// Check local port listenability
			// Use default timeout for check mode (60s)
			listenResults := CheckTCPPortListenability(streamPort, 60)
			results = append(results, listenResults...)
		}
	}

	return results
}

// CheckForDownloadMode performs checks specific to download mode
func CheckForDownloadMode(cfg *config.Config, compressType string, targetDir string, streamHost string, streamPort int) []CheckResult {
	var results []CheckResult

	// Check compression/extraction dependencies if needed
	if compressType != "" {
		if targetDir != "" {
			// Extraction mode: check extraction dependencies
			xtrabackupPath, xbstreamPath, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, true)
			if err != nil {
				results = append(results, CheckResult{
					Status:  "ERROR",
					Item:    "xtrabackup/xbstream",
					Value:   "not found",
					Message: fmt.Sprintf("Extraction requires xtrabackup/xbstream: %v", err),
				})
			} else {
				results = append(results, CheckResult{
					Status:  "OK",
					Item:    "xtrabackup/xbstream",
					Value:   fmt.Sprintf("found at %s, %s", xtrabackupPath, xbstreamPath),
					Message: "",
				})
			}

			// Check compression tool
			if compressType == "zstd" {
				zstdPath, err := exec.LookPath("zstd")
				if err != nil {
					results = append(results, CheckResult{
						Status:  "ERROR",
						Item:    "zstd",
						Value:   "not found",
						Message: "zstd is required for decompression. Install from https://github.com/facebook/zstd",
					})
				} else {
					results = append(results, CheckResult{
						Status:  "OK",
						Item:    "zstd",
						Value:   fmt.Sprintf("found at %s", zstdPath),
						Message: "",
					})
				}
			} else if compressType == "qp" {
				qpressPath, err := exec.LookPath("qpress")
				if err != nil {
					results = append(results, CheckResult{
						Status:  "ERROR",
						Item:    "qpress",
						Value:   "not found",
						Message: "qpress is required for decompression. Install from https://github.com/mariadb-corporation/qpress",
					})
				} else {
					results = append(results, CheckResult{
						Status:  "OK",
						Item:    "qpress",
						Value:   fmt.Sprintf("found at %s", qpressPath),
						Message: "",
					})
				}
			}
		}
	} else if targetDir != "" {
		// No compression but extraction requested: check xbstream
		_, xbstreamPath, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, true)
		if err != nil {
			results = append(results, CheckResult{
				Status:  "ERROR",
				Item:    "xbstream",
				Value:   "not found",
				Message: fmt.Sprintf("Extraction requires xbstream: %v", err),
			})
		} else {
			results = append(results, CheckResult{
				Status:  "OK",
				Item:    "xbstream",
				Value:   fmt.Sprintf("found at %s", xbstreamPath),
				Message: "",
			})
		}
	}

	// Check target directory if specified
	if targetDir != "" {
		if info, err := os.Stat(targetDir); err == nil {
			if !info.IsDir() {
				results = append(results, CheckResult{
					Status:  "ERROR",
					Item:    "target-dir",
					Value:   targetDir,
					Message: "Target directory path exists but is not a directory",
				})
			} else {
				// Check if directory is writable
				testFile := filepath.Join(targetDir, ".backup-helper-test")
				if f, err := os.Create(testFile); err == nil {
					f.Close()
					os.Remove(testFile)
					results = append(results, CheckResult{
						Status:  "OK",
						Item:    "target-dir",
						Value:   targetDir,
						Message: "Directory exists and is writable",
					})
				} else {
					results = append(results, CheckResult{
						Status:  "WARNING",
						Item:    "target-dir",
						Value:   targetDir,
						Message: fmt.Sprintf("Directory exists but may not be writable: %v", err),
					})
				}
			}
		} else if os.IsNotExist(err) {
			// Directory doesn't exist, check if parent is writable
			parentDir := filepath.Dir(targetDir)
			if info, err := os.Stat(parentDir); err == nil && info.IsDir() {
				testFile := filepath.Join(parentDir, ".backup-helper-test")
				if f, err := os.Create(testFile); err == nil {
					f.Close()
					os.Remove(testFile)
					results = append(results, CheckResult{
						Status:  "OK",
						Item:    "target-dir",
						Value:   targetDir,
						Message: "Directory does not exist but parent is writable (will be created)",
					})
				} else {
					results = append(results, CheckResult{
						Status:  "ERROR",
						Item:    "target-dir",
						Value:   targetDir,
						Message: fmt.Sprintf("Directory does not exist and parent is not writable: %v", err),
					})
				}
			} else {
				results = append(results, CheckResult{
					Status:  "ERROR",
					Item:    "target-dir",
					Value:   targetDir,
					Message: fmt.Sprintf("Directory does not exist and parent directory is invalid: %v", err),
				})
			}
		}
	}

	// Check TCP connectivity if stream-port or stream-host+stream-port is specified
	if streamPort > 0 {
		if streamHost != "" {
			// Check remote connectivity
			connectivityResults := CheckTCPConnectivity(streamHost, streamPort)
			results = append(results, connectivityResults...)
		} else {
			// Check local port listenability
			// Use default timeout for check mode (60s)
			listenResults := CheckTCPPortListenability(streamPort, 60)
			results = append(results, listenResults...)
		}
	}

	return results
}

// CheckTCPPortListenability checks if a local port can be listened on and waits for a connection with timeout
// This is used for --check mode to verify port availability
// timeoutSeconds: connection timeout in seconds (0 means use default 60s, max 3600s)
func CheckTCPPortListenability(port int, timeoutSeconds int) []CheckResult {
	var results []CheckResult

	if port <= 0 {
		return results
	}

	// Try to listen on the port
	addr := fmt.Sprintf(":%d", port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		results = append(results, CheckResult{
			Status:  "ERROR",
			Item:    "TCP port listenability",
			Value:   fmt.Sprintf("port %d", port),
			Message: fmt.Sprintf("Cannot listen on port %d: %v. Port may be in use or not accessible.", port, err),
		})
		return results
	}

	// Port can be listened on, now wait for a connection with timeout
	// Set a timeout for accepting connections
	if timeoutSeconds <= 0 {
		timeoutSeconds = 60 // Default 60 seconds
	}
	if timeoutSeconds > 3600 {
		timeoutSeconds = 3600 // Max 3600 seconds
	}
	timeout := time.Duration(timeoutSeconds) * time.Second
	ln.(*net.TCPListener).SetDeadline(time.Now().Add(timeout))

	i18n.Printf("Checking port %d: listening and waiting for connection (timeout: %v)...\n", port, timeout)
	conn, err := ln.Accept()
	ln.Close() // Close listener immediately after accepting or timeout

	if err != nil {
		// Check if it's a timeout error
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			// Timeout is expected in check mode - port is available but no connection received
			results = append(results, CheckResult{
				Status:  "OK",
				Item:    "TCP port listenability",
				Value:   fmt.Sprintf("port %d", port),
				Message: fmt.Sprintf("Port %d is available and can be listened on. No connection received within timeout (expected in check mode).", port),
			})
		} else {
			// Other error
			results = append(results, CheckResult{
				Status:  "WARNING",
				Item:    "TCP port listenability",
				Value:   fmt.Sprintf("port %d", port),
				Message: fmt.Sprintf("Port %d can be listened on, but error accepting connection: %v", port, err),
			})
		}
	} else {
		// Connection received - close it immediately
		conn.Close()
		results = append(results, CheckResult{
			Status:  "OK",
			Item:    "TCP port listenability",
			Value:   fmt.Sprintf("port %d", port),
			Message: fmt.Sprintf("Port %d is available and connection test successful.", port),
		})
	}

	return results
}

// CheckTCPConnectivity checks if a remote host:port is reachable
// This is used for --check mode to verify remote connectivity
func CheckTCPConnectivity(host string, port int) []CheckResult {
	var results []CheckResult

	if host == "" || port <= 0 {
		return results
	}

	addr := net.JoinHostPort(host, strconv.Itoa(port))
	timeout := 10 * time.Second

	i18n.Printf("Checking connectivity to %s (timeout: %v)...\n", addr, timeout)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		results = append(results, CheckResult{
			Status:  "ERROR",
			Item:    "TCP connectivity",
			Value:   addr,
			Message: fmt.Sprintf("Cannot connect to %s: %v. Check network connectivity, firewall rules, and that the remote service is running.", addr, err),
		})
		return results
	}

	// Connection successful - close it immediately
	conn.Close()
	results = append(results, CheckResult{
		Status:  "OK",
		Item:    "TCP connectivity",
		Value:   addr,
		Message: fmt.Sprintf("Successfully connected to %s.", addr),
	})

	return results
}

// CheckForPrepareMode performs checks specific to prepare mode
func CheckForPrepareMode(cfg *config.Config, targetDir string, db *sql.DB) []CheckResult {
	var results []CheckResult

	// Check xtrabackup (required, but xbstream not needed for prepare)
	xtrabackupPath, _, err := utils.ResolveXtrabackupPath(cfg.XtrabackupPath, false)
	if err != nil {
		results = append(results, CheckResult{
			Status:  "ERROR",
			Item:    "xtrabackup",
			Value:   "not found",
			Message: fmt.Sprintf("xtrabackup is required for prepare mode: %v", err),
		})
	} else {
		results = append(results, CheckResult{
			Status:  "OK",
			Item:    "xtrabackup",
			Value:   fmt.Sprintf("found at %s", xtrabackupPath),
			Message: "",
		})
	}

	// Check target directory (required)
	if targetDir == "" {
		results = append(results, CheckResult{
			Status:  "ERROR",
			Item:    "target-dir",
			Value:   "not specified",
			Message: "--target-dir is required for prepare mode",
		})
	} else {
		if info, err := os.Stat(targetDir); err != nil {
			results = append(results, CheckResult{
				Status:  "ERROR",
				Item:    "target-dir",
				Value:   targetDir,
				Message: fmt.Sprintf("Backup directory does not exist: %v", err),
			})
		} else if !info.IsDir() {
			results = append(results, CheckResult{
				Status:  "ERROR",
				Item:    "target-dir",
				Value:   targetDir,
				Message: "Target path exists but is not a directory",
			})
		} else {
			// Check if directory is readable
			entries, err := os.ReadDir(targetDir)
			if err != nil {
				results = append(results, CheckResult{
					Status:  "ERROR",
					Item:    "target-dir",
					Value:   targetDir,
					Message: fmt.Sprintf("Cannot read backup directory: %v", err),
				})
			} else if len(entries) == 0 {
				results = append(results, CheckResult{
					Status:  "WARNING",
					Item:    "target-dir",
					Value:   targetDir,
					Message: "Backup directory is empty",
				})
			} else {
				results = append(results, CheckResult{
					Status:  "OK",
					Item:    "target-dir",
					Value:   targetDir,
					Message: fmt.Sprintf("Backup directory exists and contains %d entries", len(entries)),
				})
			}
		}
	}

	// MySQL connection is optional for prepare, but if provided, validate defaults-file
	if db != nil && cfg.DefaultsFile != "" {
		result := ValidateDefaultsFile(cfg.DefaultsFile, db)
		results = append(results, result)
	}

	return results
}

// CheckSystemResources checks system resources
func CheckSystemResources() SystemResources {
	resources := SystemResources{}

	// CPU cores
	resources.CPUCores = runtime.NumCPU()

	// Memory - try multiple methods
	// Method 1: Try syscall.Sysinfo on Linux
	if runtime.GOOS == "linux" {
		// Try to read /proc/meminfo
		if meminfo, err := os.ReadFile("/proc/meminfo"); err == nil {
			lines := strings.Split(string(meminfo), "\n")
			for _, line := range lines {
				if strings.HasPrefix(line, "MemTotal:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
							resources.TotalMemory = kb * 1024 // Convert KB to bytes
						}
					}
				}
				if strings.HasPrefix(line, "MemAvailable:") {
					fields := strings.Fields(line)
					if len(fields) >= 2 {
						if kb, err := strconv.ParseInt(fields[1], 10, 64); err == nil {
							resources.AvailableMemory = kb * 1024 // Convert KB to bytes
						}
					}
				}
			}
		}
	} else if runtime.GOOS == "darwin" {
		// macOS: use sysctl
		cmd := exec.Command("sysctl", "-n", "hw.memsize")
		if out, err := cmd.Output(); err == nil {
			if size, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64); err == nil {
				resources.TotalMemory = size
				resources.AvailableMemory = size // macOS doesn't easily provide available memory
			}
		}
	}

	// Network info - basic interface listing
	interfaces, err := net.Interfaces()
	if err == nil {
		var ifNames []string
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
				ifNames = append(ifNames, iface.Name)
			}
		}
		if len(ifNames) > 0 {
			resources.NetworkInfo = strings.Join(ifNames, ", ")
		}
	}

	return resources
}

// RecommendParameters recommends parameters based on system resources
func RecommendParameters(resources SystemResources, mysqlSize int64, compressType string, currentCfg *config.Config) []CheckResult {
	var results []CheckResult

	// Recommend parallel
	currentParallel := currentCfg.Parallel
	if currentParallel == 0 {
		currentParallel = 4 // default
	}
	recommendedParallel := resources.CPUCores
	if recommendedParallel < 2 {
		recommendedParallel = 2
	}
	if recommendedParallel > 16 {
		recommendedParallel = 16
	}
	// For compression, can use more threads
	if compressType != "" {
		recommendedParallel = recommendedParallel * 2
		if recommendedParallel > 16 {
			recommendedParallel = 16
		}
	}

	parallelMsg := fmt.Sprintf("current: %d, recommended: %d (based on %d CPU cores)", currentParallel, recommendedParallel, resources.CPUCores)
	if currentParallel != recommendedParallel {
		results = append(results, CheckResult{
			Status:      "RECOMMEND",
			Item:        "parallel",
			Value:       fmt.Sprintf("%d", currentParallel),
			Recommended: fmt.Sprintf("%d", recommendedParallel),
			Message:     parallelMsg,
		})
	} else {
		results = append(results, CheckResult{
			Status:  "OK",
			Item:    "parallel",
			Value:   fmt.Sprintf("%d", currentParallel),
			Message: parallelMsg,
		})
	}

	// Recommend io-limit
	currentIOLimit := currentCfg.IOLimit
	if currentIOLimit == 0 {
		currentIOLimit = 200 * 1024 * 1024 // 200MB/s default
	}
	recommendedIOLimit := int64(200 * 1024 * 1024) // Default 200MB/s
	if resources.AvailableMemory > 0 {
		// If we have a lot of memory, can increase IO limit
		if resources.AvailableMemory > 16*1024*1024*1024 { // > 16GB
			recommendedIOLimit = 300 * 1024 * 1024 // 300MB/s
		}
	}

	ioLimitMsg := "current: "
	if currentIOLimit == -1 {
		ioLimitMsg += "unlimited"
	} else {
		ioLimitMsg += formatBytesForCheck(currentIOLimit) + "/s"
	}
	ioLimitMsg += fmt.Sprintf(", recommended: %s/s (default)", formatBytesForCheck(recommendedIOLimit))

	if currentIOLimit != recommendedIOLimit && currentIOLimit != -1 {
		results = append(results, CheckResult{
			Status:      "RECOMMEND",
			Item:        "io-limit",
			Value:       formatBytesForCheck(currentIOLimit) + "/s",
			Recommended: formatBytesForCheck(recommendedIOLimit) + "/s",
			Message:     ioLimitMsg,
		})
	} else {
		results = append(results, CheckResult{
			Status:  "OK",
			Item:    "io-limit",
			Value:   ioLimitMsg,
			Message: "",
		})
	}

	// Recommend use-memory
	currentUseMemory := currentCfg.UseMemory
	if currentUseMemory == "" {
		currentUseMemory = "1G"
	}
	recommendedUseMemory := "1G" // Default
	if resources.AvailableMemory > 0 {
		// Recommend 25% of available memory, but between 1G and 8G
		recommendedBytes := resources.AvailableMemory / 4
		if recommendedBytes < 1024*1024*1024 {
			recommendedUseMemory = "1G"
		} else if recommendedBytes > 8*1024*1024*1024 {
			recommendedUseMemory = "8G"
		} else {
			recommendedUseMemory = formatBytesForCheck(recommendedBytes)
		}
	}

	useMemoryMsg := fmt.Sprintf("current: %s, recommended: %s", currentUseMemory, recommendedUseMemory)
	if resources.AvailableMemory > 0 {
		useMemoryMsg += fmt.Sprintf(" (based on %.1f GB available memory)", float64(resources.AvailableMemory)/(1024*1024*1024))
	}

	if currentUseMemory != recommendedUseMemory {
		results = append(results, CheckResult{
			Status:      "RECOMMEND",
			Item:        "use-memory",
			Value:       currentUseMemory,
			Recommended: recommendedUseMemory,
			Message:     useMemoryMsg,
		})
	} else {
		results = append(results, CheckResult{
			Status:  "OK",
			Item:    "use-memory",
			Value:   currentUseMemory,
			Message: useMemoryMsg,
		})
	}

	return results
}

// PrintCheckResults prints check results in a formatted way
func PrintCheckResults(section string, results []CheckResult) {
	i18n.Printf("\n=== %s ===\n", section)
	for _, result := range results {
		var statusColor func(string, ...interface{}) string
		switch result.Status {
		case "OK":
			statusColor = color.GreenString
		case "WARNING":
			statusColor = color.YellowString
		case "ERROR":
			statusColor = color.RedString
		case "INFO":
			statusColor = color.CyanString
		case "RECOMMEND":
			statusColor = color.MagentaString
		default:
			statusColor = func(s string, args ...interface{}) string {
				return fmt.Sprintf(s, args...)
			}
		}

		statusStr := statusColor("[%s]", result.Status)
		output := fmt.Sprintf("%s %s: %s", statusStr, result.Item, result.Value)

		if result.Recommended != "" {
			output += fmt.Sprintf(" (recommended: %s)", result.Recommended)
		}

		if result.Message != "" {
			output += fmt.Sprintf(" - %s", result.Message)
		}

		i18n.Printf("%s\n", output)
	}
}

// formatBytesForCheck formats bytes to human-readable format (internal use in precheck)
func formatBytesForCheck(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
