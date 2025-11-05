# MySQL Backup Helper

高效的 MySQL 物理备份与传输工具，支持 Percona XtraBackup、阿里云 OSS、TCP 流式传输、自动压缩、AI 诊断、自动多语言。

## ✨ 特性

- 🚀 **高性能备份**：基于 Percona XtraBackup 的物理备份
- ☁️ **多种传输方式**：支持阿里云 OSS 上传和 TCP 流式传输
- 🗜️ **智能压缩**：支持 zstd、qpress 压缩算法
- 🌐 **多语言支持**：自动检测系统语言（中文/英文）
- 📊 **实时进度**：实时显示备份进度、速度、剩余时间
- 🔒 **安全传输**：支持 TCP 流认证
- 🤖 **AI 诊断**：独立 AI 命令支持日志诊断和问答（Qwen）
- ⚡ **带宽控制**：可配置上传/下载速率限制

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
- **zstd**：用于 zstd 压缩（当使用 `--compress-type=zstd` 时）
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
  "objectName": "backup/your-backup",
  "size": 104857600,
  "buffer": 10,
  "ioLimit": 209715200,
  "mysqlHost": "127.0.0.1",
  "mysqlPort": 3306,
  "mysqlUser": "root",
  "mysqlPassword": "your-mysql-password",
  "compress": true,
  "compressType": "zstd",
  "mode": "oss",
  "streamPort": 9999,
  "enableAuth": false,
  "authKey": "your-secret-key",
  "logDir": "/var/log/mysql-backup-helper",
  "qwenAPIKey": ""
}
```

### 配置字段说明

#### OSS 配置
- **endpoint**: OSS 端点地址
- **accessKeyId**: 阿里云 AccessKey ID
- **accessKeySecret**: 阿里云 AccessKey Secret
- **bucketName**: OSS 存储桶名称
- **objectName**: OSS 对象前缀（最终文件名会自动加时间戳和后缀，如 `backup/your-backup_202507181648.xb.zst`）

#### 上传配置
- **size**: 分片上传大小（字节，默认 100MB）
- **buffer**: 缓冲区数量（默认 10）
- **ioLimit**: IO 带宽限制（字节/秒，默认 200MB/s，0表示使用默认值）
- **traffic**: ⚠️ 已废弃，请使用 `ioLimit` 代替

#### MySQL 配置
- **mysqlHost**: MySQL 主机地址
- **mysqlPort**: MySQL 端口（默认 3306）
- **mysqlUser**: MySQL 用户名
- **mysqlPassword**: MySQL 密码

#### 压缩配置
- **compress**: 是否启用压缩（true/false）
- **compressType**: 压缩类型（zstd、qp 或留空）

#### 模式配置
- **mode**: 备份模式（oss 或 stream，默认 oss）
- **streamPort**: TCP 端口号（0=自动查找空闲端口）
- **enableAuth**: 是否启用流认证（默认 false）
- **authKey**: 认证密钥（用于流传输身份验证）

#### 其他配置
- **logDir**: 日志文件存储目录（默认 `/var/log/mysql-backup-helper`，支持相对/绝对路径）
- **qwenAPIKey**: Qwen AI API 密钥（用于 AI 命令）

**注意**：命令行参数会覆盖配置文件中的设置。

---

## 📖 命令行使用

### 全局参数

| 参数          | 说明                                           |
|---------------|------------------------------------------------|
| --config      | 配置文件路径（可选）                           |
| --lang        | 语言：zh（中文）或 en（英文），默认自动检测   |
| --verbose, -v | 详细输出模式                                   |
| --quiet, -q   | 安静模式（最小输出）                           |

### 子命令

#### 1. `backup` - 执行 MySQL 备份并传输

**用途**：连接 MySQL，执行 xtrabackup 备份，并上传到 OSS 或通过 TCP 流传输。

**参数**：

| 参数                | 说明                                                    |
|---------------------|--------------------------------------------------------|
| --host              | MySQL 主机地址                                         |
| --port              | MySQL 端口（默认 3306）                                |
| --user              | MySQL 用户名                                           |
| --password          | MySQL 密码（未指定则交互输入）                         |
| --mode              | 备份模式：oss 或 stream（默认：oss）                   |
| --stream-port       | TCP 流端口号（仅 stream 模式，0=自动查找）             |
| --compress-type     | 压缩类型：zstd、qp 或 none                             |
| --io-limit          | IO 带宽限制（如 '100MB/s'，-1=不限速）                 |
| --enable-auth       | 启用流认证（仅 stream 模式）                           |
| --auth-key          | 认证密钥（仅 stream 模式）                             |

**示例**：
```bash
# 备份并上传到 OSS
backup-helper backup --host 127.0.0.1 --user root --mode oss

# 备份并通过 TCP 流传输
backup-helper backup --host 127.0.0.1 --user root --mode stream --stream-port 9000

# 使用 zstd 压缩并限速
backup-helper backup --host 127.0.0.1 --user root --mode oss \
  --compress-type zstd --io-limit 100MB/s
```

#### 2. `send` - 发送已有备份文件

**用途**：将已有的备份文件上传到 OSS 或通过 TCP 流传输。

**参数**：

| 参数                | 说明                                       |
|---------------------|--------------------------------------------|
| --file              | 备份文件路径（'-' 表示从 stdin 读取）      |
| --stdin             | 从 stdin 读取备份数据                      |
| --mode              | 传输模式：oss 或 stream（默认：oss）       |
| --stream-port       | TCP 流端口号（仅 stream 模式）             |
| --skip-validation   | 跳过备份文件验证                           |
| --validate-only     | 仅验证文件，不传输                         |
| --io-limit          | IO 带宽限制                                |
| --enable-auth       | 启用流认证（仅 stream 模式）               |
| --auth-key          | 认证密钥（仅 stream 模式）                 |

**示例**：
```bash
# 上传备份文件到 OSS
backup-helper send --file backup.xb --mode oss

# 通过 TCP 流传输备份文件
backup-helper send --file backup.xb --mode stream --stream-port 9000

# 从 stdin 读取并上传
cat backup.xb | backup-helper send --stdin --mode oss

# 仅验证备份文件
backup-helper send --file backup.xb --validate-only
```

#### 3. `receive` - 接收备份数据

**用途**：从 TCP 流接收备份数据并保存。

**参数**：

| 参数                | 说明                                              |
|---------------------|---------------------------------------------------|
| --from-stream       | 监听的 TCP 端口（0=自动查找）                     |
| --output            | 输出文件路径（'-' 表示输出到 stdout，默认自动生成）|
| --stdout            | 输出到 stdout                                     |
| --io-limit          | IO 带宽限制                                       |
| --enable-auth       | 启用流认证                                        |
| --auth-key          | 认证密钥                                          |

**示例**：
```bash
# 接收备份并保存到文件
backup-helper receive --from-stream 9000 --output backup.xb

# 接收备份并输出到 stdout（可用于管道）
backup-helper receive --from-stream 9000 --stdout | xbstream -x

# 自动查找端口
backup-helper receive --from-stream 0
```

#### 4. `ai` - AI 诊断和问答

**用途**：使用 AI 诊断备份日志文件或回答 MySQL 备份相关问题。

**参数**：

| 参数                | 说明                                       |
|---------------------|--------------------------------------------|
| --log-file, -f      | 要诊断的备份日志文件路径                   |
| --question          | 向 AI 提问关于 MySQL 备份的问题            |

**示例**：
```bash
# 诊断备份日志文件
backup-helper ai --log-file /var/log/mysql-backup-helper/backup.log

# 向 AI 提问
backup-helper ai --question "如何解决 Access denied 错误？"

# 使用短选项
backup-helper ai -f /var/log/mysql-backup-helper/backup.log
```

---

## 🚀 快速开始

### 1. 编译

```bash
# 使用 Makefile
make build

# 或手动编译
go build -o backup-helper
```

### 2. 查看帮助

```bash
./backup-helper --help
./backup-helper backup --help
./backup-helper send --help
./backup-helper receive --help
./backup-helper ai --help
```

### 3. 基本用法示例

#### 场景 1：备份并上传到 OSS

```bash
# 使用配置文件
./backup-helper backup --config config.json --mode oss

# 纯命令行参数
./backup-helper backup --host 127.0.0.1 --user root --password xxx \
  --mode oss --compress-type zstd
```

#### 场景 2：备份并通过 TCP 流传输

```bash
# 发送端（备份端）
./backup-helper backup --host 127.0.0.1 --user root --mode stream --stream-port 9000

# 接收端
./backup-helper receive --from-stream 9000 --output backup.xb
```

#### 场景 3：上传已有备份文件

```bash
# 上传到 OSS
./backup-helper send --file backup.xb --mode oss

# 通过 TCP 流传输
./backup-helper send --file backup.xb --mode stream --stream-port 9000
```

---

## 💡 典型使用场景

### 1. 完整备份工作流（OSS）

```bash
# 步骤1：执行备份并上传
./backup-helper backup \
  --host 127.0.0.1 \
  --user root \
  --password yourpassword \
  --mode oss \
  --compress-type zstd \
  --io-limit 100MB/s
```

### 2. 跨网络备份传输（TCP Stream）

```bash
# 在目标服务器（接收端）
./backup-helper receive --from-stream 9000 --output /backup/mysql_backup.xb \
  --enable-auth --auth-key "your-secret-key"

# 在源服务器（备份端）
./backup-helper backup \
  --host 127.0.0.1 \
  --user root \
  --mode stream \
  --stream-port 9000 \
  --enable-auth \
  --auth-key "your-secret-key"
```

### 3. 自动查找空闲端口

```bash
# 接收端：自动查找端口
./backup-helper receive --from-stream 0
# 输出：[backup-helper] Listening on 192.168.1.100:54321

# 备份端：使用显示的端口
./backup-helper backup --host 127.0.0.1 --user root --mode stream --stream-port 54321
```

### 4. 使用管道传输

```bash
# 从 stdin 读取并上传
cat backup.xb | ./backup-helper send --stdin --mode oss

# 接收并直接解包
./backup-helper receive --from-stream 9000 --stdout | xbstream -x -C /data/mysql
```

### 5. 验证备份文件

```bash
# 仅验证，不传输
./backup-helper send --file backup.xb --validate-only
```

### 6. 指定英文界面

```bash
./backup-helper backup --lang en --host 127.0.0.1 --user root --mode oss
```

### 7. 禁用限速（最大速度）

```bash
./backup-helper backup --host 127.0.0.1 --user root --mode oss --io-limit -1
```

### 8. 不同压缩类型

```bash
# zstd 压缩（推荐，压缩率高）
./backup-helper backup --host 127.0.0.1 --user root --mode oss --compress-type zstd

# qpress 压缩
./backup-helper backup --host 127.0.0.1 --user root --mode oss --compress-type qp

# 不压缩
./backup-helper backup --host 127.0.0.1 --user root --mode oss --compress-type none
```

**注意**：stream 模式下压缩参数无效，始终传输原始数据流。

### 9. AI 诊断使用

```bash
# 诊断备份日志文件（需要在 config.json 中配置 qwenAPIKey）
./backup-helper ai --log-file /var/log/mysql-backup-helper/backup_20240101.log

# 向 AI 提问
./backup-helper ai --question "如何优化 MySQL 备份速度？"

# 使用短选项
./backup-helper ai -f /var/log/mysql-backup-helper/backup.log
```

**备份失败时的提示**：
当备份失败时，工具会自动提示使用 AI 诊断命令：
```
Backup failed (no 'completed OK!').
You can check the backup log file for details: /var/log/mysql-backup-helper/backup_20240101.log

💡 Tip: Use AI to diagnose the issue:
   mysql-backup-helper ai --log-file /var/log/mysql-backup-helper/backup_20240101.log
```

---

## 日志与对象命名

- 所有备份日志自动保存在 `logs/` 目录，仅保留最近 10 个日志文件。
- OSS 对象名自动加时间戳，如 `backup/your-backup_202507181648.xb.zst`，便于归档和查找。

## 进度跟踪

工具会在备份上传过程中实时显示进度信息：

- **实时进度**：显示已上传/已下载大小、总大小、百分比、传输速度和持续时间
- **最终统计**：显示总上传/总下载大小、持续时间、平均速度
- **自动大小检测**：
  - 对于实时备份，自动计算 MySQL datadir 大小
  - 对于已有备份文件，自动读取文件大小
  - 从 stdin 读取时，无法获取大小，只显示上传量和速度

## 带宽限速

- **默认限速**：如果不指定 `--io-limit`，默认使用 200 MB/s 的限速
- **手动限速**：使用 `--io-limit` 指定上传/下载带宽限制
  - 支持单位：`KB/s`, `MB/s`, `GB/s`, `TB/s`（如 `100MB/s`, `1GB/s`）
  - 也可以直接使用字节/秒（如 `104857600` 表示 100 MB/s）
  - 使用 `-1` 表示完全禁用限速（不限速上传）
- **配置文件**：可以在配置文件中设置 `ioLimit` 字段，或使用 `traffic` 字段（单位：字节/秒，已废弃）

示例输出：
```
[backup-helper] IO rate limit set to: 100.0 MB/s

Progress: 1.1 GB / 1.5 GB (73.3%) - 98.5 MB/s - Duration: 11.4s
Progress: 1.3 GB / 1.5 GB (86.7%) - 99.2 MB/s - Duration: 13.1s
[backup-helper] Upload completed!
  Total uploaded: 1.5 GB
  Duration: 15s
  Average speed: 102.4 MB/s
```

---

## 🌐 多语言支持

- 自动检测系统语言（支持中文/英文）
- 可通过 `--lang=zh` 或 `--lang=en` 强制切换
- 所有终端输出均支持中英文切换

---

## 🔧 开发与贡献

### 代码质量

本项目采用现代化的 Go 开发实践：

- ✅ **清晰的分层架构**：遵循 DDD 和 Clean Architecture 原则
- ✅ **依赖注入**：使用构造函数注入，提高可测试性
- ✅ **统一错误处理**：自定义错误类型系统
- ✅ **代码复用**：公共函数提取，消除重复代码
- ✅ **单一职责**：方法职责单一，易于维护和测试

### 最近优化（v1.0.0-alpha）

#### 1. 命令行接口重构
- 从单一命令改为子命令架构（`backup`、`send`、`receive`）
- 更符合 UNIX 哲学和用户习惯
- 提供更清晰的功能划分

#### 2. 代码质量提升
- 提取公共函数到 `cmd/common.go`，减少 **30%+** 重复代码
- 拆分大方法为小方法，提高可读性和可测试性
- 引入统一错误类型系统（`internal/pkg/errors`）
- 统一配置字段（`ioLimit` 优先于 `traffic`）

#### 3. 服务层优化
- 将 `BackupService.Execute()` 从 200+ 行拆分为 10+ 个小方法
- 每个方法职责单一：验证连接、执行备份、传输数据、验证结果等
- 提高了代码的可维护性和可测试性

#### 4. 架构改进
- 更清晰的关注点分离
- 更好的依赖管理
- 为单元测试打下良好基础

### 贡献指南

欢迎贡献代码、报告问题或提出建议！

1. Fork 本项目
2. 创建特性分支（`git checkout -b feature/AmazingFeature`）
3. 提交更改（`git commit -m 'Add some AmazingFeature'`）
4. 推送到分支（`git push origin feature/AmazingFeature`）
5. 开启 Pull Request

---

## ❓ 常见问题

### 安装问题

**Q: zstd 未安装怎么办？**
```bash
# macOS
brew install zstd

# Ubuntu/Debian
sudo apt-get install zstd

# CentOS/RHEL
sudo yum install zstd
```

**Q: xtrabackup 未找到？**

请访问 [Percona XtraBackup 下载页面](https://www.percona.com/downloads/Percona-XtraBackup-LATEST/) 安装。

### 使用问题

**Q: OSS 上传失败？**

检查配置文件中的 OSS 相关参数：
- `endpoint` 是否正确
- `accessKeyId` 和 `accessKeySecret` 是否有效
- `bucketName` 是否存在且有写入权限

**Q: MySQL 连接失败？**

检查：
- 主机地址和端口是否正确
- 用户名和密码是否正确
- MySQL 用户是否有足够的备份权限（RELOAD, LOCK TABLES, PROCESS, REPLICATION CLIENT）

**Q: 备份失败但没有错误信息？**

查看日志文件（默认在 `/var/log/mysql-backup-helper/` 或 `logs/` 目录）。

**Q: 如何使用 AI 诊断？**

在配置文件中添加：
```json
{
  "qwenAPIKey": "your-qwen-api-key"
}
```

然后使用 `ai` 命令：
```bash
# 诊断日志文件
./backup-helper ai --log-file /var/log/mysql-backup-helper/backup.log

# 提问
./backup-helper ai --question "如何解决连接超时问题？"
```

### 性能问题

**Q: 备份速度慢？**

1. 检查是否设置了 `--io-limit`，如需全速备份使用 `-1`
2. 考虑使用 `qp` 压缩代替 `zstd`（压缩速度更快）
3. 使用 `--mode stream` 代替 `--mode oss`（跳过 OSS 上传延迟）

**Q: 日志文件堆积？**

程序会自动清理日志目录，仅保留最近 10 个日志文件。如需修改，可以在代码中调整 `cleanOldLogs()` 函数的参数。

---

如需更多帮助或遇到其他问题，请查阅源码或提交 issue。

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