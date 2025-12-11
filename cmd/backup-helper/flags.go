package main

import (
	"backup-helper/internal/config"
	"flag"
)

// ParseFlags parses all command line flags and returns a config.Flags struct
func ParseFlags() *config.Flags {
	flags := &config.Flags{}

	flag.BoolVar(&flags.DoBackup, "backup", false, "Run xtrabackup and upload to OSS")
	flag.BoolVar(&flags.AutoYes, "y", false, "Automatically answer 'yes' to all prompts (non-interactive mode)")
	flag.BoolVar(&flags.AutoYes, "yes", false, "Automatically answer 'yes' to all prompts (non-interactive mode)")
	flag.BoolVar(&flags.DoDownload, "download", false, "Download backup from TCP stream (listen on port)")
	flag.BoolVar(&flags.DoPrepare, "prepare", false, "Prepare backup for restore (xtrabackup --prepare)")
	flag.BoolVar(&flags.DoCheck, "check", false, "Perform pre-flight validation checks (dependencies, MySQL compatibility, system resources, parameter recommendations)")
	flag.StringVar(&flags.DownloadOutput, "output", "", "Output file path for download mode (use '-' for stdout, default: backup_YYYYMMDDHHMMSS.xb)")
	flag.StringVar(&flags.TargetDir, "target-dir", "", "Directory for extraction (download mode) or backup directory (prepare mode)")
	flag.StringVar(&flags.EstimatedSizeStr, "estimated-size", "", "Estimated backup size with unit (e.g., '100MB', '1GB', '500KB') or bytes (for progress tracking)")
	flag.StringVar(&flags.IOLimitStr, "io-limit", "", "IO bandwidth limit with unit (e.g., '100MB/s', '1GB/s', '500KB/s') or bytes per second. Use -1 for unlimited speed")
	flag.StringVar(&flags.UseMemory, "use-memory", "", "Memory to use for prepare operation (e.g., '1G', '512M'). Default: 1G")
	flag.StringVar(&flags.XtrabackupPath, "xtrabackup-path", "", "Path to xtrabackup binary or directory containing xtrabackup/xbstream (overrides config and environment variable)")
	flag.StringVar(&flags.DefaultsFile, "defaults-file", "", "Path to MySQL configuration file (my.cnf). If not specified, --defaults-file will not be passed to xtrabackup")
	flag.StringVar(&flags.ExistedBackup, "existed-backup", "", "Path to existing xtrabackup backup file to upload (use '-' for stdin)")
	flag.BoolVar(&flags.ShowVersion, "version", false, "Show version information")
	flag.BoolVar(&flags.ShowVersion, "v", false, "Show version information (shorthand)")
	flag.StringVar(&flags.ConfigPath, "config", "", "config file path (optional)")
	flag.StringVar(&flags.Host, "host", "", "Connect to host")
	flag.IntVar(&flags.Port, "port", 0, "Port number to use for connection")
	flag.StringVar(&flags.User, "user", "", "User for login")
	flag.StringVar(&flags.Password, "password", "", "Password to use when connecting to server. If password is not given it's asked from the tty.")
	flag.IntVar(&flags.StreamPort, "stream-port", 0, "Local TCP port for streaming (0 = auto-find available port), or remote port when --stream-host is specified")
	flag.StringVar(&flags.StreamHost, "stream-host", "", "Remote host IP for pushing data (e.g., '192.168.1.100'). When specified, actively connects to remote instead of listening locally")
	flag.StringVar(&flags.Mode, "mode", "stream", "Backup mode: oss (upload to OSS) or stream (push to TCP port)")
	flag.StringVar(&flags.CompressType, "compress", "__NOT_SET__", "Compression: qp(qpress)/zstd/no, or no value (default: qp). Priority is higher than config file")
	flag.StringVar(&flags.LangFlag, "lang", "", "Language: zh (Chinese) or en (English), auto-detect if unset")
	flag.StringVar(&flags.AIDiagnoseFlag, "ai-diagnose", "", "AI diagnosis on backup failure: on/off. If not set, prompt interactively.")
	flag.BoolVar(&flags.EnableHandshake, "enable-handshake", false, "Enable handshake for TCP streaming (default: false, can be set in config)")
	flag.StringVar(&flags.StreamKey, "stream-key", "", "Handshake key for TCP streaming (default: empty, can be set in config)")
	flag.IntVar(&flags.Timeout, "timeout", 0, "TCP connection timeout in seconds for listening (default: 60, max: 3600)")
	flag.BoolVar(&flags.UseSSH, "ssh", false, "Use SSH to start receiver on remote host (requires --stream-host)")
	flag.StringVar(&flags.RemoteOutput, "remote-output", "", "Remote output path when using SSH mode (default: auto-generated)")
	flag.IntVar(&flags.Parallel, "parallel", 0, "Number of parallel threads for xtrabackup (default: 4)")
	flag.StringVar(&flags.LogFileName, "log-file", "", "Custom log file name (relative to logDir or absolute path). If not specified, auto-generates backup-helper-{timestamp}.log")

	flag.Parse()
	return flags
}
