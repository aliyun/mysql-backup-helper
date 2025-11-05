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
- **zstd**: For zstd compression (when using `--compress-type=zstd`)
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
  "compress": true,
  "compressType": "zstd",
  "mode": "oss",
  "streamPort": 9999,
  "qwenAPIKey": "your-qwen-api-key",
  "enableAuth": false,
  "authKey": "your-secret-key",
  "existedBackup": "",
  "traffic": 209715200,
  "downloadOutput": "",
  "logDir": "/var/log/mysql-backup-helper"
}
```

- **objectName**: Only specify the prefix. The final OSS object will be `objectName_YYYYMMDDHHMM<suffix>`, e.g. `backup/your-backup_202507181648.xb.zst`
- **existedBackup**: Path to existing backup file for upload or streaming (use '-' for stdin)
- **logDir**: Log file storage directory, defaults to `/var/log/mysql-backup-helper`, supports both relative and absolute paths
- All config fields can be overridden by command-line arguments. Command-line arguments take precedence over config.

---

## 📖 Command-line Usage

### Global Parameters

| Parameter     | Description                                    |
|---------------|------------------------------------------------|
| --config      | Path to config file (optional)                 |
| --lang        | Language: zh (Chinese) or en (English), auto-detect if unset |
| --verbose, -v | Verbose output mode                            |
| --quiet, -q   | Quiet mode (minimal output)                    |

### Subcommands

#### 1. `backup` - Execute MySQL backup and transfer

**Purpose**: Connect to MySQL, execute xtrabackup backup, and upload to OSS or stream via TCP.

**Parameters**:

| Parameter         | Description                                             |
|-------------------|---------------------------------------------------------|
| --host            | MySQL server host address                               |
| --port            | MySQL server port (default: 3306)                       |
| --user            | MySQL username                                          |
| --password        | MySQL password (prompt if not specified)                |
| --mode            | Backup mode: oss or stream (default: oss)               |
| --stream-port     | TCP port number (stream mode only, 0=auto-find)         |
| --endpoint        | OSS endpoint URL (oss mode only)                        |
| --bucket-name     | OSS bucket name (oss mode only)                         |
| --object-name     | OSS object name prefix (oss mode only)                  |
| --compress-type   | Compression type: zstd, qp, or none                     |
| --traffic         | Traffic bandwidth limit (e.g., '100MB/s', -1=unlimited) |
| --enable-auth     | Enable stream authentication (stream mode only)         |
| --auth-key        | Authentication key (stream mode only)                   |

**Note**: OSS credentials (accessKeyId, accessKeySecret, securityToken) must be provided via config file, not command-line arguments (for security reasons).

**Examples**:
```bash
# Backup and upload to OSS (requires config file for credentials)
backup-helper backup --config config.json --host 127.0.0.1 --user root --mode oss

# Backup to specific bucket (override config)
backup-helper backup --config config.json --host 127.0.0.1 --user root \
  --mode oss --bucket-name my-backup-bucket --object-name backup/mysql

# Backup and stream via TCP
backup-helper backup --host 127.0.0.1 --user root --mode stream --stream-port 9000

# Use zstd compression with bandwidth limit
backup-helper backup --config config.json --host 127.0.0.1 --user root --mode oss \
  --compress-type zstd --traffic 100MB/s
```

#### 2. `send` - Send existing backup file

**Purpose**: Upload existing backup file to OSS or stream via TCP.

**Parameters**:

| Parameter         | Description                                          |
|-------------------|------------------------------------------------------|
| --file            | Backup file path ('-' for stdin)                     |
| --stdin           | Read backup data from stdin                          |
| --mode            | Transfer mode: oss or stream (default: oss)          |
| --stream-port     | TCP port number (stream mode only)                   |
| --endpoint        | OSS endpoint URL (oss mode only)                     |
| --bucket-name     | OSS bucket name (oss mode only)                      |
| --object-name     | OSS object name prefix (oss mode only)               |
| --skip-validation | Skip backup file validation                          |
| --validate-only   | Validate only, don't transfer                        |
| --traffic         | Traffic bandwidth limit                              |
| --enable-auth     | Enable stream authentication (stream mode only)      |
| --auth-key        | Authentication key (stream mode only)                |

**Note**: OSS credentials (accessKeyId, accessKeySecret, securityToken) must be provided via config file, not command-line arguments (for security reasons).

**Examples**:
```bash
# Upload backup file to OSS (requires config file for credentials)
backup-helper send --config config.json --file backup.xb --mode oss

# Upload to specific bucket (override config)
backup-helper send --config config.json --file backup.xb \
  --mode oss --bucket-name my-backup-bucket --object-name backup/mysql

# Stream backup file via TCP
backup-helper send --file backup.xb --mode stream --stream-port 9000

# Read from stdin and upload
cat backup.xb | backup-helper send --config config.json --stdin --mode oss

# Validate backup file only
backup-helper send --file backup.xb --validate-only
```

#### 3. `receive` - Receive backup data

**Purpose**: Receive backup data from TCP stream and save to file.

**Parameters**:

| Parameter     | Description                                                  |
|---------------|--------------------------------------------------------------|
| --from-stream | TCP port to listen on (0=auto-find)                          |
| --output      | Output file path ('-' for stdout, default: auto-generated)   |
| --stdout      | Output to stdout                                             |
| --traffic     | Traffic bandwidth limit                                      |
| --enable-auth | Enable stream authentication                                 |
| --auth-key    | Authentication key                                           |

**Examples**:
```bash
# Receive backup and save to file
backup-helper receive --from-stream 9000 --output backup.xb

# Receive backup and output to stdout (for piping)
backup-helper receive --from-stream 9000 --stdout | xbstream -x

# Auto-find available port
backup-helper receive --from-stream 0
```

#### 4. `version` - Show version information

**Examples**:
```bash
backup-helper version
```

#### 5. `ai` - AI diagnosis (experimental)

**Purpose**: Use AI to diagnose backup issues.

**Examples**:
```bash
backup-helper ai --diagnose
```

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
./backup-helper --config config.json --backup --mode=oss --compress-type=zstd
./backup-helper --config config.json --backup --mode=oss --compress-type=qp
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

### 6. Parameter check only (no backup)

```sh
./backup-helper --config config.json
```

### 7. All command-line (no config.json)

```sh
./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306 --backup --mode=oss --compress-type=qp
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
./backup-helper --config config.json --backup --mode=oss --traffic 100MB/s
# Supports units: KB/s, MB/s, GB/s, TB/s, or use bytes per second directly
```

### 13. Disable rate limiting (unlimited upload speed)

```sh
./backup-helper --config config.json --backup --mode=oss --traffic -1
# Use -1 to completely disable rate limiting for maximum upload speed
```

### 14. Specify estimated size for accurate progress display

```sh
./backup-helper --config config.json --backup --mode=oss --estimated-size 1GB
# Supports units: KB, MB, GB, TB, or use bytes directly
# Example: --estimated-size 1073741824 or --estimated-size 1GB
```

### 15. Download mode: Receive backup data from TCP stream

```sh
# Download to default file (backup_YYYYMMDDHHMMSS.xb)
./backup-helper --download --stream-port 9999

# Download to specified file
./backup-helper --download --stream-port 9999 --output my_backup.xb

# Stream to stdout (can be used with pipes for compression or extraction)
./backup-helper --download --stream-port 9999 --output - | zstd -d > backup.xb

# Direct extraction using xbstream
./backup-helper --download --stream-port 9999 --output - | xbstream -x -C /path/to/extract/dir

# If backup is compressed, decompress first then extract
./backup-helper --download --stream-port 9999 --output - | xbstream -x -C /path/to/extract/dir --decompress --decompress-threads=4

# Download with rate limiting
./backup-helper --download --stream-port 9999 --traffic 100MB/s

# Download with progress display (requires estimated size)
./backup-helper --download --stream-port 9999 --estimated-size 1GB

---

## Logging & Object Naming

- All backup logs are saved in the `logs/` directory, only the latest 10 logs are kept.
- OSS object names are auto-appended with a timestamp, e.g. `backup/your-backup_202507181648.xb.zst`, for easy archiving and lookup.

## Progress Tracking

The tool displays real-time progress information during backup upload/download:

- **Real-time Progress**: Shows uploaded/downloaded size, total size, percentage, transfer speed, and duration
- **Final Statistics**: Shows total uploaded/downloaded size, duration, and average speed
- **Size Calculation**:
  - If `--estimated-size` is provided, uses that value directly (supports units: KB, MB, GB, TB)
  - For live backups, automatically calculates MySQL datadir size
  - For existing backup files, automatically reads file size
  - When reading from stdin, size is unknown, only displays upload amount and speed

## Rate Limiting

- **Default Rate Limit**: If `--traffic` is not specified, defaults to 200 MB/s
- **Manual Rate Limit**: Use `--traffic` to specify upload/download bandwidth limit
  - Supports units: `KB/s`, `MB/s`, `GB/s`, `TB/s` (e.g., `100MB/s`, `1GB/s`)
  - Can also use bytes per second directly (e.g., `104857600` for 100 MB/s)
  - Use `-1` to completely disable rate limiting (unlimited upload speed)
- **Config File**: Can set `traffic` field in config file (in bytes per second, supports both OSS upload and stream transfer)

Example output:
```
[backup-helper] Traffic limit set to: 100.0 MB/s

Progress: 1.1 GB / 1.5 GB (73.3%) - 98.5 MB/s - Duration: 11.4s
Progress: 1.3 GB / 1.5 GB (86.7%) - 99.2 MB/s - Duration: 13.1s
[backup-helper] Upload completed!
  Total uploaded: 1.5 GB
  Duration: 15s
  Average speed: 102.4 MB/s
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
- **Log accumulation**: The program auto-cleans the logs directory, keeping only the latest 10 logs.

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
