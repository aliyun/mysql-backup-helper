# MySQL Backup Helper Delivery Guide

## Version Information

- **Version**: v1.0.0
- **Build Date**: 2025-11-06
- **Main Features**: Streaming backup transfer, automatic compression, remote reception

---

## 1. Product Overview

MySQL Backup Helper is an efficient MySQL physical backup tool designed for streaming backup transfer. Key features include:

### Core Features

1. **Streaming Backup Transfer**
   - Real-time data transfer over TCP
   - Supports active push mode (similar to `xtrabackup | nc host port`)
   - Supports passive receive mode (listen on port for incoming data)

2. **Automatic Compression**
   - Supports Zstandard (zstd) compression (recommended)
   - Supports qpress compression
   - Fully automated compression and decompression

3. **Progress Display**
   - Real-time display of transfer progress, speed, transferred size
   - Supports estimated size setting with percentage display

4. **Bandwidth Rate Limiting**
   - Configurable transfer rate limit (default 200MB/s)
   - Prevents network or disk I/O saturation

5. **Unified Logging**
   - All operations logged to unified log files
   - Facilitates troubleshooting and auditing

---

## 2. Installation and Configuration

### 2.1 Download Binary

```bash
# Download latest version
wget https://mysql-fanke.oss-cn-beijing.aliyuncs.com/backup-helper/mysql-backup-helper

# Or using curl
curl -O https://mysql-fanke.oss-cn-beijing.aliyuncs.com/backup-helper/mysql-backup-helper

# Add execute permission
chmod +x mysql-backup-helper
```

### 2.2 Configure Environment Variables

#### Linux/macOS

```bash
# Create installation directory (optional)
sudo mkdir -p /usr/local/bin

# Move to system directory
sudo mv mysql-backup-helper /usr/local/bin/

# Or move to custom directory, e.g.:
sudo mv mysql-backup-helper /opt/mysql-backup-helper/

# Add to PATH (choose one method):

# Method 1: Add to ~/.bashrc (current user)
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
source ~/.bashrc

# Method 2: Add to ~/.zshrc (if using zsh)
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.zshrc
source ~/.zshrc

# Method 3: Add to /etc/profile (all users)
echo 'export PATH=$PATH:/usr/local/bin' >> /etc/profile
source /etc/profile

# Verify installation
which mysql-backup-helper
mysql-backup-helper --version
```

#### Custom Installation Path

If installed in custom directory (e.g., `/opt/mysql-backup-helper/`):

```bash
# Add to PATH
echo 'export PATH=$PATH:/opt/mysql-backup-helper' >> ~/.bashrc
source ~/.bashrc

# Verify
mysql-backup-helper --version
```

### 2.3 Verify Installation

```bash
# Check version
mysql-backup-helper --version

# View help information
mysql-backup-helper --help
```

### 2.4 Dependencies

#### Required Dependencies

- **Percona XtraBackup**: For MySQL physical backup
  - Download: https://www.percona.com/downloads/Percona-XtraBackup-LATEST/
  - Ensure `xtrabackup` and `xbstream` commands are in PATH after installation

#### Optional Dependencies (for compression)

- **zstd**: For zstd compression (recommended)
  - Download: https://github.com/facebook/zstd
  - Ensure `zstd` command is in PATH after installation

---

## 3. Usage Scenarios

### Scenario 1: Streaming Backup + Direct Extraction to Target Directory (Recommended)

This is the most common scenario, suitable when backups need to be extracted immediately after completion.

#### Sender Side (Backup Server)

```bash
# Basic usage
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=192.168.1.100 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=your_password \
  --compress=zstd \
  --estimated-size=10GB

# Parameter explanation:
# --backup: Start backup process
# --mode=stream: Streaming transfer mode
# --stream-host: Receiver IP address
# --stream-port: Receiver port (default 9999)
# --host: MySQL server address
# --user: MySQL username
# --password: MySQL password
# --compress=zstd: Use zstd compression (recommended)
# --estimated-size: Estimated backup size (for progress display)
```

#### Receiver Side (Backup Storage Server)

```bash
# Direct extraction to target directory (auto decompress + extract)
mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=/backup/mysql \
  --compress=zstd \
  --estimated-size=10GB \
  --io-limit=200MB/s

# Parameter explanation:
# --download: Download mode
# --stream-port: Listening port
# --target-dir: Extraction target directory
# --compress=zstd: Compression type (must match sender)
# --estimated-size: Estimated size (for progress display)
# --io-limit: Bandwidth limit (optional, default 200MB/s)
```

**Complete Example**:

```bash
# Sender side (172.24.215.240)
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=172.24.215.241 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=Test123! \
  --compress=zstd \
  --estimated-size=50GB

# Receiver side (172.24.215.241)
mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=/backup/mysql-restore \
  --compress=zstd \
  --estimated-size=50GB
```

### Scenario 2: Streaming Backup + Save to File (Stream Output)

Suitable when backup files need to be saved first and processed manually later.

#### Sender Side (same as above)

```bash
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=192.168.1.100 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=your_password \
  --compress=zstd \
  --estimated-size=10GB
```

#### Receiver Side (save to file)

```bash
# Save as compressed file (auto decompress)
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=/backup/mysql_backup.xb \
  --compress=zstd \
  --estimated-size=10GB

# Stream output to stdout (for pipeline operations)
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=- \
  --compress=zstd \
  2>/tmp/download.log | xbstream -x -C /backup/mysql-restore

# Parameter explanation:
# --output: Output path
#   - Specify path: Save to file
#   - Set to "-": Output to stdout
# 2>/tmp/download.log: Redirect logs and progress to file
```

**Complete Example**:

```bash
# Sender side (172.24.215.240)
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=172.24.215.241 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=Test123! \
  --compress=zstd \
  --estimated-size=50GB

# Receiver side (172.24.215.241) - Save to file
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=/backup/mysql_backup_$(date +%Y%m%d).xb \
  --compress=zstd \
  --estimated-size=50GB

# Or stream output to stdout
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=- \
  --compress=zstd \
  2>/tmp/download.log | xbstream -x -C /backup/mysql-restore
```

### Scenario 3: Prepare Backup

After backup transfer completes, backup needs to be prepared before it can be used for restore.

```bash
# Basic usage
mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore

# Specify parallel threads and memory
mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore \
  --parallel=8 \
  --use-memory=4G

# Provide MySQL connection info to auto-get config file path
mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore \
  --host=127.0.0.1 \
  --user=root \
  --port=3306

# Parameter explanation:
# --prepare: Prepare mode
# --target-dir: Backup directory (required)
# --parallel: Parallel threads (default 4)
# --use-memory: Memory size to use (default 1G)
# --host, --user, --port: Optional, for auto-getting MySQL config file path
```

---

## 4. Complete Workflow Example

### 4.1 Typical Backup Workflow

```bash
# ========== Step 1: Start backup on sender side ==========
# Execute on backup server (172.24.215.240):

mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=172.24.215.241 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=Test123! \
  --compress=zstd \
  --estimated-size=50GB \
  --io-limit=300MB/s

# Output example:
# [backup-helper] IO rate limit set to: 300.0 MB/s
# [backup-helper] Handshake OK, start streaming backup to 172.24.215.241:9999...
# Progress: 1.2 GB / 50.0 GB (2.4%) - 285.3 MB/s - Duration: 4.2s
# Progress: 5.8 GB / 50.0 GB (11.6%) - 298.7 MB/s - Duration: 19.4s
# ...
# [backup-helper] Backup and upload completed!
#   Total uploaded: 50.0 GB
#   Duration: 2m48s
#   Average speed: 304.2 MB/s
# [backup-helper] Log file: /var/log/mysql-backup-helper/backup-helper-20251106120000.log

# ========== Step 2: Receive and extract on receiver side ==========
# Execute on backup storage server (172.24.215.241):

mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=/backup/mysql-restore \
  --compress=zstd \
  --estimated-size=50GB \
  --io-limit=300MB/s

# Output example:
# [backup-helper] IO rate limit set to: 300.0 MB/s
# [backup-helper] Listening on 172.24.215.241:9999
# [backup-helper] Waiting for remote connection...
# [backup-helper] Remote client connected, no handshake required
# Progress: 1.2 GB / 50.0 GB (2.4%) - 285.3 MB/s - Duration: 4.2s
# Progress: 5.8 GB / 50.0 GB (11.6%) - 298.7 MB/s - Duration: 19.4s
# ...
# [backup-helper] Extraction completed successfully
# [backup-helper] Download completed! Extracted to: /backup/mysql-restore
#   Total downloaded: 50.0 GB
#   Duration: 2m48s
#   Average speed: 304.2 MB/s
# [backup-helper] Log file: /var/log/mysql-backup-helper/backup-helper-20251106120001.log

# ========== Step 3: Prepare backup ==========
# Execute on backup storage server:

mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore \
  --parallel=8 \
  --use-memory=4G

# Output example:
# [backup-helper] Preparing backup in directory: /backup/mysql-restore
# [backup-helper] Parallel threads: 8
# [backup-helper] Use memory: 4G
# Equivalent shell command: xtrabackup --prepare --target-dir=/backup/mysql-restore --parallel=8 --use-memory=4G
# [backup-helper] Backup is ready for restore in: /backup/mysql-restore
# [backup-helper] Log file: /var/log/mysql-backup-helper/backup-helper-20251106120300.log
```

### 4.2 Stream Output Scenario

```bash
# ========== Sender side (same as above) ==========

# ========== Receiver side: Stream output and extract ==========
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=- \
  --compress=zstd \
  2>/tmp/download.log | xbstream -x -C /backup/mysql-restore

# Note:
# - When using --output=-, all non-data output (logs, progress) goes to stderr
# - Use 2>/tmp/download.log to save logs to file
# - Compressed data is auto-decompressed first, then piped to xbstream
```

---

## 5. Common Parameters

### 5.1 Sender Parameters (--backup)

| Parameter | Description | Example | Required |
|-----------|-------------|---------|----------|
| `--backup` | Start backup process | `--backup` | Yes |
| `--mode` | Backup mode | `--mode=stream` | Yes |
| `--stream-host` | Receiver IP address | `--stream-host=192.168.1.100` | Yes |
| `--stream-port` | Receiver port | `--stream-port=9999` | No (default 9999) |
| `--host` | MySQL server address | `--host=127.0.0.1` | Yes |
| `--user` | MySQL username | `--user=root` | Yes |
| `--password` | MySQL password | `--password=your_password` | Yes |
| `--compress` | Compression type | `--compress=zstd` | No (recommended: zstd) |
| `--estimated-size` | Estimated backup size | `--estimated-size=50GB` | No (for progress display) |
| `--io-limit` | Bandwidth limit | `--io-limit=300MB/s` | No (default 200MB/s) |
| `--parallel` | Parallel threads | `--parallel=8` | No (default 4) |

### 5.2 Receiver Parameters (--download)

| Parameter | Description | Example | Required |
|-----------|-------------|---------|----------|
| `--download` | Download mode | `--download` | Yes |
| `--stream-port` | Listening port | `--stream-port=9999` | Yes |
| `--target-dir` | Extraction target directory | `--target-dir=/backup/mysql` | Either this or `--output` |
| `--output` | Output path | `--output=/backup/backup.xb` or `--output=-` | Either this or `--target-dir` |
| `--compress` | Compression type | `--compress=zstd` | Yes (must match sender) |
| `--estimated-size` | Estimated size | `--estimated-size=50GB` | No (for progress display) |
| `--io-limit` | Bandwidth limit | `--io-limit=300MB/s` | No (default 200MB/s) |
| `--parallel` | Parallel threads | `--parallel=8` | No (default 4) |

### 5.3 Prepare Parameters (--prepare)

| Parameter | Description | Example | Required |
|-----------|-------------|---------|----------|
| `--prepare` | Prepare mode | `--prepare` | Yes |
| `--target-dir` | Backup directory | `--target-dir=/backup/mysql` | Yes |
| `--parallel` | Parallel threads | `--parallel=8` | No (default 4) |
| `--use-memory` | Memory size | `--use-memory=4G` | No (default 1G) |
| `--host` | MySQL server address | `--host=127.0.0.1` | No (for getting config file) |
| `--user` | MySQL username | `--user=root` | No (for getting config file) |
| `--port` | MySQL port | `--port=3306` | No (for getting config file) |

---

## 6. Important Notes

### 6.1 Network Requirements

- **Firewall Configuration**: Ensure sender can access receiver's specified port
- **Network Bandwidth**: Recommend network bandwidth at least 2x backup data size for stable transfer
- **Latency**: Low latency network environment provides better transfer performance

### 6.2 Directory Permissions

- **Receiver Target Directory**: Ensure write permissions
- **Log Directory**: Default `/var/log/mysql-backup-helper`, needs create permission or use `--log-dir` to specify other directory

### 6.3 Compression Type Matching

- **Sender and receiver must use the same compression type**
- Recommended: `zstd` (best performance)
- If sender uses `--compress=zstd`, receiver must also use `--compress=zstd`

### 6.4 Target Directory Handling

- If target directory exists and is not empty, program will ask if you want to clear it
- Input `y` or `yes` to continue (will clear directory)
- Input `n` or other value to cancel

---

## 7. Troubleshooting

### 7.1 Log File Location

All operation logs are saved in:
- **Default location**: `/var/log/mysql-backup-helper/backup-helper-{timestamp}.log`
- **Custom location**: Specify via `--log-dir` parameter

```bash
# View latest logs
ls -lt /var/log/mysql-backup-helper/ | head -5

# View latest log content
tail -f /var/log/mysql-backup-helper/backup-helper-*.log
```

### 7.2 Common Issues

#### Issue 1: Connection Refused

**Error message**:
```
Failed to connect to 192.168.1.100:9999: connection refused
```

**Troubleshooting steps**:
1. Check if receiver is started
2. Check if firewall allows the port
3. Check network connectivity: `telnet 192.168.1.100 9999` or `nc -zv 192.168.1.100 9999`
4. Check receiver log file

**Solution**:
```bash
# Check receiver process
ps aux | grep mysql-backup-helper

# Check port listening
netstat -tlnp | grep 9999
# or
ss -tlnp | grep 9999

# Check firewall (CentOS/RHEL)
firewall-cmd --list-ports
# or
iptables -L -n | grep 9999

# Temporarily open port (for testing)
firewall-cmd --add-port=9999/tcp --permanent
firewall-cmd --reload
```

#### Issue 2: xbstream Extraction Failed

**Error message**:
```
xbstream: failed to create file
xbstream extraction failed: exit status 1
```

**Troubleshooting steps**:
1. Check target directory permissions
2. Check if disk space is sufficient
3. Check if target directory has existing files (needs clearing)

**Solution**:
```bash
# Check directory permissions
ls -ld /backup/mysql-restore

# Check disk space
df -h /backup/mysql-restore

# Manually clear directory (if confirmed overwrite)
rm -rf /backup/mysql-restore/*
# or
rm -rf /backup/mysql-restore/* /backup/mysql-restore/.*
```

#### Issue 3: Compression Tool Not Found

**Error message**:
```
zstd: executable file not found in $PATH
```

**Troubleshooting steps**:
1. Check if compression tool is installed
2. Check if tool is in PATH

**Solution**:
```bash
# Check if zstd is installed
which zstd
zstd --version

# If not installed, install zstd
# CentOS/RHEL
yum install zstd -y
# or
dnf install zstd -y

# Ubuntu/Debian
apt-get update && apt-get install zstd -y

# macOS
brew install zstd
```

#### Issue 4: Inaccurate Progress Display

**Reason**: `--estimated-size` parameter not provided

**Solution**:
- Use `--estimated-size` parameter to specify estimated size
- Supports units: KB, MB, GB, TB
- Example: `--estimated-size=50GB`

#### Issue 5: Slow Transfer Speed

**Troubleshooting steps**:
1. Check network bandwidth
2. Check `--io-limit` setting
3. Check receiver disk I/O performance

**Solution**:
```bash
# Check current rate limit setting
# If set too low, can increase:
--io-limit=500MB/s

# If unlimited (not recommended):
--io-limit=-1

# Check disk I/O
iostat -x 1 5
```

#### Issue 6: Prepare Failed

**Error message**:
```
xtrabackup: Error: cannot find /backup/mysql-restore/ibdata1
```

**Troubleshooting steps**:
1. Check if target directory exists
2. Check if backup is completely extracted
3. Check backup file integrity

**Solution**:
```bash
# Check directory contents
ls -la /backup/mysql-restore/

# Check if key files exist
test -f /backup/mysql-restore/ibdata1 && echo "ibdata1 exists" || echo "ibdata1 missing"
test -f /backup/mysql-restore/xtrabackup_checkpoints && echo "checkpoints exists" || echo "checkpoints missing"

# Re-extract if files are incomplete
```

### 7.3 Log Analysis

Log files contain the following module identifiers:

- `[SYSTEM]`: System-level logs
- `[BACKUP]`: Backup operation logs
- `[TCP]`: TCP transfer logs
- `[DOWNLOAD]`: Download operation logs
- `[DECOMPRESS]`: Decompression logs
- `[XBSTREAM]`: xbstream extraction logs
- `[PREPARE]`: Prepare operation logs

**View specific module logs**:
```bash
# View backup-related logs
grep "\[BACKUP\]" /var/log/mysql-backup-helper/backup-helper-*.log

# View TCP transfer logs
grep "\[TCP\]" /var/log/mysql-backup-helper/backup-helper-*.log

# View error logs
grep -i "error\|failed\|fatal" /var/log/mysql-backup-helper/backup-helper-*.log
```

---

## 8. Performance Optimization Recommendations

### 8.1 Network Optimization

- **Use dedicated network**: Separate backup network from business network
- **Increase bandwidth**: Ensure sufficient network bandwidth
- **Reduce latency**: Choose low-latency network paths

### 8.2 Parallel Processing

- **Adjust parallel count**: Adjust `--parallel` parameter based on server CPU cores
- **Recommended setting**: `--parallel=8` (8-core CPU) or `--parallel=16` (16-core CPU)

### 8.3 Memory Configuration

- **Prepare operation**: Adjust `--use-memory` based on available memory
- **Recommended setting**: `--use-memory=4G` or `--use-memory=8G`

### 8.4 Bandwidth Rate Limiting

- **Default rate limit**: 200MB/s (protects system resources)
- **Production environment**: Adjust `--io-limit` based on actual network bandwidth
- **Fast transfer**: Can set to `--io-limit=500MB/s` or higher

---

## 9. Security Recommendations

### 9.1 Password Security

- **Do not use `--password` parameter**: Program will prompt for password, avoiding password in command history
- **Use config file**: Sensitive information can be stored in config file with appropriate permissions

### 9.2 Network Security

- **Use VPN or dedicated network**: Use dedicated network for backup transfer in production
- **Firewall configuration**: Limit access source IP

### 9.3 Log Security

- **Log permissions**: Ensure log file permissions are set appropriately (recommended 600)
- **Log cleanup**: Regularly clean old log files

---

## 10. Example Scripts

### 10.1 Complete Backup Script (Sender Side)

```bash
#!/bin/bash
# backup.sh - Backup script

BACKUP_HOST="172.24.215.241"
BACKUP_PORT="9999"
MYSQL_HOST="127.0.0.1"
MYSQL_USER="root"
MYSQL_PASSWORD="your_password"
ESTIMATED_SIZE="50GB"

mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=${BACKUP_HOST} \
  --stream-port=${BACKUP_PORT} \
  --host=${MYSQL_HOST} \
  --user=${MYSQL_USER} \
  --password=${MYSQL_PASSWORD} \
  --compress=zstd \
  --estimated-size=${ESTIMATED_SIZE} \
  --io-limit=300MB/s \
  --parallel=8

if [ $? -eq 0 ]; then
    echo "Backup completed successfully!"
else
    echo "Backup failed! Check log file for details."
    exit 1
fi
```

### 10.2 Complete Receive Script (Receiver Side)

```bash
#!/bin/bash
# receive.sh - Receive script

BACKUP_PORT="9999"
TARGET_DIR="/backup/mysql-restore"
ESTIMATED_SIZE="50GB"

mysql-backup-helper --download \
  --stream-port=${BACKUP_PORT} \
  --target-dir=${TARGET_DIR} \
  --compress=zstd \
  --estimated-size=${ESTIMATED_SIZE} \
  --io-limit=300MB/s \
  --parallel=8

if [ $? -eq 0 ]; then
    echo "Download and extraction completed successfully!"
    
    # Auto execute prepare
    echo "Starting prepare operation..."
    mysql-backup-helper --prepare \
      --target-dir=${TARGET_DIR} \
      --parallel=8 \
      --use-memory=4G
    
    if [ $? -eq 0 ]; then
        echo "Backup is ready for restore!"
    else
        echo "Prepare failed! Check log file for details."
        exit 1
    fi
else
    echo "Download failed! Check log file for details."
    exit 1
fi
```

---

## 11. Technical Support

### 11.1 Get Help

```bash
# View help information
mysql-backup-helper --help

# View version information
mysql-backup-helper --version
```

### 11.2 Log Files

Detailed logs for all operations are saved in:
- `/var/log/mysql-backup-helper/backup-helper-{timestamp}.log`

### 11.3 Issue Reporting

If you encounter issues, please provide:
1. Error message (complete output)
2. Log file content
3. Operation steps
4. System environment information

---

## Appendix A: Quick Reference

### A.1 Sender Command Template

```bash
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=<Receiver IP> \
  --stream-port=9999 \
  --host=<MySQL Host> \
  --user=<MySQL User> \
  --password=<Password> \
  --compress=zstd \
  --estimated-size=<Size> \
  --io-limit=300MB/s
```

### A.2 Receiver Command Template (Direct Extraction)

```bash
mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=<Target Directory> \
  --compress=zstd \
  --estimated-size=<Size> \
  --io-limit=300MB/s
```

### A.3 Receiver Command Template (Save to File)

```bash
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=<Output File> \
  --compress=zstd \
  --estimated-size=<Size> \
  --io-limit=300MB/s
```

### A.4 Prepare Command Template

```bash
mysql-backup-helper --prepare \
  --target-dir=<Backup Directory> \
  --parallel=8 \
  --use-memory=4G
```

---

## Appendix B: Version History

- **v1.0.0** (2025-11-06)
  - Initial delivery version
  - Support streaming backup transfer
  - Support zstd and qpress compression
  - Support progress display and bandwidth rate limiting
  - Unified logging system

---

**Document Version**: 1.0  
**Last Updated**: 2025-11-06  
**Maintainer**: MySQL Backup Helper Team

