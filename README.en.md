# MySQL Backup Helper

A high-efficiency MySQL physical backup and OSS upload tool. Supports Percona XtraBackup, Alibaba Cloud OSS, streaming mode, automatic compression, and multi-language output.

---

## Requirements

### Go Version
- **Go 1.21 or higher is required** (latest Go toolchain recommended)
- If your `go.mod` contains a `toolchain` directive, you must use a Go toolchain of that version or higher. To build with an older Go version, remove the `toolchain` line from `go.mod`.

### Required
- **Percona XtraBackup**: For MySQL physical backup
  - [Download](https://www.percona.com/downloads/Percona-XtraBackup-LATEST/)
  - Ensure `xtrabackup` is in your PATH

### Optional
- **zstd**: For zstd compression (when using `--compress=zstd`)
  - [Download](https://github.com/facebook/zstd)
  - Ensure `zstd` is in your PATH

---

## Example config.json

```json
{
  "endpoint": "http://oss-cn-hangzhou.aliyuncs.com",
  "accessKeyId": "your-access-key-id",
  "accessKeySecret": "your-access-key-secret",
  "securityToken": "",
  "bucketName": "your-bucket-name",
  "objectName": "backup/your-backup",   // Only prefix needed, timestamp and suffix are auto-appended
  "size": 104857600,
  "buffer": 10,
  "mysqlHost": "127.0.0.1",
  "mysqlPort": 3306,
  "mysqlUser": "root",
  "mysqlPassword": "your-mysql-password",
  "compressType": "zstd",
  "mode": "oss",
  "streamPort": 9999,
  "streamHost": "",
  "mysqlVersion": {
    "major": 5,
    "minor": 7,
    "patch": 0
  },
  "qwenAPIKey": "your-qwen-api-key",
  "enableHandshake": false,
  "streamKey": "your-secret-key",
  "existedBackup": "",
  "logDir": "/var/log/mysql-backup-helper",
  "estimatedSize": 0,
  "ioLimit": 0,
  "downloadOutput": "",
  "remoteOutput": ""
}
```

- **objectName**: Only specify the prefix. The final OSS object will be `objectName_YYYYMMDDHHMM<suffix>`, e.g. `backup/your-backup_202507181648.xb.zst`
- **compressType**: Compression type, options: `zstd`, `qp` (qpress), or empty string/`no` (no compression). Supported in all modes (oss, stream)
- **streamPort**: Streaming port, set to `0` to auto-find available port
- **streamHost**: Remote host IP for active push mode
- **existedBackup**: Path to existing backup file for upload or streaming (use '-' for stdin)
- **logDir**: Log file storage directory, defaults to `/var/log/mysql-backup-helper`, supports both relative and absolute paths
- **downloadOutput**: Default output path for download mode
- **remoteOutput**: Remote save path for SSH mode
- **ioLimit**: IO bandwidth limit (bytes per second), set to `0` to use default (200MB/s), set to `-1` for unlimited speed
- **parallel**: Number of parallel threads (default: 4), used for xtrabackup backup, compression, decompression, and xbstream extraction operations
- **useMemory**: Memory to use for prepare operation (default: 1G), supports units (e.g., '1G', '512M')
- **xtrabackupPath**: Path to xtrabackup binary or directory containing xtrabackup/xbstream. Priority: command-line flag > config file > environment variable `XTRABACKUP_PATH` > PATH lookup
- All config fields can be overridden by command-line arguments. Command-line arguments take precedence over config.

**Note**: The tool automatically handles the following xtrabackup options without user configuration:
- `--defaults-file`: Automatically retrieved from MySQL connection (my.cnf path) and passed as the first argument to xtrabackup
- `--close-files=1`: Automatically enabled to handle large number of tables
- File descriptor limit: Automatically set to 655360 (via ulimit)

**Compatibility Notes**:
- The tool supports a wide range of xtrabackup/xbstream versions, including older versions that don't support the `--version` flag (e.g., xbstream 2.4.12)
- The tool uses multiple fallback methods to verify binary executability (`--version` → `-h` → `--help` → run without arguments)
- `--prepare` mode does not require xbstream, only xtrabackup is needed

---

## Command-line Arguments

| Argument           | Description                                                      |
|--------------------|------------------------------------------------------------------|
| --config           | Path to config file (e.g. `config.json`)                         |
| --host             | MySQL host (overrides config)                                    |
| --port             | MySQL port (overrides config)                                    |
| --user             | MySQL username (overrides config)                                |
| --password         | MySQL password (overrides config, prompt if omitted)             |
| --backup           | Run backup (otherwise only checks parameters)                    |
| --download         | Download mode: receive backup data from TCP stream and save      |
| --prepare          | Prepare mode: execute xtrabackup --prepare to make backup ready for restore |
| --output           | Output file path for download mode (use '-' for stdout, default: backup_YYYYMMDDHHMMSS.xb) |
| --target-dir       | Directory: extraction directory for download mode, backup directory for prepare mode |
| --mode             | Backup mode: `oss` (upload to OSS) or `stream` (push to TCP)     |
| --stream-port      | Local port for streaming mode (e.g. 9999, 0 = auto-find available port), or remote port when --stream-host is specified |
| --stream-host      | Remote host IP (e.g., '192.168.1.100'). When specified, actively connects to remote server to push data, similar to `nc host port` |
| --ssh              | Use SSH to automatically start receiver on remote host (requires --stream-host, relies on system SSH config) |
| --remote-output    | Remote output path for SSH mode (default: auto-generated) |
| --compress    | Compression: `qp` (qpress), `zstd`, or `no` (no compression). Defaults to qp when no value provided. Supported in all modes (oss, stream) |
| --lang             | Language: `zh` (Chinese) or `en` (English), auto-detect if unset |
| --ai-diagnose=on/off| AI diagnosis on operation failure. 'on' runs automatically (requires Qwen API Key in config), 'off' skips, unset will prompt interactively. Supports all modules (BACKUP, PREPARE, TCP, OSS, EXTRACT, etc.). |
| --enable-handshake   | Enable handshake for TCP streaming (default: false, can be set in config) |
| --stream-key         | Handshake key for TCP streaming (default: empty, can be set in config)    |
| --existed-backup     | Path to existing xtrabackup backup file to upload or stream (use '-' for stdin) |
| --estimated-size     | Estimated backup size with units (e.g., '100MB', '1GB') or bytes (for progress tracking) |
| --io-limit           | IO bandwidth limit with units (e.g., '100MB/s', '1GB/s') or bytes per second. Use -1 for unlimited speed |
| --parallel           | Number of parallel threads (default: 4), used for xtrabackup backup (--parallel), qpress compression (--compress-threads), zstd compression/decompression (-T), xbstream extraction (--parallel), and xtrabackup decompression (--parallel) |
| --use-memory         | Memory to use for prepare operation (e.g., '1G', '512M'). Default: 1G |
| --xtrabackup-path    | Path to xtrabackup binary or directory containing xtrabackup/xbstream (overrides config file and environment variable) |
| -y, --yes            | Non-interactive mode: automatically answer 'yes' to all prompts (including directory overwrite confirmation and AI diagnosis confirmation) |
| --version, -v        | Show version information                                               |

---

## Typical Usage

### 1. Build

```sh
go build -a -o backup-helper main.go
```

### 2. One-click backup and upload to OSS (auto language)

```sh
./backup-helper --config config.json --backup --mode=oss
```

### 3. Force English output

```sh
./backup-helper --config config.json --backup --mode=oss --lang=en
```

### 4. Specify compression type

```sh
./backup-helper --config config.json --backup --mode=oss --compress=zstd
./backup-helper --config config.json --backup --mode=oss --compress=qp
./backup-helper --config config.json --backup --mode=oss --compress=no
./backup-helper --config config.json --backup --mode=oss --compress
```

### 5. Streaming mode

```sh
./backup-helper --config config.json --backup --mode=stream --stream-port=9999
# In another terminal, pull the stream:
nc 127.0.0.1 9999 > streamed-backup.xb
```

### 5.1. Auto-find available port (recommended)

```sh
./backup-helper --config config.json --backup --mode=stream --stream-port=0
# The program will automatically find an available port and display the local IP and port
# Example output:
# [backup-helper] Listening on 192.168.1.100:54321
# [backup-helper] Waiting for remote connection...
# In another terminal, pull the stream (using the displayed port):
nc 192.168.1.100 54321 > streamed-backup.xb
```

- **In stream mode, all compression options are ignored; the backup is always sent as a raw physical stream.**
- **When auto-finding ports, the program automatically obtains and displays the local IP in the output, making remote connections easy.**
- **Use `--stream-host` to actively push to a remote server; the receiver side uses `--download --stream-port` to listen on the specified port.**

### 5.2. Actively push to remote server

```sh
# Sender side: actively connect to remote server and push data
./backup-helper --config config.json --backup --mode=stream --stream-host=192.168.1.100 --stream-port=9999

# Receiver side: listen and receive data on remote server
./backup-helper --download --stream-port=9999
```

This achieves similar functionality to `xtrabackup | nc 192.168.1.100 9999`.

### 5.3. SSH Mode: Automatically start receiver on remote host

If you have SSH access, you can use `--ssh` to automatically start the receiver on the remote host:

```sh
# SSH mode + auto-discover port (recommended)
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=replica-server \
    --ssh \
    --remote-output=/backup/mysql_backup.xb \
    --estimated-size=10GB

# SSH mode + specified port
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=replica-server \
    --ssh \
    --stream-port=9999 \
    --remote-output=/backup/mysql_backup.xb

# Traditional mode: requires manually running receiver on remote
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=replica-server \
    --stream-port=9999
```

**SSH Mode Notes:**
- When using `--ssh`, the program automatically executes `backup-helper --download` on the remote host via SSH
- Relies on existing SSH configuration (`~/.ssh/config`, keys, etc.), no additional setup needed
- If `--stream-port` is specified, starts service on that port; otherwise auto-discovers available port
- Automatically cleans up remote process after transfer completes
- Similar to `rsync -e ssh` usage - if SSH keys are configured, it just works

### 6. Parameter check only (no backup)

```sh
./backup-helper --config config.json
```

### 7. All command-line (no config.json)

```sh
./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306 --backup --mode=oss --compress=qp
```

### 8. Upload existing backup file to OSS

```sh
./backup-helper --config config.json --existed-backup backup.xb --mode=oss
```

### 9. Stream existing backup file via TCP

```sh
./backup-helper --config config.json --existed-backup backup.xb --mode=stream --stream-port=9999
# In another terminal, pull the stream:
nc 127.0.0.1 9999 > streamed-backup.xb
```

### 10. Use cat command to read from stdin and upload to OSS

```sh
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=oss
```

### 11. Use cat command to read from stdin and stream via TCP

```sh
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=stream --stream-port=9999
```

### 12. Manually specify upload rate limit (e.g., limit to 100 MB/s)

```sh
./backup-helper --config config.json --backup --mode=oss --io-limit 100MB/s
# Supports units: KB/s, MB/s, GB/s, TB/s, or use bytes per second directly
```

### 13. Disable rate limiting (unlimited upload speed)

```sh
./backup-helper --config config.json --backup --mode=oss --io-limit -1
# Use -1 to completely disable rate limiting for maximum upload speed
```

### 14. Specify estimated size for accurate progress display

```sh
./backup-helper --config config.json --backup --mode=oss --estimated-size 1GB
# Supports units: KB, MB, GB, TB, or use bytes directly
# Example: --estimated-size 1073741824 or --estimated-size 1GB
```

### 15. Prepare backup (Prepare Mode)

After backup is complete, execute prepare to make the backup ready for restore:

```sh
# Basic usage
./backup-helper --prepare --target-dir=/path/to/backup

# Specify parallel threads and memory size
./backup-helper --prepare --target-dir=/path/to/backup --parallel=8 --use-memory=2G

# Use config file
./backup-helper --config config.json --prepare --target-dir=/path/to/backup

# Optional: Provide MySQL connection info to auto-get --defaults-file
./backup-helper --prepare --target-dir=/path/to/backup --host=127.0.0.1 --user=root --port=3306
```

**Notes**:
- `--target-dir`: Required, specifies the backup directory to prepare
- `--parallel`: Number of parallel threads, default 4 (can be set in config file or command line)
- `--use-memory`: Memory to use for prepare operation, default 1G (supports units: G, M, K)
- `--host`, `--user`, `--port`: Optional, if provided can auto-get `--defaults-file`

### 16. Download mode: Receive backup data from TCP stream

```sh
# Download to default file (backup_YYYYMMDDHHMMSS.xb)
./backup-helper --download --stream-port 9999

# Download to specified file
./backup-helper --download --stream-port 9999 --output my_backup.xb

# Stream to stdout (can be used with pipes for compression or extraction)
./backup-helper --download --stream-port 9999 --output - | zstd -d > backup.xb

# Direct extraction using xbstream (uncompressed backup)
./backup-helper --download --stream-port 9999 --output - | xbstream -x -C /path/to/extract/dir

# Zstd compressed backup: stream decompress then extract (recommended)
./backup-helper --download --stream-port 9999 --compress=zstd --target-dir /path/to/extract/dir

# Zstd compressed backup: stream to stdout (can be piped to xbstream)
./backup-helper --download --stream-port 9999 --compress=zstd --output - | xbstream -x -C /path/to/extract/dir

# Qpress compressed backup: auto decompress and extract (note: requires saving to file first, no stream decompression)
./backup-helper --download --stream-port 9999 --compress=qp --target-dir /path/to/extract/dir

# Save zstd compressed backup (auto decompress)
./backup-helper --download --stream-port 9999 --compress=zstd --output my_backup.xb

# Download with rate limiting
./backup-helper --download --stream-port 9999 --io-limit 100MB/s

# Download with progress display (requires estimated size)
./backup-helper --download --stream-port 9999 --estimated-size 1GB

# Non-interactive mode: automatically confirm all prompts
./backup-helper --download --stream-port 9999 --target-dir /backup/mysql --compress=zstd -y
```

**Note**:
- If the directory specified by `--target-dir` already exists and is not empty, the program will prompt you to confirm overwriting existing files
- Enter `y` or `yes` to continue extraction (may overwrite existing files)
- Enter `n` or any other value to cancel extraction and exit
- Use `-y` or `--yes` flag to automatically confirm all prompts (non-interactive mode), suitable for scripts and automation scenarios

**Download mode compression type notes:**

- **Zstd compression (`--compress=zstd`)**:
  - Supports stream decompression, can directly decompress and extract to directory
  - When using `--target-dir`, automatically executes `zstd -d | xbstream -x`
  - When using `--output -`, outputs decompressed stream that can be piped to `xbstream`

- **Qpress compression (`--compress=qp` or `--compress`)**:
  - **Does not support stream decompression** (xbstream in MySQL 5.7 does not support `--decompress` in stream mode)
  - When using `--target-dir`, saves compressed file first, then uses `xbstream -x` to extract, finally uses `xtrabackup --decompress` to decompress
  - When using `--output -`, warns and outputs raw compressed stream

- **Uncompressed backup**:
  - When `--compress` is not specified, saves or extracts directly
  - When using `--target-dir`, directly uses `xbstream -x` to extract

---

## Logging & Object Naming

### Unified Logging System

The tool uses a unified logging system that records all critical operations into a single log file:

- **Log File Naming**: `backup-helper-{timestamp}.log` (e.g., `backup-helper-20251106105903.log`)
- **Log Storage Location**: Defaults to `/var/log/mysql-backup-helper`, can be specified via `--config` or `logDir` in config file (supports both relative and absolute paths)
- **Log Content**: Unified recording of all operation steps
  - **[BACKUP]**: xtrabackup backup operations
  - **[PREPARE]**: xtrabackup prepare operations
  - **[TCP]**: TCP stream transfers (send/receive)
  - **[OSS]**: OSS upload operations
  - **[XBSTREAM]**: xbstream extraction operations
  - **[DECOMPRESS]**: Decompression operations (zstd/qpress)
  - **[EXTRACT]**: Extraction operations
  - **[SYSTEM]**: System-level logs

- **Log Format**: Each log entry includes timestamp and module prefix, format: `[YYYY-MM-DD HH:MM:SS] [MODULE] message content`
- **Log Cleanup**: Automatically cleans old logs, keeping only the latest 10 log files
- **Error Handling**:
  - On operation completion or failure, displays log file location in console
  - On failure, automatically extracts error summary and displays in console
  - All modules support AI diagnosis (requires Qwen API Key configuration)
  - **Connection Interruption Detection**: Automatically detects TCP connection interruptions, process abnormal terminations, etc., logs to file and aborts the process to avoid processing incomplete data

Example log content:
```
[2025-11-06 10:59:03] [SYSTEM] === MySQL Backup Helper Log Started ===
[2025-11-06 10:59:03] [SYSTEM] Timestamp: 2025-11-06 10:59:03
[2025-11-06 10:59:03] [BACKUP] Starting backup operation
[2025-11-06 10:59:03] [BACKUP] Command: xtrabackup --backup --stream=xbstream ...
[2025-11-06 10:59:03] [TCP] Listening on 192.168.1.100:9999
[2025-11-06 10:59:03] [TCP] Client connected
[2025-11-06 10:59:03] [TCP] Transfer started
```

### OSS Object Naming

- OSS object names are auto-appended with a timestamp, e.g. `backup/your-backup_202507181648.xb.zst`, for easy archiving and lookup.

## Progress Tracking

The tool displays real-time progress information during backup upload/download:

- **Real-time Progress**: Shows uploaded/downloaded size, total size, percentage (when uncompressed), transfer speed, and duration
  - When compression is enabled, percentage is not shown (because compressed size differs from original size)
  - When uncompressed: `Progress: 100 MB / 500 MB (20.0%) - 50 MB/s - Duration: 2s`
  - When compressed: `Progress: 100 MB - 50 MB/s - Duration: 2s`
- **Final Statistics**: Shows total uploaded/downloaded size, duration, and average speed
- **Size Calculation**:
  - If `--estimated-size` is provided, uses that value directly (supports units: KB, MB, GB, TB)
  - For live backups, automatically calculates MySQL datadir size
  - For existing backup files, automatically reads file size
  - When reading from stdin, size is unknown, only displays upload amount and speed

## Rate Limiting

- **Default Rate Limit**: If `--io-limit` is not specified, defaults to 200 MB/s
- **Manual Rate Limit**: Use `--io-limit` to specify upload/download bandwidth limit
  - Supports units: `KB/s`, `MB/s`, `GB/s`, `TB/s` (e.g., `100MB/s`, `1GB/s`)
  - Can also use bytes per second directly (e.g., `104857600` for 100 MB/s)
  - Use `-1` to completely disable rate limiting (unlimited upload speed)
- **Config File**: Can set `ioLimit` field in config file (in bytes per second), can be overridden by `--io-limit` command-line argument

Example output (uncompressed):
```
[backup-helper] IO rate limit set to: 100.0 MB/s

Progress: 1.1 GB / 1.5 GB (73.3%) - 98.5 MB/s - Duration: 11.4s
Progress: 1.3 GB / 1.5 GB (86.7%) - 99.2 MB/s - Duration: 13.1s
[backup-helper] Upload completed!
  Total uploaded: 1.5 GB
  Duration: 15s
  Average speed: 102.4 MB/s
```

Example output (with compression):
```
[backup-helper] IO rate limit set to: 100.0 MB/s

Progress: 500 MB - 95.2 MB/s - Duration: 5.2s
Progress: 800 MB - 96.1 MB/s - Duration: 8.3s
[backup-helper] Upload completed!
  Total uploaded: 1.0 GB
  Duration: 10.5s
  Average speed: 97.5 MB/s
```

---

## Multi-language Support

- Auto-detects system language (Chinese/English), or force with `--lang=zh` or `--lang=en`.
- All terminal output supports bilingual switching.

---

## FAQ

- **zstd not installed**: Please install zstd and ensure it is in your PATH.
- **OSS upload failed**: Check OSS-related config parameters.
- **MySQL connection failed**: Check DB host, port, username, password.
- **Log accumulation**: The program auto-cleans the log directory, keeping only the latest 10 log files.
- **Log location**: On operation completion or failure, displays the full path to the log file in the console for troubleshooting.
- **Transfer interruption**: If the connection is interrupted during transfer, the system will automatically detect and log the error, then abort the process. Please check the log file for detailed error information.

---

For advanced usage or issues, please check the source code or submit an issue.

## Makefile Usage

- `make build`: Build the backup-helper executable.
- `make clean`: Clean build artifacts.
- `make test`: Run test.sh for automated integration tests, covering multi-language, compression, streaming, and AI diagnosis scenarios.

## Version Management

- `make version`: Show current version number
- `make get-version`: Get current version number (for scripts)
- `make set-version VER=1.0.1`: Set new version number
- `./version.sh show`: Show current version number
- `./version.sh set 1.0.1`: Set new version number
- `./version.sh get`: Get current version number (for scripts)

### Test Account Preparation

- Please prepare two MySQL accounts:
  - One with sufficient privileges for backup (e.g., `root` or an account with `RELOAD`, `LOCK TABLES`, `PROCESS`, `REPLICATION CLIENT` privileges).
  - One with limited privileges (e.g., only `SELECT`), to trigger backup failures and test AI diagnosis.
- Configure these accounts in `config.json` for different test scenarios.
