# MySQL Backup Helper 交付文档

## 版本信息

- **版本**: v1.0.0
- **构建日期**: 2025-11-06
- **主要功能**: 流式备份传输、自动压缩、远程接收

---

## 一、产品概述

MySQL Backup Helper 是一个高效的 MySQL 物理备份工具，专为流式备份传输而设计。主要特性包括：

### 核心特性

1. **流式备份传输**
   - 基于 TCP 的实时数据传输
   - 支持主动推送模式（类似 `xtrabackup | nc host port`）
   - 支持被动接收模式（监听端口接收数据）

2. **自动压缩**
   - 支持 Zstandard (zstd) 压缩（推荐）
   - 支持 qpress 压缩
   - 压缩和解压缩全流程自动化

3. **进度显示**
   - 实时显示传输进度、速度、已传输大小
   - 支持预估大小设置，显示百分比

4. **带宽限速**
   - 可配置的传输速率限制（默认 200MB/s）
   - 防止网络或磁盘 I/O 饱和

5. **统一日志**
   - 所有操作记录到统一日志文件
   - 便于问题排查和审核

---

## 二、安装与配置

### 2.1 下载二进制文件

```bash
# 下载最新版本
wget https://mysql-fanke.oss-cn-beijing.aliyuncs.com/backup-helper/mysql-backup-helper

# 或者使用 curl
curl -O https://mysql-fanke.oss-cn-beijing.aliyuncs.com/backup-helper/mysql-backup-helper

# 添加执行权限
chmod +x mysql-backup-helper
```

### 2.2 配置环境变量

#### Linux/macOS

```bash
# 创建安装目录（可选）
sudo mkdir -p /usr/local/bin

# 移动到系统目录
sudo mv mysql-backup-helper /usr/local/bin/

# 或者移动到自定义目录，例如：
sudo mv mysql-backup-helper /opt/mysql-backup-helper/

# 添加到 PATH（选择一种方式）：

# 方式1：添加到 ~/.bashrc（当前用户）
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
source ~/.bashrc

# 方式2：添加到 ~/.zshrc（如果使用 zsh）
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.zshrc
source ~/.zshrc

# 方式3：添加到 /etc/profile（所有用户）
echo 'export PATH=$PATH:/usr/local/bin' >> /etc/profile
source /etc/profile

# 验证安装
which mysql-backup-helper
mysql-backup-helper --version
```

#### 自定义安装路径

如果安装在自定义目录（如 `/opt/mysql-backup-helper/`）：

```bash
# 添加到 PATH
echo 'export PATH=$PATH:/opt/mysql-backup-helper' >> ~/.bashrc
source ~/.bashrc

# 验证
mysql-backup-helper --version
```

### 2.3 验证安装

```bash
# 检查版本
mysql-backup-helper --version

# 查看帮助信息
mysql-backup-helper --help
```

### 2.4 依赖要求

#### 必需依赖

- **Percona XtraBackup**: 用于 MySQL 物理备份
  - 下载地址：https://www.percona.com/downloads/Percona-XtraBackup-LATEST/
  - 安装后确保 `xtrabackup` 和 `xbstream` 命令在 PATH 中

#### 可选依赖（用于压缩）

- **zstd**: 用于 zstd 压缩（推荐）
  - 下载地址：https://github.com/facebook/zstd
  - 安装后确保 `zstd` 命令在 PATH 中

---

## 三、使用场景

### 场景1：流式备份 + 直接提取到目标目录（推荐）

这是最常用的场景，适用于备份完成后需要立即提取到目标目录的情况。

#### 发送端（备份服务器）

```bash
# 基本用法
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=192.168.1.100 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=your_password \
  --compress=zstd \
  --estimated-size=10GB

# 参数说明：
# --backup: 启动备份流程
# --mode=stream: 流式传输模式
# --stream-host: 接收端IP地址
# --stream-port: 接收端端口（默认9999）
# --host: MySQL服务器地址
# --user: MySQL用户名
# --password: MySQL密码
# --compress=zstd: 使用zstd压缩（推荐）
# --estimated-size: 预估备份大小（用于显示进度）
```

#### 接收端（备份存储服务器）

```bash
# 直接提取到目标目录（自动解压+解包）
mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=/backup/mysql \
  --compress=zstd \
  --estimated-size=10GB \
  --io-limit=200MB/s

# 参数说明：
# --download: 下载模式
# --stream-port: 监听端口
# --target-dir: 提取目标目录
# --compress=zstd: 指定压缩类型（必须与发送端一致）
# --estimated-size: 预估大小（用于显示进度）
# --io-limit: 带宽限制（可选，默认200MB/s）
```

**完整示例**：

```bash
# 发送端（172.24.215.240）
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=172.24.215.241 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=Test123! \
  --compress=zstd \
  --estimated-size=50GB

# 接收端（172.24.215.241）
mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=/backup/mysql-restore \
  --compress=zstd \
  --estimated-size=50GB
```

### 场景2：流式备份 + 保存为文件（流式输出）

适用于需要先保存备份文件，后续再手动处理的情况。

#### 发送端（同上）

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

#### 接收端（保存为文件）

```bash
# 保存为压缩文件（会自动解压）
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=/backup/mysql_backup.xb \
  --compress=zstd \
  --estimated-size=10GB

# 流式输出到标准输出（可用于管道操作）
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=- \
  --compress=zstd \
  2>/tmp/download.log | xbstream -x -C /backup/mysql-restore

# 参数说明：
# --output: 输出路径
#   - 指定路径: 保存到文件
#   - 设置为 "-": 输出到标准输出（stdout）
# 2>/tmp/download.log: 将日志和进度信息重定向到文件
```

**完整示例**：

```bash
# 发送端（172.24.215.240）
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=172.24.215.241 \
  --stream-port=9999 \
  --host=127.0.0.1 \
  --user=root \
  --password=Test123! \
  --compress=zstd \
  --estimated-size=50GB

# 接收端（172.24.215.241）- 保存为文件
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=/backup/mysql_backup_$(date +%Y%m%d).xb \
  --compress=zstd \
  --estimated-size=50GB

# 或者流式输出到标准输出
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=- \
  --compress=zstd \
  2>/tmp/download.log | xbstream -x -C /backup/mysql-restore
```

### 场景3：准备备份（Prepare）

备份传输完成后，需要对备份进行 prepare 操作才能用于恢复。

```bash
# 基本用法
mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore

# 指定并行线程数和内存
mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore \
  --parallel=8 \
  --use-memory=4G

# 提供MySQL连接信息以自动获取配置文件路径
mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore \
  --host=127.0.0.1 \
  --user=root \
  --port=3306

# 参数说明：
# --prepare: 准备模式
# --target-dir: 备份目录（必需）
# --parallel: 并行线程数（默认4）
# --use-memory: 使用的内存大小（默认1G）
# --host, --user, --port: 可选，用于自动获取MySQL配置文件路径
```

---

## 四、完整工作流程示例

### 4.1 典型备份流程

```bash
# ========== 步骤1：发送端启动备份 ==========
# 在备份服务器（172.24.215.240）执行：

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

# 输出示例：
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

# ========== 步骤2：接收端接收并提取 ==========
# 在备份存储服务器（172.24.215.241）执行：

mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=/backup/mysql-restore \
  --compress=zstd \
  --estimated-size=50GB \
  --io-limit=300MB/s

# 输出示例：
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

# ========== 步骤3：准备备份 ==========
# 在备份存储服务器执行：

mysql-backup-helper --prepare \
  --target-dir=/backup/mysql-restore \
  --parallel=8 \
  --use-memory=4G

# 输出示例：
# [backup-helper] Preparing backup in directory: /backup/mysql-restore
# [backup-helper] Parallel threads: 8
# [backup-helper] Use memory: 4G
# Equivalent shell command: xtrabackup --prepare --target-dir=/backup/mysql-restore --parallel=8 --use-memory=4G
# [backup-helper] Backup is ready for restore in: /backup/mysql-restore
# [backup-helper] Log file: /var/log/mysql-backup-helper/backup-helper-20251106120300.log
```

### 4.2 使用流式输出场景

```bash
# ========== 发送端（同上）==========

# ========== 接收端：流式输出并提取 ==========
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=- \
  --compress=zstd \
  2>/tmp/download.log | xbstream -x -C /backup/mysql-restore

# 注意：
# - 使用 --output=- 时，所有非数据输出（日志、进度）会输出到 stderr
# - 使用 2>/tmp/download.log 可以将日志保存到文件
# - 压缩数据会先被自动解压，然后通过管道传递给 xbstream
```

---

## 五、常用参数说明

### 5.1 发送端参数（--backup）

| 参数 | 说明 | 示例 | 是否必需 |
|------|------|------|----------|
| `--backup` | 启动备份流程 | `--backup` | 是 |
| `--mode` | 备份模式 | `--mode=stream` | 是 |
| `--stream-host` | 接收端IP地址 | `--stream-host=192.168.1.100` | 是 |
| `--stream-port` | 接收端端口 | `--stream-port=9999` | 否（默认9999） |
| `--host` | MySQL服务器地址 | `--host=127.0.0.1` | 是 |
| `--user` | MySQL用户名 | `--user=root` | 是 |
| `--password` | MySQL密码 | `--password=your_password` | 是 |
| `--compress` | 压缩类型 | `--compress=zstd` | 否（推荐zstd） |
| `--estimated-size` | 预估备份大小 | `--estimated-size=50GB` | 否（用于显示进度） |
| `--io-limit` | 带宽限制 | `--io-limit=300MB/s` | 否（默认200MB/s） |
| `--parallel` | 并行线程数 | `--parallel=8` | 否（默认4） |

### 5.2 接收端参数（--download）

| 参数 | 说明 | 示例 | 是否必需 |
|------|------|------|----------|
| `--download` | 下载模式 | `--download` | 是 |
| `--stream-port` | 监听端口 | `--stream-port=9999` | 是 |
| `--target-dir` | 提取目标目录 | `--target-dir=/backup/mysql` | 与 `--output` 二选一 |
| `--output` | 输出路径 | `--output=/backup/backup.xb` 或 `--output=-` | 与 `--target-dir` 二选一 |
| `--compress` | 压缩类型 | `--compress=zstd` | 是（必须与发送端一致） |
| `--estimated-size` | 预估大小 | `--estimated-size=50GB` | 否（用于显示进度） |
| `--io-limit` | 带宽限制 | `--io-limit=300MB/s` | 否（默认200MB/s） |
| `--parallel` | 并行线程数 | `--parallel=8` | 否（默认4） |

### 5.3 准备参数（--prepare）

| 参数 | 说明 | 示例 | 是否必需 |
|------|------|------|----------|
| `--prepare` | 准备模式 | `--prepare` | 是 |
| `--target-dir` | 备份目录 | `--target-dir=/backup/mysql` | 是 |
| `--parallel` | 并行线程数 | `--parallel=8` | 否（默认4） |
| `--use-memory` | 使用内存大小 | `--use-memory=4G` | 否（默认1G） |
| `--host` | MySQL服务器地址 | `--host=127.0.0.1` | 否（用于获取配置文件） |
| `--user` | MySQL用户名 | `--user=root` | 否（用于获取配置文件） |
| `--port` | MySQL端口 | `--port=3306` | 否（用于获取配置文件） |

---

## 六、重要注意事项

### 6.1 网络要求

- **防火墙配置**：确保发送端可以访问接收端的指定端口
- **网络带宽**：建议网络带宽至少是备份数据大小的 2 倍以确保稳定传输
- **延迟**：低延迟网络环境可获得更好的传输性能

### 6.2 目录权限

- **接收端目标目录**：确保有写入权限
- **日志目录**：默认 `/var/log/mysql-backup-helper`，需要创建权限或使用 `--log-dir` 指定其他目录

### 6.3 压缩类型匹配

- **发送端和接收端必须使用相同的压缩类型**
- 推荐使用 `zstd`（性能最佳）
- 如果发送端使用 `--compress=zstd`，接收端也必须使用 `--compress=zstd`

### 6.4 目标目录处理

- 如果目标目录已存在且不为空，程序会询问是否清空
- 输入 `y` 或 `yes` 继续（会清空目录）
- 输入 `n` 或其他值取消操作

---

## 七、问题排查

### 7.1 日志文件位置

所有操作的日志文件保存在：
- **默认位置**: `/var/log/mysql-backup-helper/backup-helper-{timestamp}.log`
- **自定义位置**: 通过 `--log-dir` 参数指定

```bash
# 查看最新日志
ls -lt /var/log/mysql-backup-helper/ | head -5

# 查看最新日志内容
tail -f /var/log/mysql-backup-helper/backup-helper-*.log
```

### 7.2 常见问题

#### 问题1：连接被拒绝

**错误信息**：
```
Failed to connect to 192.168.1.100:9999: connection refused
```

**排查步骤**：
1. 检查接收端是否已启动
2. 检查防火墙是否开放端口
3. 检查网络连通性：`telnet 192.168.1.100 9999` 或 `nc -zv 192.168.1.100 9999`
4. 检查接收端日志文件

**解决方案**：
```bash
# 检查接收端进程
ps aux | grep mysql-backup-helper

# 检查端口监听
netstat -tlnp | grep 9999
# 或
ss -tlnp | grep 9999

# 检查防火墙（CentOS/RHEL）
firewall-cmd --list-ports
# 或
iptables -L -n | grep 9999

# 临时开放端口（测试用）
firewall-cmd --add-port=9999/tcp --permanent
firewall-cmd --reload
```

#### 问题2：xbstream 提取失败

**错误信息**：
```
xbstream: failed to create file
xbstream extraction failed: exit status 1
```

**排查步骤**：
1. 检查目标目录权限
2. 检查磁盘空间是否充足
3. 检查目标目录是否已存在文件（需要清空）

**解决方案**：
```bash
# 检查目录权限
ls -ld /backup/mysql-restore

# 检查磁盘空间
df -h /backup/mysql-restore

# 手动清空目录（如果确认覆盖）
rm -rf /backup/mysql-restore/*
# 或
rm -rf /backup/mysql-restore/* /backup/mysql-restore/.*
```

#### 问题3：压缩工具未找到

**错误信息**：
```
zstd: executable file not found in $PATH
```

**排查步骤**：
1. 检查压缩工具是否安装
2. 检查工具是否在 PATH 中

**解决方案**：
```bash
# 检查 zstd 是否安装
which zstd
zstd --version

# 如果未安装，安装 zstd
# CentOS/RHEL
yum install zstd -y
# 或
dnf install zstd -y

# Ubuntu/Debian
apt-get update && apt-get install zstd -y

# macOS
brew install zstd
```

#### 问题4：进度显示不准确

**原因**：未提供 `--estimated-size` 参数

**解决方案**：
- 使用 `--estimated-size` 参数指定预估大小
- 支持单位：KB, MB, GB, TB
- 例如：`--estimated-size=50GB`

#### 问题5：传输速度慢

**排查步骤**：
1. 检查网络带宽
2. 检查 `--io-limit` 设置
3. 检查接收端磁盘 I/O 性能

**解决方案**：
```bash
# 检查当前限速设置
# 如果设置过低，可以增加：
--io-limit=500MB/s

# 如果不限速（不推荐）：
--io-limit=-1

# 检查磁盘I/O
iostat -x 1 5
```

#### 问题6：Prepare 失败

**错误信息**：
```
xtrabackup: Error: cannot find /backup/mysql-restore/ibdata1
```

**排查步骤**：
1. 检查目标目录是否存在
2. 检查备份是否完整提取
3. 检查备份文件完整性

**解决方案**：
```bash
# 检查目录内容
ls -la /backup/mysql-restore/

# 检查关键文件是否存在
test -f /backup/mysql-restore/ibdata1 && echo "ibdata1 exists" || echo "ibdata1 missing"
test -f /backup/mysql-restore/xtrabackup_checkpoints && echo "checkpoints exists" || echo "checkpoints missing"

# 重新执行提取（如果文件不完整）
```

### 7.3 日志分析

日志文件包含以下模块标识：

- `[SYSTEM]`: 系统级别日志
- `[BACKUP]`: 备份操作日志
- `[TCP]`: TCP 传输日志
- `[DOWNLOAD]`: 下载操作日志
- `[DECOMPRESS]`: 解压缩日志
- `[XBSTREAM]`: xbstream 提取日志
- `[PREPARE]`: Prepare 操作日志

**查看特定模块日志**：
```bash
# 查看备份相关日志
grep "\[BACKUP\]" /var/log/mysql-backup-helper/backup-helper-*.log

# 查看TCP传输日志
grep "\[TCP\]" /var/log/mysql-backup-helper/backup-helper-*.log

# 查看错误日志
grep -i "error\|failed\|fatal" /var/log/mysql-backup-helper/backup-helper-*.log
```

---

## 八、性能优化建议

### 8.1 网络优化

- **使用专用网络**：将备份网络与业务网络分离
- **增加带宽**：确保网络带宽充足
- **减少延迟**：选择低延迟的网络路径

### 8.2 并行处理

- **调整并行数**：根据服务器CPU核心数调整 `--parallel` 参数
- **推荐设置**：`--parallel=8`（8核CPU）或 `--parallel=16`（16核CPU）

### 8.3 内存配置

- **Prepare 操作**：根据可用内存调整 `--use-memory`
- **推荐设置**：`--use-memory=4G` 或 `--use-memory=8G`

### 8.4 带宽限速

- **默认限速**：200MB/s（保护系统资源）
- **生产环境**：根据实际网络带宽调整 `--io-limit`
- **快速传输**：可以设置为 `--io-limit=500MB/s` 或更高

---

## 九、安全建议

### 9.1 密码安全

- **不要使用 `--password` 参数**：程序会提示输入密码，避免密码出现在命令行历史中
- **使用配置文件**：敏感信息可以存储在配置文件中，并设置合适的权限

### 9.2 网络安全

- **使用VPN或专网**：在生产环境中使用专用网络进行备份传输
- **防火墙配置**：限制访问源IP

### 9.3 日志安全

- **日志权限**：确保日志文件权限设置合理（建议 600）
- **日志清理**：定期清理旧日志文件

---

## 十、示例脚本

### 10.1 完整备份脚本（发送端）

```bash
#!/bin/bash
# backup.sh - 备份脚本

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

### 10.2 完整接收脚本（接收端）

```bash
#!/bin/bash
# receive.sh - 接收脚本

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
    
    # 自动执行 prepare
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

## 十一、技术支持

### 11.1 获取帮助

```bash
# 查看帮助信息
mysql-backup-helper --help

# 查看版本信息
mysql-backup-helper --version
```

### 11.2 日志文件

所有操作的详细日志保存在：
- `/var/log/mysql-backup-helper/backup-helper-{timestamp}.log`

### 11.3 问题反馈

如遇到问题，请提供以下信息：
1. 错误信息（完整输出）
2. 日志文件内容
3. 操作步骤
4. 系统环境信息

---

## 附录A：快速参考

### A.1 发送端命令模板

```bash
mysql-backup-helper --backup \
  --mode=stream \
  --stream-host=<接收端IP> \
  --stream-port=9999 \
  --host=<MySQL主机> \
  --user=<MySQL用户> \
  --password=<密码> \
  --compress=zstd \
  --estimated-size=<大小> \
  --io-limit=300MB/s
```

### A.2 接收端命令模板（直接提取）

```bash
mysql-backup-helper --download \
  --stream-port=9999 \
  --target-dir=<目标目录> \
  --compress=zstd \
  --estimated-size=<大小> \
  --io-limit=300MB/s
```

### A.3 接收端命令模板（保存文件）

```bash
mysql-backup-helper --download \
  --stream-port=9999 \
  --output=<输出文件> \
  --compress=zstd \
  --estimated-size=<大小> \
  --io-limit=300MB/s
```

### A.4 Prepare 命令模板

```bash
mysql-backup-helper --prepare \
  --target-dir=<备份目录> \
  --parallel=8 \
  --use-memory=4G
```

---

## 附录B：版本历史

- **v1.0.0** (2025-11-06)
  - 初始交付版本
  - 支持流式备份传输
  - 支持 zstd 和 qpress 压缩
  - 支持进度显示和带宽限速
  - 统一日志系统

---

**文档版本**: 1.0  
**最后更新**: 2025-11-06  
**维护者**: MySQL Backup Helper Team

