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
- **MySQL Server Connection**: The tool connects to MySQL server via TCP/IP protocol
  - No need to install `mysql` command-line client tool
  - No need for local `mysqld` or socket files
  - Only requires TCP/IP connectivity to MySQL server (host:port)

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
- `--defaults-file`: Can be manually specified via `--defaults-file` parameter for MySQL config file path (my.cnf). If not specified, no auto-detection is performed to avoid using wrong config files
- `--close-files=1`: Automatically enabled to handle large number of tables
- File descriptor limit: Automatically set to 655360 (via ulimit)

**Compatibility Notes**:
- The tool supports a wide range of xtrabackup/xbstream versions, including older versions that don't support the `--version` flag (e.g., xbstream 2.4.12)
- The tool uses multiple fallback methods to verify binary executability (`--version` → `-h` → `--help` → run without arguments)
- `--prepare` mode does not require xbstream, only xtrabackup is needed
- The tool does not depend on `mysql` command-line client, connects directly to MySQL server via Go MySQL driver
- When getting config file path, if MySQL variables cannot be queried (e.g., insufficient permissions), it gracefully falls back to checking common paths

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
| --check            | Pre-check mode: perform pre-flight validation. Can be used alone (check all modes) or combined with other modes (e.g., `--check --backup` checks backup mode only) |
| --download         | Download mode: receive backup data from TCP stream and save      |
| --prepare          | Prepare mode: execute xtrabackup --prepare to make backup ready for restore |
| --output           | Output file path for download mode (use '-' for stdout, default: backup_YYYYMMDDHHMMSS.xb) |
| --target-dir       | Directory: extraction directory for download mode, backup directory for prepare mode |
| --mode             | Backup mode: `oss` (upload to OSS) or `stream` (push to TCP, default)     |
| --log-file         | Custom log file name (relative to logDir or absolute path). If not specified, auto-generates `backup-helper-{timestamp}.log` |
| --stream-port      | Local port for streaming mode (e.g. 9999, 0 = auto-find available port), or remote port when --stream-host is specified |
| --stream-host      | Remote host IP (e.g., '192.168.1.100'). When specified, actively connects to remote server to push data, similar to `nc host port` |
| --ssh              | Use SSH to automatically start receiver on remote host (requires --stream-host, relies on system SSH config) |
| --remote-output    | Remote output path for SSH mode (default: auto-generated) |
| --compress    | Compression: `qp` (qpress), `zstd`, or `no` (no compression). Defaults to qp when no value provided. Supported in all modes (oss, stream) |
| --lang             | Language: `zh` (Chinese) or `en` (English), auto-detect if unset |
| --ai-diagnose=on/off| AI diagnosis on operation failure. 'on' prompts user whether to run diagnosis (use with -y to skip prompt and run directly), 'off' skips, unset defaults to 'off' (no diagnosis). Supports all modules (BACKUP, PREPARE, TCP, OSS, EXTRACT, etc.). |
| --enable-handshake   | Enable handshake for TCP streaming (default: false, can be set in config) |
| --stream-key         | Handshake key for TCP streaming (default: empty, can be set in config)    |
| --existed-backup     | Path to existing xtrabackup backup file to upload or stream (use '-' for stdin) |
| --estimated-size     | Estimated backup size with units (e.g., '100MB', '1GB') or bytes (for progress tracking) |
| --io-limit           | IO bandwidth limit with units (e.g., '100MB/s', '1GB/s') or bytes per second. Use -1 for unlimited speed |
| --parallel           | Number of parallel threads (default: 4), used for xtrabackup backup (--parallel), qpress compression (--compress-threads), zstd compression/decompression (-T), xbstream extraction (--parallel), and xtrabackup decompression (--parallel) |
| --use-memory         | Memory to use for prepare operation (e.g., '1G', '512M'). Default: 1G |
| --defaults-file     | Path to MySQL configuration file (my.cnf). If not specified, no auto-detection is performed and --defaults-file will not be passed to xtrabackup |
| --xtrabackup-path    | Path to xtrabackup binary or directory containing xtrabackup/xbstream (overrides config file and environment variable) |
| -y, --yes            | Non-interactive mode: automatically answer 'yes' to all prompts (including directory overwrite confirmation and AI diagnosis confirmation) |
| --version, -v        | Show version information                                               |

---

## Quick Start

### Build

```sh
# Using makefile (recommended)
make build

# Or directly using go build
go build -a -o backup-helper ./cmd/backup-helper
```

---

## Usage Modes

The tool supports multiple usage modes for different scenarios:

### 1. Backup Mode (BACKUP)

Backup mode performs MySQL physical backup and supports two output methods: OSS upload and TCP streaming.

#### 1.1 OSS Mode

Upload backup directly to Alibaba Cloud OSS.

**Basic Usage:**

```sh
# Using config file, one-click backup and upload to OSS
./backup-helper --config config.json --backup --mode=oss

# Pure command-line arguments (no config file needed)
./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306 \
    --backup --mode=oss \
    --endpoint=http://oss-cn-hangzhou.aliyuncs.com \
    --access-key-id=your-key-id --access-key-secret=your-secret \
    --bucket-name=your-bucket --object-name=backup/mysql
```

**Compression Options:**

```sh
# Use zstd compression (recommended, high compression ratio and fast)
./backup-helper --config config.json --backup --mode=oss --compress=zstd

# Use qpress compression (MySQL 5.7 default compression)
./backup-helper --config config.json --backup --mode=oss --compress=qp
./backup-helper --config config.json --backup --mode=oss --compress  # Default qp

# No compression (raw backup stream)
./backup-helper --config config.json --backup --mode=oss --compress=no
```

**Rate Limiting and Progress:**

```sh
# Limit upload speed to 100 MB/s
./backup-helper --config config.json --backup --mode=oss --io-limit 100MB/s

# Disable rate limiting (maximum upload speed)
./backup-helper --config config.json --backup --mode=oss --io-limit -1

# Specify estimated size for accurate progress display
./backup-helper --config config.json --backup --mode=oss --estimated-size 10GB
```

**Advanced Options:**

```sh
# Specify parallel threads (default: 4)
./backup-helper --config config.json --backup --mode=oss --parallel=8

# Specify language interface
./backup-helper --config config.json --backup --mode=oss --lang=en

# Enable AI diagnosis (auto-diagnose on failure)
./backup-helper --config config.json --backup --mode=oss --ai-diagnose=on

# Non-interactive mode (auto-confirm all prompts)
./backup-helper --config config.json --backup --mode=oss -y
```

#### 1.2 Stream Mode (Passive Listen)

Listen on local port, wait for remote client to connect and receive backup data.

**Basic Usage:**

```sh
# Specify port to listen
./backup-helper --config config.json --backup --mode=stream --stream-port=9999
# In another terminal, connect and receive data
nc 127.0.0.1 9999 > streamed-backup.xb
```

**Auto-find Available Port (Recommended):**

```sh
./backup-helper --config config.json --backup --mode=stream --stream-port=0
# The program will automatically find an available port and display the local IP and port
# Example output:
# [backup-helper] Listening on 192.168.1.100:54321
# [backup-helper] Waiting for remote connection...
# In another terminal, pull the stream (using the displayed port):
nc 192.168.1.100 54321 > streamed-backup.xb
```

**Important Notes:**
- In stream mode, all compression options are ignored; the backup is always sent as a raw physical stream
- When auto-finding ports, the program automatically obtains and displays the local IP in the output, making remote connections easy
- The receiver side can use `backup-helper --download` or `nc` to receive data

#### 1.3 Stream Mode (Active Push)

Actively connect to remote server and push backup data.

**Basic Usage:**

```sh
# Sender side: actively connect to remote server and push data
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=192.168.1.100 --stream-port=9999

# Receiver side: listen and receive data on remote server
./backup-helper --download --stream-port=9999
```

This achieves similar functionality to `xtrabackup | nc 192.168.1.100 9999`.

#### 1.4 SSH Mode

Automatically start receiver service on remote host via SSH, no manual operation needed.

**Basic Usage:**

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

# SSH mode + auto decompress and extract to directory
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=replica-server \
    --ssh \
    --target-dir=/backup/mysql \
    --compress=zstd
```

**SSH Mode Notes:**
- When using `--ssh`, the program automatically executes `backup-helper --download` on the remote host via SSH
- Relies on existing SSH configuration (`~/.ssh/config`, keys, etc.), no additional setup needed
- If `--stream-port` is specified, starts service on that port; otherwise auto-discovers available port
- Automatically cleans up remote process after transfer completes
- Similar to `rsync -e ssh` usage - if SSH keys are configured, it just works

---

### 2. Download Mode (DOWNLOAD)

Receive backup data from TCP stream and save or extract.

**Basic Usage:**

```sh
# Download to default file (backup_YYYYMMDDHHMMSS.xb)
./backup-helper --download --stream-port 9999

# Download to specified file
./backup-helper --download --stream-port 9999 --output my_backup.xb

# Stream to stdout (can be used with pipes for compression or extraction)
./backup-helper --download --stream-port 9999 --output - | zstd -d > backup.xb
./backup-helper --download --stream-port 9999 --output - | xbstream -x -C /path/to/extract/dir
```

**Extract to Directory:**

```sh
# Uncompressed backup: directly extract to directory
./backup-helper --download --stream-port 9999 --target-dir /path/to/extract/dir

# Zstd compressed backup: stream decompress then extract (recommended)
./backup-helper --download --stream-port 9999 --compress=zstd --target-dir /path/to/extract/dir

# Qpress compressed backup: auto decompress and extract (note: requires saving to file first, no stream decompression)
./backup-helper --download --stream-port 9999 --compress=qp --target-dir /path/to/extract/dir
```

**Advanced Options:**

```sh
# Download with rate limiting
./backup-helper --download --stream-port 9999 --io-limit 100MB/s

# Download with progress display (requires estimated size)
./backup-helper --download --stream-port 9999 --estimated-size 1GB

# Non-interactive mode: automatically confirm all prompts
./backup-helper --download --stream-port 9999 --target-dir /backup/mysql --compress=zstd -y

# Use config file
./backup-helper --config config.json --download --stream-port 9999
```

**Download Mode Compression Type Notes:**

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

**Note:**
- If the directory specified by `--target-dir` already exists and is not empty, the program will prompt you to confirm overwriting existing files
- Enter `y` or `yes` to continue extraction (may overwrite existing files)
- Enter `n` or any other value to cancel extraction and exit
- Use `-y` or `--yes` flag to automatically confirm all prompts (non-interactive mode), suitable for scripts and automation scenarios

---

### 3. Prepare Mode (PREPARE)

After backup is complete, execute prepare to make the backup ready for restore.

**Basic Usage:**

```sh
# Basic usage
./backup-helper --prepare --target-dir=/path/to/backup

# Specify parallel threads and memory size
./backup-helper --prepare --target-dir=/path/to/backup --parallel=8 --use-memory=2G

# Use config file
./backup-helper --config config.json --prepare --target-dir=/path/to/backup

# Optional: Provide MySQL connection info and --defaults-file
./backup-helper --prepare --target-dir=/path/to/backup \
    --host=127.0.0.1 --user=root --port=3306 \
    --defaults-file=/etc/my.cnf
```

**Notes:**
- `--target-dir`: Required, specifies the backup directory to prepare
- `--parallel`: Number of parallel threads, default 4 (can be set in config file or command line)
- `--use-memory`: Memory to use for prepare operation, default 1G (supports units: G, M, K)
- `--defaults-file`: Optional, manually specify MySQL config file path (if not specified, no auto-detection is performed)

---

### 4. Pre-check Mode (CHECK)

Perform pre-flight validation, can be used alone or combined with other modes.

**Basic Usage:**

```sh
# Use alone: check all modes (BACKUP, DOWNLOAD, PREPARE)
./backup-helper --check

# Check all modes (including MySQL compatibility checks)
./backup-helper --check --host=127.0.0.1 --user=root --password=yourpass --port=3306

# Check backup mode only (does not execute backup)
./backup-helper --check --backup --host=127.0.0.1 --user=root --password=yourpass

# Check download mode only (does not execute download)
./backup-helper --check --download --target-dir=/path/to/extract

# Check prepare mode only (does not execute prepare)
./backup-helper --check --prepare --target-dir=/path/to/backup

# Check with compression type specified
./backup-helper --check --compress=zstd --host=127.0.0.1 --user=root --password=yourpass
```

**Check Contents:**
- **Dependency Checks**: Verify if xtrabackup, xbstream, zstd, qpress tools are installed
- **MySQL Compatibility Checks** (backup mode): MySQL version, xtrabackup version compatibility, data size estimation, replication parameters, config file validation
- **System Resource Checks** (when using --check alone): CPU cores, memory size, network interfaces
- **Parameter Recommendations** (backup mode): Recommend parallel, io-limit, use-memory parameters based on system resources
- **Target Directory Checks** (download/prepare modes): Verify directory existence, writability, backup file presence, etc.

**Important Notes:**
- When using `--backup`, `--download`, or `--prepare`, the tool automatically performs pre-flight checks before execution
- If pre-flight checks find critical issues (ERROR), the tool will stop and prompt you to fix them
- When using `--check` combined with a mode (e.g., `--check --backup`), only checks are performed, no actual operations are executed

---

### 5. Existing Backup Handling (EXISTED_BACKUP)

Upload or stream existing backup files.

**Upload to OSS:**

```sh
# Upload local backup file to OSS
./backup-helper --config config.json --existed-backup backup.xb --mode=oss

# Read from stdin and upload to OSS
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=oss

# Upload with compression (note: if backup is already compressed, this option is ignored)
./backup-helper --config config.json --existed-backup backup.xb --mode=oss --compress=zstd
```

**Stream via TCP:**

```sh
# Stream local backup file
./backup-helper --config config.json --existed-backup backup.xb --mode=stream --stream-port=9999
# In another terminal, pull the stream:
nc 127.0.0.1 9999 > streamed-backup.xb

# Read from stdin and stream via TCP
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=stream --stream-port=9999

# Actively push to remote server
./backup-helper --config config.json --existed-backup backup.xb \
    --mode=stream --stream-host=192.168.1.100 --stream-port=9999
```

**Advanced Options:**

```sh
# Upload with rate limiting
./backup-helper --config config.json --existed-backup backup.xb --mode=oss --io-limit 100MB/s

# Specify estimated size for accurate progress display
./backup-helper --config config.json --existed-backup backup.xb --mode=oss --estimated-size 10GB
```

---

### 6. Parameter Validation Mode

Only validate configuration parameters, do not execute any operations.

```sh
# Validate parameters using config file
./backup-helper --config config.json

# Validate pure command-line parameters
./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306
```

---

## Logging & Object Naming

### Unified Logging System

The tool uses a unified logging system that records all critical operations into a single log file:

- **Log File Naming**: Defaults to auto-generated `backup-helper-{timestamp}.log` (e.g., `backup-helper-20251106105903.log`), can be customized via `--log-file` or `logFileName` in config file (supports both relative and absolute paths)
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
