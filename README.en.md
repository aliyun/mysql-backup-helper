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
  "traffic": 83886080,
  "mysqlHost": "127.0.0.1",
  "mysqlPort": 3306,
  "mysqlUser": "root",
  "mysqlPassword": "your-mysql-password",
  "compress": true,
  "mode": "oss",
  "streamPort": 9999,
  "enableHandshake": false,
  "streamKey": "your-secret-key",
  "existedBackup": ""
}
```

- **objectName**: Only specify the prefix. The final OSS object will be `objectName_YYYYMMDDHHMM<suffix>`, e.g. `backup/your-backup_202507181648.xb.zst`
- **existedBackup**: Path to existing backup file for upload or streaming (use '-' for stdin)
- All config fields can be overridden by command-line arguments. Command-line arguments take precedence over config.

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
| --mode             | Backup mode: `oss` (upload to OSS) or `stream` (push to TCP)     |
| --stream-port      | Local port for streaming mode (e.g. 9999)                        |
| --compress         | Enable compression                                               |
| --compress-type    | Compression type: `qp` (qpress), `zstd`                          |
| --lang             | Language: `zh` (Chinese) or `en` (English), auto-detect if unset |
| --ai-diagnose=on/off| AI diagnosis on backup failure. 'on' runs automatically (requires Qwen API Key in config), 'off' skips, unset will prompt interactively. |
| --enable-handshake   | Enable handshake for TCP streaming (default: false, can be set in config) |
| --stream-key         | Handshake key for TCP streaming (default: empty, can be set in config)    |
| --existed-backup     | Path to existing xtrabackup backup file to upload or stream (use '-' for stdin) |

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

- **In stream mode, all compression options are ignored; the backup is always sent as a raw physical stream.**

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

---

## Logging & Object Naming

- All backup logs are saved in the `logs/` directory, only the latest 10 logs are kept.
- OSS object names are auto-appended with a timestamp, e.g. `backup/your-backup_202507181648.xb.zst`, for easy archiving and lookup.

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

### Test Account Preparation

- Please prepare two MySQL accounts:
  - One with sufficient privileges for backup (e.g., `root` or an account with `RELOAD`, `LOCK TABLES`, `PROCESS`, `REPLICATION CLIENT` privileges).
  - One with limited privileges (e.g., only `SELECT`), to trigger backup failures and test AI diagnosis.
- Configure these accounts in `config.json` for different test scenarios.
