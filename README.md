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
- **parallel**：xtrabackup 并行线程数（默认：4）
- 其它参数可通过命令行覆盖，命令行参数优先于配置文件。

**注意**：工具会自动处理以下 xtrabackup 选项，无需用户配置：
- `--defaults-file`：自动从 MySQL 连接获取配置文件路径（my.cnf）
- `--close-files=1`：自动启用，用于处理大量表的情况
- 文件描述符限制：自动设置为 655360（通过 ulimit）

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
| --download          | 下载模式：从 TCP 流接收备份数据并保存                       |
| --output            | 下载模式输出文件路径（使用 '-' 表示输出到 stdout，默认：backup_YYYYMMDDHHMMSS.xb） |
| --target-dir        | 解包目录：下载后自动解压（如需要）和解包到指定目录 |
| --mode              | 备份模式：`oss`（上传到 OSS）或 `stream`（推送到 TCP 端口）  |
| --stream-port       | 流式推送时监听的本地端口（如 9999，设为 0 则自动查找空闲端口），或指定远程端口（当使用 --stream-host 时） |
| --stream-host       | 远程主机 IP（如 '192.168.1.100'）。指定后主动连接到远程服务器推送数据，类似 `nc host port` |
| --ssh               | 使用 SSH 在远程主机自动启动接收服务（需要 --stream-host，依赖系统 SSH 配置） |
| --remote-output     | SSH 模式下远程保存路径（默认：自动生成） |
| --compress          | 压缩：`qp`（qpress）、`zstd` 或 `no`（不压缩）。不带值时默认使用 qp。支持所有模式（oss、stream）          |
| --lang              | 语言：`zh`（中文）或 `en`（英文），不指定则自动检测系统语言   |
| --ai-diagnose=on/off| 备份失败时 AI 诊断，on 为自动诊断（需配置 Qwen API Key），off 为跳过，未指定时交互式询问 |
| --enable-handshake   | TCP流推送启用握手认证（默认false，可在配置文件设置）         |
| --stream-key         | TCP流推送握手密钥（默认空，可在配置文件设置）                |
| --existed-backup     | 已存在的xtrabackup备份文件路径，用于上传或流式传输（使用'-'表示从stdin读取） |
| --estimated-size     | 预估备份大小，支持单位（如 '100MB', '1GB'）或字节（用于进度跟踪） |
| --io-limit           | IO 带宽限制，支持单位（如 '100MB/s', '1GB/s'）或字节/秒，使用 -1 表示不限速 |
| --parallel           | xtrabackup 并行线程数（默认：4） |
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

### 6. 仅做参数检查（不备份）

```sh
./backup-helper --config config.json
```

### 7. 纯命令行参数（无 config.json）

```sh
./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306 --backup --mode=oss --compress=qp
```

### 8. 上传已存在的备份文件到 OSS

```sh
./backup-helper --config config.json --existed-backup backup.xb --mode=oss
```

### 9. 通过 TCP 流式传输已存在的备份文件

```sh
./backup-helper --config config.json --existed-backup backup.xb --mode=stream --stream-port=9999
# 另一个终端拉流
nc 127.0.0.1 9999 > streamed-backup.xb
```

### 10. 使用 cat 命令从 stdin 读取并上传到 OSS

```sh
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=oss
```

### 11. 使用 cat 命令从 stdin 读取并通过 TCP 传输

```sh
cat backup.xb | ./backup-helper --config config.json --existed-backup - --mode=stream --stream-port=9999
```

### 12. 手动指定上传限速（如限制到 100 MB/s）

```sh
./backup-helper --config config.json --backup --mode=oss --io-limit 100MB/s
# 支持单位：KB/s, MB/s, GB/s, TB/s，也可以直接使用字节/秒
```

### 13. 禁用限速（不限速上传）

```sh
./backup-helper --config config.json --backup --mode=oss --io-limit -1
# 使用 -1 表示完全禁用限速，以最大速度上传
```

### 14. 指定预估大小以显示准确的进度

```sh
./backup-helper --config config.json --backup --mode=oss --estimated-size 1GB
# 支持单位：KB, MB, GB, TB，也可以直接使用字节
# 例如：--estimated-size 1073741824 或 --estimated-size 1GB
```

### 15. 下载模式：从 TCP 流接收备份数据

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
```

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

- 所有备份日志自动保存在 `logs/` 目录，仅保留最近 10 个日志文件。
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
- **日志堆积**：程序会自动清理 logs 目录，仅保留最近 10 个日志。

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