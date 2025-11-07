# MySQL Backup Helper

高效的 MySQL 物理备份与 OSS 上传工具，支持 Percona XtraBackup、阿里云 OSS、流式推送、自动压缩、自动多语言。

---

## 依赖要求

### Go 版本要求
- **Go 1.21 及以上**（推荐使用最新版 Go 工具链）
- 如 go.mod 中存在 `toolchain` 字段，低于该版本的 Go 工具链将无法 build，请删除 `toolchain` 行或升级 Go 版本。

### 必需依赖
- **Percona XtraBackup**：用于 MySQL 物理备份
  - [下载地址](https://www.percona.com/downloads/Percona-XtraBackup-LATEST/)
  - 安装后确保 `xtrabackup` 命令在 PATH 中
- **MySQL 服务器连接**：工具通过 TCP/IP 协议连接 MySQL 服务器
  - 不需要安装 `mysql` 命令行客户端工具
  - 不需要本地 `mysqld` 或 socket 文件
  - 只需要能够通过 TCP/IP 连接到 MySQL 服务器（host:port）

### 可选依赖
- **zstd**：用于 zstd 压缩（当使用 `--compress=zstd` 时）
  - [下载地址](https://github.com/facebook/zstd)
  - 安装后确保 `zstd` 命令在 PATH 中

---

## 配置文件（config.json）示例

```json
{
  "endpoint": "http://oss-cn-hangzhou.aliyuncs.com",
  "accessKeyId": "your-access-key-id",
  "accessKeySecret": "your-access-key-secret",
  "securityToken": "",
  "bucketName": "your-bucket-name",
  "objectName": "backup/your-backup",   // 只需前缀，实际文件名会自动加时间戳和后缀
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

- **objectName**：只需指定前缀，最终 OSS 文件名会自动变为 `objectName_YYYYMMDDHHMM后缀`，如 `backup/your-backup_202507181648.xb.zst`
- **compressType**：压缩类型，可选值：`zstd`、`qp`（qpress）或空字符串/`no`（不压缩）。支持所有模式（oss、stream）
- **streamPort**：流式传输端口，设为 `0` 表示自动查找可用端口
- **streamHost**：远程主机 IP，用于主动推送模式
- **existedBackup**：已存在的备份文件路径，用于上传或流式传输（使用'-'表示从stdin读取）
- **logDir**：日志文件存储目录，默认为 `/var/log/mysql-backup-helper`，支持相对路径和绝对路径
- **downloadOutput**：下载模式默认输出路径
- **remoteOutput**：SSH 模式下远程保存路径
- **ioLimit**：IO 带宽限制（字节/秒），设为 `0` 使用默认值（200MB/s），设为 `-1` 表示不限速
- **parallel**：并行线程数（默认：4），用于 xtrabackup 备份、压缩、解压缩和 xbstream 解包操作
- **useMemory**：准备操作使用的内存大小（默认：1G），支持单位（如 '1G', '512M'）
- **xtrabackupPath**：xtrabackup 二进制文件路径或包含 xtrabackup/xbstream 的目录路径。优先级：命令行参数 > 配置文件 > 环境变量 `XTRABACKUP_PATH` > PATH 查找
- 其它参数可通过命令行覆盖，命令行参数优先于配置文件。

**注意**：工具会自动处理以下 xtrabackup 选项，无需用户配置：
- `--defaults-file`：可通过 `--defaults-file` 参数手动指定 MySQL 配置文件路径（my.cnf）。如果不指定，不会自动检测，避免使用错误的配置文件
- `--close-files=1`：自动启用，用于处理大量表的情况
- 文件描述符限制：自动设置为 655360（通过 ulimit）

**兼容性说明**：
- 工具支持广泛的 xtrabackup/xbstream 版本，包括不支持 `--version` 参数的旧版本（如 xbstream 2.4.12）
- 工具会使用多重回退机制验证二进制文件的可执行性（`--version` → `-h` → `--help` → 无参数运行）
- `--prepare` 模式不需要 xbstream，仅需要 xtrabackup
- 工具不依赖 `mysql` 命令行客户端，通过 Go MySQL 驱动直接连接 MySQL 服务器
- 获取配置文件路径时，如果无法查询 MySQL 变量（如权限不足），会优雅降级到检查常见路径

---

## 命令行参数

| 参数                | 说明                                                         |
|---------------------|--------------------------------------------------------------|
| --config            | 配置文件路径（如 `config.json`）                             |
| --host              | MySQL 主机（优先于配置文件）                                 |
| --port              | MySQL 端口（优先于配置文件）                                 |
| --user              | MySQL 用户名（优先于配置文件）                               |
| --password          | MySQL 密码（优先于配置文件，未指定则交互输入）               |
| --backup            | 启动备份流程（否则只做参数检查）                             |
| --check             | 预检查模式：执行预检验证。可单独使用（检查所有模式）或与其他模式组合（如 `--check --backup` 只检查备份模式） |
| --download          | 下载模式：从 TCP 流接收备份数据并保存                       |
| --prepare           | 准备模式：执行 xtrabackup --prepare 使备份可用于恢复         |
| --output            | 下载模式输出文件路径（使用 '-' 表示输出到 stdout，默认：backup_YYYYMMDDHHMMSS.xb） |
| --target-dir        | 目录：下载模式用于解包目录，准备模式用于备份目录             |
| --mode              | 备份模式：`oss`（上传到 OSS）或 `stream`（推送到 TCP 端口）  |
| --stream-port       | 流式推送时监听的本地端口（如 9999，设为 0 则自动查找空闲端口），或指定远程端口（当使用 --stream-host 时） |
| --stream-host       | 远程主机 IP（如 '192.168.1.100'）。指定后主动连接到远程服务器推送数据，类似 `nc host port` |
| --ssh               | 使用 SSH 在远程主机自动启动接收服务（需要 --stream-host，依赖系统 SSH 配置） |
| --remote-output     | SSH 模式下远程保存路径（默认：自动生成） |
| --compress          | 压缩：`qp`（qpress）、`zstd` 或 `no`（不压缩）。不带值时默认使用 qp。支持所有模式（oss、stream）          |
| --lang              | 语言：`zh`（中文）或 `en`（英文），不指定则自动检测系统语言   |
| --ai-diagnose=on/off| 操作失败时 AI 诊断，on 为自动诊断（需配置 Qwen API Key），off 为跳过，未指定时交互式询问。支持所有模块（BACKUP、PREPARE、TCP、OSS、EXTRACT等） |
| --enable-handshake   | TCP流推送启用握手认证（默认false，可在配置文件设置）         |
| --stream-key         | TCP流推送握手密钥（默认空，可在配置文件设置）                |
| --existed-backup     | 已存在的xtrabackup备份文件路径，用于上传或流式传输（使用'-'表示从stdin读取） |
| --estimated-size     | 预估备份大小，支持单位（如 '100MB', '1GB'）或字节（用于进度跟踪） |
| --io-limit           | IO 带宽限制，支持单位（如 '100MB/s', '1GB/s'）或字节/秒，使用 -1 表示不限速 |
| --parallel           | 并行线程数（默认：4），用于 xtrabackup 备份（--parallel）、qpress 压缩（--compress-threads）、zstd 压缩/解压缩（-T）、xbstream 解包（--parallel）和 xtrabackup 解压缩（--parallel） |
| --use-memory         | 准备操作使用的内存大小（如 '1G', '512M'），默认：1G          |
| --defaults-file      | MySQL 配置文件路径（my.cnf）。如果不指定，不会自动检测，也不会传递给 xtrabackup |
| --xtrabackup-path    | xtrabackup 二进制文件路径或包含 xtrabackup/xbstream 的目录路径（覆盖配置文件和环境变量） |
| -y, --yes            | 非交互模式：自动对所有提示回答 'yes'（包括目录覆盖确认和 AI 诊断确认） |
| --version, -v        | 显示版本信息                                                      |

---

## 典型用法

### 1. 编译

```sh
go build -a -o backup-helper main.go
```

### 2. 一键备份并上传 OSS（自动中文/英文）

```sh
./backup-helper --config config.json --backup --mode=oss
```

### 3. 指定英文界面

```sh
./backup-helper --config config.json --backup --mode=oss --lang=en
```

### 4. 指定压缩类型

```sh
./backup-helper --config config.json --backup --mode=oss --compress=zstd
./backup-helper --config config.json --backup --mode=oss --compress=qp
./backup-helper --config config.json --backup --mode=oss --compress=no
./backup-helper --config config.json --backup --mode=oss --compress
```

### 5. 流式推送（stream 模式）

```sh
./backup-helper --config config.json --backup --mode=stream --stream-port=9999
# 另一个终端拉流
nc 127.0.0.1 9999 > streamed-backup.xb
```

### 5.1. 自动查找空闲端口（推荐）

```sh
./backup-helper --config config.json --backup --mode=stream --stream-port=0
# 程序会自动找到空闲端口并显示本地 IP 和端口
# 输出示例：
# [backup-helper] Listening on 192.168.1.100:54321
# [backup-helper] Waiting for remote connection...
# 另一个终端拉流（使用显示的端口）
nc 192.168.1.100 54321 > streamed-backup.xb
```

- **stream 模式下所有压缩参数均无效，始终为原始物理备份流。**
- **自动查找端口时会自动获取本地 IP 并显示在输出中，便于远程连接。**
- **使用 `--stream-host` 可以主动推送到远程服务器，接收端使用 `--download --stream-port` 在指定端口监听。**

### 5.2. 主动推送到远程服务器

```sh
# 发送端：主动连接到远程服务器并推送数据
./backup-helper --config config.json --backup --mode=stream --stream-host=192.168.1.100 --stream-port=9999

# 接收端：在远程服务器上监听并接收数据
./backup-helper --download --stream-port=9999
```

这样可以实现类似 `xtrabackup | nc 192.168.1.100 9999` 的功能。

### 5.3. SSH 模式：自动在远程启动接收服务

如果有 SSH 权限，可以使用 `--ssh` 选项自动在远程主机启动接收服务，无需手动操作：

```sh
# SSH 模式 + 自动发现端口（推荐）
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=replica-server \
    --ssh \
    --remote-output=/backup/mysql_backup.xb \
    --estimated-size=10GB

# SSH 模式 + 指定端口
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=replica-server \
    --ssh \
    --stream-port=9999 \
    --remote-output=/backup/mysql_backup.xb

# 传统模式：需要提前在远程运行接收服务
./backup-helper --config config.json --backup --mode=stream \
    --stream-host=replica-server \
    --stream-port=9999
```

**SSH 模式说明：**
- 使用 `--ssh` 时，程序会通过 SSH 在远程主机自动执行 `backup-helper --download` 命令
- 依赖系统已有的 SSH 配置（`~/.ssh/config`、密钥等），无需额外配置
- 如果指定了 `--stream-port`，在远程的该端口启动服务；如果未指定，自动发现可用端口
- 传输完成后自动清理远程进程
- 类似 `rsync -e ssh` 的使用方式，如果 SSH 密钥已配置好，直接就能用

### 6. 预检查模式（--check）

`--check` 模式可以单独使用，也可以与其他模式组合使用：

```sh
# 单独使用：检查所有模式（BACKUP、DOWNLOAD、PREPARE）
./backup-helper --check

# 检查所有模式（包括 MySQL 兼容性检查）
./backup-helper --check --host=127.0.0.1 --user=root --password=yourpass --port=3306

# 只检查备份模式（不执行备份）
./backup-helper --check --backup --host=127.0.0.1 --user=root --password=yourpass

# 只检查下载模式（不执行下载）
./backup-helper --check --download --target-dir=/path/to/extract

# 只检查准备模式（不执行准备）
./backup-helper --check --prepare --target-dir=/path/to/backup

# 指定压缩类型进行检查
./backup-helper --check --compress=zstd --host=127.0.0.1 --user=root --password=yourpass
```

**检查内容：**
- **依赖检查**：验证 xtrabackup、xbstream、zstd、qpress 等工具是否已安装
- **MySQL 兼容性检查**（备份模式）：MySQL 版本、xtrabackup 版本兼容性、数据大小估算、复制参数、配置文件验证
- **系统资源检查**（单独 --check 时）：CPU 核心数、内存大小、网络接口
- **参数推荐**（备份模式）：基于系统资源推荐 parallel、io-limit、use-memory 等参数
- **目标目录检查**（下载/准备模式）：验证目录是否存在、可写、包含备份文件等

**重要提示：**
- 当使用 `--backup`、`--download` 或 `--prepare` 时，工具会在执行前自动进行预检查
- 如果预检查发现重大问题（ERROR），工具会停止执行并提示修复
- 使用 `--check` 组合模式（如 `--check --backup`）时，只进行检查，不执行实际操作

### 7. 仅做参数检查（不备份）

```sh
./backup-helper --config config.json
```

### 8. 纯命令行参数（无 config.json）

```sh
./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306 --backup --mode=oss --compress=qp
```

### 9. 上传已存在的备份文件到 OSS

```sh
./backup-helper --config config.json --existed-backup backup.xb --mode=oss
```

### 10. 通过 TCP 流式传输已存在的备份文件

```sh
./backup-helper --config config.json --existed-backup backup.xb --mode=stream --stream-port=9999
# 另一个终端拉流
nc 127.0.0.1 9999 > streamed-backup.xb
```

### 11. 使用 cat 命令从 stdin 读取并上传到 OSS

```sh
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=oss
```

### 12. 使用 cat 命令从 stdin 读取并通过 TCP 传输

```sh
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=stream --stream-port=9999
```

### 13. 手动指定上传限速（如限制到 100 MB/s）

```sh
./backup-helper --config config.json --backup --mode=oss --io-limit 100MB/s
# 支持单位：KB/s, MB/s, GB/s, TB/s，也可以直接使用字节/秒
```

### 14. 禁用限速（不限速上传）

```sh
./backup-helper --config config.json --backup --mode=oss --io-limit -1
# 使用 -1 表示完全禁用限速，以最大速度上传
```

### 15. 指定预估大小以显示准确的进度

```sh
./backup-helper --config config.json --backup --mode=oss --estimated-size 1GB
# 支持单位：KB, MB, GB, TB，也可以直接使用字节
# 例如：--estimated-size 1073741824 或 --estimated-size 1GB
```

### 15. 准备备份（Prepare Mode）

备份完成后，需要执行 prepare 操作使备份可用于恢复：

```sh
# 基本用法
./backup-helper --prepare --target-dir=/path/to/backup

# 指定并行线程数和内存大小
./backup-helper --prepare --target-dir=/path/to/backup --parallel=8 --use-memory=2G

# 使用配置文件
./backup-helper --config config.json --prepare --target-dir=/path/to/backup

# 可选：提供 MySQL 连接信息和 --defaults-file
./backup-helper --prepare --target-dir=/path/to/backup --host=127.0.0.1 --user=root --port=3306 --defaults-file=/etc/my.cnf
```

**说明**：
- `--target-dir`：必需，指定要准备的备份目录
- `--parallel`：并行线程数，默认 4（可使用配置文件或在命令行指定）
- `--use-memory`：准备操作使用的内存大小，默认 1G（支持单位：G, M, K）
- `--defaults-file`：可选，手动指定 MySQL 配置文件路径（如果不指定，不会自动检测）

### 16. 下载模式：从 TCP 流接收备份数据

```sh
# 下载到默认文件（backup_YYYYMMDDHHMMSS.xb）
./backup-helper --download --stream-port 9999

# 下载到指定文件
./backup-helper --download --stream-port 9999 --output my_backup.xb

# 流式输出到 stdout（可用于管道压缩或解包）
./backup-helper --download --stream-port 9999 --output - | zstd -d > backup.xb

# 直接使用 xbstream 解包到目录（未压缩备份）
./backup-helper --download --stream-port 9999 --output - | xbstream -x -C /path/to/extract/dir

# Zstd 压缩备份：流式解压后解包（推荐方式）
./backup-helper --download --stream-port 9999 --compress=zstd --target-dir /path/to/extract/dir

# Zstd 压缩备份：流式输出到 stdout（可用于管道到 xbstream）
./backup-helper --download --stream-port 9999 --compress=zstd --output - | xbstream -x -C /path/to/extract/dir

# Qpress 压缩备份：自动解压和解包（注意：需要先保存文件，不支持流式解压）
./backup-helper --download --stream-port 9999 --compress=qp --target-dir /path/to/extract/dir

# 保存 zstd 压缩的备份（自动解压）
./backup-helper --download --stream-port 9999 --compress=zstd --output my_backup.xb

# 带限速下载
./backup-helper --download --stream-port 9999 --io-limit 100MB/s

# 带进度显示（需要提供预估大小）
./backup-helper --download --stream-port 9999 --estimated-size 1GB

# 非交互模式：自动确认所有提示
./backup-helper --download --stream-port 9999 --target-dir /backup/mysql --compress=zstd -y
```

**注意**：
- 如果 `--target-dir` 指定的目录已存在且不为空，程序会询问是否覆盖现有文件
- 输入 `y` 或 `yes` 继续提取（可能覆盖现有文件）
- 输入 `n` 或任何其他值取消提取并退出
- 使用 `-y` 或 `--yes` 参数可以自动确认所有提示（非交互模式），适合脚本和自动化场景

**下载模式压缩类型说明：**

- **Zstd 压缩（`--compress=zstd`）**：
  - 支持流式解压，可直接解压并解包到目录
  - 使用 `--target-dir` 时，自动执行 `zstd -d | xbstream -x`
  - 使用 `--output -` 时，输出解压后的流，可继续管道到 `xbstream`

- **Qpress 压缩（`--compress=qp` 或 `--compress`）**：
  - **不支持流式解压**（MySQL 5.7 的 xbstream 不支持 `--decompress` 流式操作）
  - 使用 `--target-dir` 时，会先保存压缩文件，然后使用 `xbstream -x` 解包，最后使用 `xtrabackup --decompress` 解压
  - 使用 `--output -` 时，会警告并输出原始压缩流

- **未压缩备份**：
  - 不指定 `--compress` 时，直接保存或解包
  - 使用 `--target-dir` 时，直接使用 `xbstream -x` 解包

---

## 日志与对象命名

### 统一日志系统

工具采用统一日志系统，将所有关键操作的日志记录到单个日志文件中：

- **日志文件命名**：`backup-helper-{timestamp}.log`（如 `backup-helper-20251106105903.log`）
- **日志存储位置**：默认在 `/var/log/mysql-backup-helper`，可通过 `--config` 或配置文件中的 `logDir` 指定（支持相对路径和绝对路径）
- **日志内容**：统一记录所有操作步骤
  - **[BACKUP]**：xtrabackup 备份操作
  - **[PREPARE]**：xtrabackup prepare 操作
  - **[TCP]**：TCP 流传输（发送/接收）
  - **[OSS]**：OSS 上传操作
  - **[XBSTREAM]**：xbstream 解包操作
  - **[DECOMPRESS]**：解压缩操作（zstd/qpress）
  - **[EXTRACT]**：提取操作
  - **[SYSTEM]**：系统级别的日志

- **日志格式**：每条日志包含时间戳和模块前缀，格式为 `[YYYY-MM-DD HH:MM:SS] [MODULE] 消息内容`
- **日志清理**：自动清理旧日志，仅保留最近 10 个日志文件
- **错误处理**：
  - 操作完成或失败时，会在控制台显示日志文件位置
  - 失败时自动提取错误摘要并显示在控制台
  - 所有模块支持 AI 诊断（需配置 Qwen API Key）
  - **传输中断检测**：自动检测 TCP 连接中断、进程异常终止等情况，记录到日志文件并中止流程，避免处理不完整的数据

示例日志内容：
```
[2025-11-06 10:59:03] [SYSTEM] === MySQL Backup Helper Log Started ===
[2025-11-06 10:59:03] [SYSTEM] Timestamp: 2025-11-06 10:59:03
[2025-11-06 10:59:03] [BACKUP] Starting backup operation
[2025-11-06 10:59:03] [BACKUP] Command: xtrabackup --backup --stream=xbstream ...
[2025-11-06 10:59:03] [TCP] Listening on 192.168.1.100:9999
[2025-11-06 10:59:03] [TCP] Client connected
[2025-11-06 10:59:03] [TCP] Transfer started
```

### OSS 对象命名

- OSS 对象名自动加时间戳，如 `backup/your-backup_202507181648.xb.zst`，便于归档和查找。

## 进度跟踪

工具会在备份上传过程中实时显示进度信息：

- **实时进度**：显示已上传/已下载大小、总大小、百分比（未压缩时）、传输速度和持续时间
  - 启用压缩时，不显示百分比（因为压缩后的实际大小与原始大小不一致）
  - 未压缩时，显示完整进度：`Progress: 100 MB / 500 MB (20.0%) - 50 MB/s - Duration: 2s`
  - 压缩时，仅显示：`Progress: 100 MB - 50 MB/s - Duration: 2s`
- **最终统计**：显示总上传/总下载大小、持续时间、平均速度
- **大小计算**：
  - 如果提供了 `--estimated-size`，直接使用该值（支持单位：KB, MB, GB, TB）
  - 对于实时备份，自动计算 MySQL datadir 大小
  - 对于已有备份文件，自动读取文件大小
  - 从 stdin 读取时，无法获取大小，只显示上传量和速度

## 带宽限速

- **默认限速**：如果不指定 `--io-limit`，默认使用 200 MB/s 的限速
- **手动限速**：使用 `--io-limit` 指定上传/下载带宽限制
  - 支持单位：`KB/s`, `MB/s`, `GB/s`, `TB/s`（如 `100MB/s`, `1GB/s`）
  - 也可以直接使用字节/秒（如 `104857600` 表示 100 MB/s）
  - 使用 `-1` 表示完全禁用限速（不限速上传）
- **配置文件**：可以在配置文件中设置 `ioLimit` 字段（单位：字节/秒），支持使用 `--io-limit` 命令行参数覆盖

示例输出（未压缩）：
```
[backup-helper] IO rate limit set to: 100.0 MB/s

Progress: 1.1 GB / 1.5 GB (73.3%) - 98.5 MB/s - Duration: 11.4s
Progress: 1.3 GB / 1.5 GB (86.7%) - 99.2 MB/s - Duration: 13.1s
[backup-helper] Upload completed!
  Total uploaded: 1.5 GB
  Duration: 15s
  Average speed: 102.4 MB/s
```

示例输出（启用压缩）：
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

## 多语言支持

- 自动检测系统语言（支持中文/英文），也可通过 `--lang=zh` 或 `--lang=en` 强制切换。
- 所有终端输出均支持中英文切换。

---

## 常见问题

- **zstd 未安装**：请先安装 zstd 并确保在 PATH 中。
- **OSS 上传失败**：请检查配置文件中的 OSS 相关参数。
- **MySQL 连接失败**：请检查数据库主机、端口、用户名、密码。
- **日志堆积**：程序会自动清理日志目录，仅保留最近 10 个日志文件。
- **日志位置**：操作完成或失败时，会在控制台显示日志文件完整路径，便于排查问题。
- **传输中断**：如果传输过程中连接中断，系统会自动检测并记录错误日志，中止流程。请检查日志文件了解详细错误信息。

---

如需更多高级用法或遇到问题，请查阅源码或提交 issue。

## Makefile 使用说明

- `make build`：编译 backup-helper 可执行文件。
- `make clean`：清理编译产物。
- `make test`：自动运行 test.sh，覆盖多语言、压缩、流式、AI诊断等集成测试。

### 测试账号准备

- 请在 MySQL 中准备两个账号：
  - 一个拥有足够备份权限的账号（如 `root` 或具备 `RELOAD`, `LOCK TABLES`, `PROCESS`, `REPLICATION CLIENT` 等权限）。
  - 一个权限不足的账号（如只具备 `SELECT` 权限），用于触发备份失败和 AI 诊断测试。
- 在 `config.json` 中分别配置这两个账号进行不同场景测试。

## 版本管理

- `make version`：显示当前版本号
- `make get-version`：获取当前版本号（用于脚本）
- `make set-version VER=1.0.1`：设置新版本号
- `./version.sh show`：显示当前版本号
- `./version.sh set 1.0.1`：设置新版本号
- `./version.sh get`：获取当前版本号（用于脚本）