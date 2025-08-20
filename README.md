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
  "objectName": "backup/your-backup",   // 只需前缀，实际文件名会自动加时间戳和后缀
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

- **objectName**：只需指定前缀，最终 OSS 文件名会自动变为 `objectName_YYYYMMDDHHMM后缀`，如 `backup/your-backup_202507181648.xb.zst`
- **existedBackup**：已存在的备份文件路径，用于上传或流式传输（使用'-'表示从stdin读取）
- 其它参数可通过命令行覆盖，命令行参数优先于配置文件。

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
| --mode              | 备份模式：`oss`（上传到 OSS）或 `stream`（推送到 TCP 端口）  |
| --stream-port       | 流式推送时监听的本地端口（如 9999）                          |
| --compress          | 启用压缩                                                  |
| --compress-type     | 压缩类型：`qp`（qpress）、`zstd`          |
| --lang              | 语言：`zh`（中文）或 `en`（英文），不指定则自动检测系统语言   |
| --ai-diagnose=on/off| 备份失败时 AI 诊断，on 为自动诊断（需配置 Qwen API Key），off 为跳过，未指定时交互式询问 |
| --enable-handshake   | TCP流推送启用握手认证（默认false，可在配置文件设置）         |
| --stream-key         | TCP流推送握手密钥（默认空，可在配置文件设置）                |
| --existed-backup     | 已存在的xtrabackup备份文件路径，用于上传或流式传输（使用'-'表示从stdin读取） |
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
./backup-helper --config config.json --backup --mode=oss --compress-type=zstd
./backup-helper --config config.json --backup --mode=oss --compress-type=qp
./backup-helper --config config.json --backup --mode=oss --compress-type=none
```

### 5. 流式推送（stream 模式）

```sh
./backup-helper --config config.json --backup --mode=stream --stream-port=9999
# 另一个终端拉流
nc 127.0.0.1 9999 > streamed-backup.xb
```

- **stream 模式下所有压缩参数均无效，始终为原始物理备份流。**

### 6. 仅做参数检查（不备份）

```sh
./backup-helper --config config.json
```

### 7. 纯命令行参数（无 config.json）

```sh
./backup-helper --host=127.0.0.1 --user=root --password=123456 --port=3306 --backup --mode=oss --compress-type=qp
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

---

## 日志与对象命名

- 所有备份日志自动保存在 `logs/` 目录，仅保留最近 10 个日志文件。
- OSS 对象名自动加时间戳，如 `backup/your-backup_202507181648.xb.zst`，便于归档和查找。

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