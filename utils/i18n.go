package utils

import (
	"os"
	"strings"

	"github.com/gioco-play/easy-i18n/i18n"
	"github.com/jeandeaual/go-locale"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// InitI18nAuto initializes i18n, registers all translations, and sets the language based on system locale or LANG env.
func InitI18nAuto() {
	// Register all translations
	InitEn()

	// Detect system locale
	userLocales, _ := locale.GetLocales()
	i18n.Printf("Current locale: %s\n", userLocales[0])
	lang := language.English
	if len(userLocales) > 0 && strings.HasSuffix(strings.ToUpper(userLocales[0]), "CN") {
		lang = language.SimplifiedChinese
	} else if strings.Contains(os.Getenv("LANG"), "zh_CN") || strings.Contains(os.Getenv("LC_ALL"), "zh_CN") {
		lang = language.SimplifiedChinese
	}
	message.SetString(lang, "Current locale: %s\n", "当前语言环境: %s\n")
	message.SetString(language.English, "Current locale: %s\n", "Current locale: %s\n")
	// Set the language for i18n
	i18n.SetLang(lang)
}

// InitEn will init both English and Chinese support, all translations are registered here.
func InitEn() {
	// English
	message.SetString(language.English, "\t%s\t%s\t%s\t%s", "\t%s\t%s\t%s\t%s")
	message.SetString(language.English, "Backup Command Example(Percona XtraBackup):", "Backup Command Example(Percona XtraBackup):")
	message.SetString(language.English, "Checking MySQL Server Version...", "Checking MySQL Server Version...")
	message.SetString(language.English, "Version", "Version")
	message.SetString(language.English, "maybe incompatible", "maybe incompatible")
	message.SetString(language.English, "Your MySQL Server version may newer than version that provided On Alibaba Cloud, data file probably incompatible, read doc online for more info.", "Your MySQL Server version may newer than version that provided On Alibaba Cloud, data file probably incompatible, read doc online for more info.")
	message.SetString(language.English, "Parameter", "Parameter")
	message.SetString(language.English, "Multiple parameters are not supported", "Multiple parameters are not supported")
	message.SetString(language.English, "Recommended parameter: ibdata1", "Recommended parameter: ibdata1")
	message.SetString(language.English, "Checking backup related parameters...\n", "Checking backup related parameters...\n")
	message.SetString(language.English, "Backup related parameters checked...\n", "Backup related parameters checked...\n")
	message.SetString(language.English, "Checking replication parameters (these parameters affect master-slave replication, but do not affect backup) ...\n", "Checking replication parameters (these parameters affect master-slave replication, but do not affect backup) ...\n")
	message.SetString(language.English, "Recommended parameter: %s", "Recommended parameter: %s")
	message.SetString(language.English, "Replication parameter check completed", "Replication parameter check completed")
	message.SetString(language.English, "Parameter not set", "Parameter not set")
	message.SetString(language.English, "Get parameter for checking...\n", "Get parameter for checking...\n")
	message.SetString(language.English, "Parsing file: %s", "Parsing file: %s")
	message.SetString(language.English, "zstd command not found. Please install zstd: https://github.com/facebook/zstd", "zstd command not found. Please install zstd: https://github.com/facebook/zstd")
	message.SetString(language.English, "Transfer Started, ConsumedBytes: %d, TotalBytes %d.\n", "Transfer Started, ConsumedBytes: %d, TotalBytes %d.\n")
	message.SetString(language.English, "\rTransfer Data, ConsumedBytes: %d, TotalBytes %d, %d%%.", "\rTransfer Data, ConsumedBytes: %d, TotalBytes %d, %d%%.")
	message.SetString(language.English, "\nTransfer Completed, ConsumedBytes: %d, TotalBytes %d.\n", "\nTransfer Completed, ConsumedBytes: %d, TotalBytes %d.\n")
	message.SetString(language.English, "\nTransfer Failed, ConsumedBytes: %d, TotalBytes %d.\n", "\nTransfer Failed, ConsumedBytes: %d, TotalBytes %d.\n")
	message.SetString(language.English, "Please input mysql-server password: ", "Please input mysql-server password: ")
	message.SetString(language.English, "connect to mysql-server host=%s port=%d user=%s", "connect to mysql-server host=%s port=%d user=%s")
	message.SetString(language.English, "Load config error: %v\n", "Load config error: %v\n")
	message.SetString(language.English, "[backup-helper] Loaded config: %+v\n", "[backup-helper] Loaded config: %+v\n")
	message.SetString(language.English, "[backup-helper] Running xtrabackup...\n", "[backup-helper] Running xtrabackup...\n")
	message.SetString(language.English, "[backup-helper] Uploading to OSS...\n", "[backup-helper] Uploading to OSS...\n")
	message.SetString(language.English, "OSS upload error: %v\n", "OSS upload error: %v\n")
	message.SetString(language.English, "You must specify --stream-port when mode=stream\n", "You must specify --stream-port when mode=stream\n")
	message.SetString(language.English, "Stream server error: %v\n", "Stream server error: %v\n")
	message.SetString(language.English, "TCP stream error: %v\n", "TCP stream error: %v\n")
	message.SetString(language.English, "Unknown mode: %s\n", "Unknown mode: %s\n")
	message.SetString(language.English, "Run xtrabackup error: %v\n", "Run xtrabackup error: %v\n")
	message.SetString(language.English, "Backup log read error, OSS file deleted.\n", "Backup log read error, OSS file deleted.\n")
	message.SetString(language.English, "Backup log read error.\n", "Backup log read error.\n")
	message.SetString(language.English, "Backup failed (no 'completed OK!'), OSS file deleted.\n", "Backup failed (no 'completed OK!'), OSS file deleted.\n")
	message.SetString(language.English, "Backup failed (no 'completed OK!').\n", "Backup failed (no 'completed OK!').\n")
	message.SetString(language.English, "[backup-helper] Backup and upload completed!\n", "[backup-helper] Backup and upload completed!\n")
	message.SetString(language.English, "  MySQL backup pre-check\n", "  MySQL backup pre-check\n")
	message.SetString(language.English, "  2020-2020 Alibaba Cloud Inc\n", "  2020-2020 Alibaba Cloud Inc\n")

	// Chinese
	message.SetString(language.SimplifiedChinese, "\t%s\t%s\t%s\t%s", "\t%s\t%s\t%s\t%s")
	message.SetString(language.SimplifiedChinese, "Backup Command Example(Percona XtraBackup):", "备份命令参考(Percona XtraBackup):")
	message.SetString(language.SimplifiedChinese, "Checking MySQL Server Version...", "检查MySQL版本...")
	message.SetString(language.SimplifiedChinese, "Version", "版本")
	message.SetString(language.SimplifiedChinese, "maybe incompatible", "可能无法兼容")
	message.SetString(language.SimplifiedChinese, "Your MySQL Server version may newer than version that provided On Alibaba Cloud, data file probably incompatible, read doc online for more info.", "\t您需要通过物理备份迁移到云上的数据库小版本较高，云上MySQL可能无法兼容该版本的数据文件，可在MySQL全量备份上云帮助文档页面确认")
	message.SetString(language.SimplifiedChinese, "Parameter", "参数")
	message.SetString(language.SimplifiedChinese, "Multiple parameters are not supported", "不支持多参数")
	message.SetString(language.SimplifiedChinese, "Recommended parameter: ibdata1", "建议参数: ibdata1")
	message.SetString(language.SimplifiedChinese, "Checking backup related parameters...\n", "检查备份相关参数...\n")
	message.SetString(language.SimplifiedChinese, "Backup related parameters checked...\n", "备份相关参数检查完毕...\n")
	message.SetString(language.SimplifiedChinese, "Checking replication parameters (these parameters affect master-slave replication, but do not affect backup) ...\n", "检查复制参数中(以下参数影响主备复制, 并不影响备份)...\n")
	message.SetString(language.SimplifiedChinese, "Recommended parameter: %s", "建议参数: %s")
	message.SetString(language.SimplifiedChinese, "Replication parameter check completed", "复制参数检查完毕")
	message.SetString(language.SimplifiedChinese, "Parameter not set", "参数未设置")
	message.SetString(language.SimplifiedChinese, "Get parameter for checking...\n", "获取参数中...\n")
	message.SetString(language.SimplifiedChinese, "Parsing file: %s", "解析文件:%s")
	message.SetString(language.SimplifiedChinese, "zstd command not found. Please install zstd: https://github.com/facebook/zstd", "未找到zstd命令。请安装zstd: https://github.com/facebook/zstd")
	message.SetString(language.SimplifiedChinese, "Transfer Started, ConsumedBytes: %d, TotalBytes %d.\n", "开始传输，已消耗字节: %d, 总字节: %d.\n")
	message.SetString(language.SimplifiedChinese, "\rTransfer Data, ConsumedBytes: %d, TotalBytes %d, %d%%.", "\r传输数据，已消耗字节: %d, 总字节: %d, %d%%.")
	message.SetString(language.SimplifiedChinese, "\nTransfer Completed, ConsumedBytes: %d, TotalBytes %d.\n", "\n传输完成，已消耗字节: %d, 总字节: %d.\n")
	message.SetString(language.SimplifiedChinese, "\nTransfer Failed, ConsumedBytes: %d, TotalBytes %d.\n", "\n传输失败，已消耗字节: %d, 总字节: %d.\n")
	message.SetString(language.SimplifiedChinese, "Please input mysql-server password: ", "请输入数据库密码: ")
	message.SetString(language.SimplifiedChinese, "connect to mysql-server host=%s port=%d user=%s", "连接数据库host=%s port=%d user=%s")
	message.SetString(language.SimplifiedChinese, "Load config error: %v\n", "加载配置错误: %v\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Loaded config: %+v\n", "[backup-helper] 加载配置: %+v\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Running xtrabackup...\n", "[backup-helper] 正在运行xtrabackup...\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Uploading to OSS...\n", "[backup-helper] 正在上传到OSS...\n")
	message.SetString(language.SimplifiedChinese, "OSS upload error: %v\n", "OSS上传错误: %v\n")
	message.SetString(language.SimplifiedChinese, "You must specify --stream-port when mode=stream\n", "mode=stream时必须指定--stream-port\n")
	message.SetString(language.SimplifiedChinese, "Stream server error: %v\n", "流服务错误: %v\n")
	message.SetString(language.SimplifiedChinese, "TCP stream error: %v\n", "TCP流错误: %v\n")
	message.SetString(language.SimplifiedChinese, "Unknown mode: %s\n", "未知模式: %s\n")
	message.SetString(language.SimplifiedChinese, "Run xtrabackup error: %v\n", "运行xtrabackup错误: %v\n")
	message.SetString(language.SimplifiedChinese, "Backup log read error, OSS file deleted.\n", "备份日志读取错误，OSS文件已删除。\n")
	message.SetString(language.SimplifiedChinese, "Backup log read error.\n", "备份日志读取错误。\n")
	message.SetString(language.SimplifiedChinese, "Backup failed (no 'completed OK!'), OSS file deleted.\n", "备份失败（无'completed OK!'），OSS文件已删除。\n")
	message.SetString(language.SimplifiedChinese, "Backup failed (no 'completed OK!').\n", "备份失败（无'completed OK!'）。\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Backup and upload completed!\n", "[backup-helper] 备份和上传完成！\n")
	message.SetString(language.SimplifiedChinese, "  MySQL backup pre-check\n", "  MySQL备份前检查\n")
	message.SetString(language.SimplifiedChinese, "  2020-2020 Alibaba Cloud Inc\n", "  2020-2020 阿里云\n")

	message.SetString(language.English, "[OK] MySQL 5.7 detected xtrabackup 2.4 version, compatible", "[OK] MySQL 5.7 detected xtrabackup 2.4 version, compatible")
	message.SetString(language.English, "[Warning] MySQL 5.7 recommends xtrabackup 2.4, but detected version: %d.%d", "[Warning] MySQL 5.7 recommends xtrabackup 2.4, but detected version: %d.%d")
	message.SetString(language.English, "[OK] MySQL 8.0 detected xtrabackup 8.0 version, compatible\n", "[OK] MySQL 8.0 detected xtrabackup 8.0 version, compatible")
	message.SetString(language.English, "[Hint] Detected xtrabackup 8.0.34-29 or later, default zstd compression may cause recovery to fail.", "[Hint] Detected xtrabackup 8.0.34-29 or later, default zstd compression may cause recovery to fail.")
	message.SetString(language.English, "[Warning] MySQL 8.0 recommends xtrabackup 8.0, but detected version: %d.%d", "[Warning] MySQL 8.0 recommends xtrabackup 8.0, but detected version: %d.%d")
	message.SetString(language.English, "[Error] Cannot execute xtrabackup --version, please confirm that Percona XtraBackup is installed and in PATH", "[Error] Cannot execute xtrabackup --version, please confirm that Percona XtraBackup is installed and in PATH")

	message.SetString(language.SimplifiedChinese, "[OK] MySQL 5.7 detected xtrabackup 2.4 version, compatible", "[OK] MySQL 5.7 检测到 xtrabackup 2.4 版本，兼容")
	message.SetString(language.SimplifiedChinese, "[Warning] MySQL 5.7 recommends xtrabackup 2.4, but detected version: %d.%d", "[警告] MySQL 5.7 推荐使用 xtrabackup 2.4，但检测到版本: %d.%d")
	message.SetString(language.SimplifiedChinese, "[OK] MySQL 8.0 detected xtrabackup 8.0 version, compatible", "[OK] MySQL 8.0 检测到 xtrabackup 8.0 版本，兼容")
	message.SetString(language.SimplifiedChinese, "[Hint] Detected xtrabackup 8.0.34-29 or later, default zstd compression may cause recovery to fail.", "[提示] 检测到 xtrabackup 8.0.34-29 及以上，默认zstd压缩方式可能会导致恢复失败。")
	message.SetString(language.SimplifiedChinese, "[Warning] MySQL 8.0 recommends xtrabackup 8.0, but detected version: %d.%d", "[警告] MySQL 8.0 推荐使用 xtrabackup 8.0，但检测到版本: %d.%d")
	message.SetString(language.SimplifiedChinese, "[Error] Cannot execute xtrabackup --version, please confirm that Percona XtraBackup is installed and in PATH", "[错误] 无法执行 xtrabackup --version，请确认已安装 Percona XtraBackup 并在 PATH 中")

	// Add \n variants for all keys that are used with i18n.Printf("...\n")
	message.SetString(language.English, "Backup Command Example(Percona XtraBackup):\n", "Backup Command Example(Percona XtraBackup):\n")
	message.SetString(language.English, "Checking MySQL Server Version...\n", "Checking MySQL Server Version...\n")
	message.SetString(language.English, "connect to mysql-server host=%s port=%d user=%s\n", "connect to mysql-server host=%s port=%d user=%s\n")
	message.SetString(language.English, "[backup-helper] Loaded config: %+v\n", "[backup-helper] Loaded config: %+v\n")
	message.SetString(language.English, "[backup-helper] Running xtrabackup...\n", "[backup-helper] Running xtrabackup...\n")
	message.SetString(language.English, "[backup-helper] Uploading to OSS...\n", "[backup-helper] Uploading to OSS...\n")
	message.SetString(language.English, "OSS upload error: %v\n", "OSS upload error: %v\n")
	message.SetString(language.English, "You must specify --stream-port when mode=stream\n", "You must specify --stream-port when mode=stream\n")
	message.SetString(language.English, "Stream server error: %v\n", "Stream server error: %v\n")
	message.SetString(language.English, "TCP stream error: %v\n", "TCP stream error: %v\n")
	message.SetString(language.English, "Unknown mode: %s\n", "Unknown mode: %s\n")
	message.SetString(language.English, "Run xtrabackup error: %v\n", "Run xtrabackup error: %v\n")
	message.SetString(language.English, "Backup log read error, OSS file deleted.\n", "Backup log read error, OSS file deleted.\n")
	message.SetString(language.English, "Backup log read error.\n", "Backup log read error.\n")
	message.SetString(language.English, "Backup failed (no 'completed OK!'), OSS file deleted.\n", "Backup failed (no 'completed OK!'), OSS file deleted.\n")
	message.SetString(language.English, "Backup failed (no 'completed OK!').\n", "Backup failed (no 'completed OK!').\n")
	message.SetString(language.English, "[backup-helper] Backup and upload completed!\n", "[backup-helper] Backup and upload completed!\n")
	message.SetString(language.English, "  MySQL backup pre-check\n", "  MySQL backup pre-check\n")
	message.SetString(language.English, "  2020-2020 Alibaba Cloud Inc\n", "  2020-2020 Alibaba Cloud Inc\n")

	message.SetString(language.SimplifiedChinese, "Backup Command Example(Percona XtraBackup):\n", "备份命令参考(Percona XtraBackup):\n")
	message.SetString(language.SimplifiedChinese, "Checking MySQL Server Version...\n", "检查MySQL版本...\n")
	message.SetString(language.SimplifiedChinese, "connect to mysql-server host=%s port=%d user=%s\n", "连接数据库host=%s port=%d user=%s\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Loaded config: %+v\n", "[backup-helper] 加载配置: %+v\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Running xtrabackup...\n", "[backup-helper] 正在运行xtrabackup...\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Uploading to OSS...\n", "[backup-helper] 正在上传到OSS...\n")
	message.SetString(language.SimplifiedChinese, "OSS upload error: %v\n", "OSS上传错误: %v\n")
	message.SetString(language.SimplifiedChinese, "You must specify --stream-port when mode=stream\n", "mode=stream时必须指定--stream-port\n")
	message.SetString(language.SimplifiedChinese, "Stream server error: %v\n", "流服务错误: %v\n")
	message.SetString(language.SimplifiedChinese, "TCP stream error: %v\n", "TCP流错误: %v\n")
	message.SetString(language.SimplifiedChinese, "Unknown mode: %s\n", "未知模式: %s\n")
	message.SetString(language.SimplifiedChinese, "Run xtrabackup error: %v\n", "运行xtrabackup错误: %v\n")
	message.SetString(language.SimplifiedChinese, "Backup log read error, OSS file deleted.\n", "备份日志读取错误，OSS文件已删除。\n")
	message.SetString(language.SimplifiedChinese, "Backup log read error.\n", "备份日志读取错误。\n")
	message.SetString(language.SimplifiedChinese, "Backup failed (no 'completed OK!'), OSS file deleted.\n", "备份失败（无'completed OK!'），OSS文件已删除。\n")
	message.SetString(language.SimplifiedChinese, "Backup failed (no 'completed OK!').\n", "备份失败（无'completed OK!'）。\n")
	message.SetString(language.SimplifiedChinese, "[backup-helper] Backup and upload completed!\n", "[backup-helper] 备份和上传完成！\n")
	message.SetString(language.SimplifiedChinese, "  MySQL backup pre-check\n", "  MySQL备份前检查\n")
	message.SetString(language.SimplifiedChinese, "  2020-2020 Alibaba Cloud Inc\n", "  2020-2020 阿里云\n")
	message.SetString(language.English, "Equivalent shell command: %s\n", "Equivalent shell command: %s\n")
	message.SetString(language.SimplifiedChinese, "Equivalent shell command: %s\n", "等价 shell 命令: %s\n")
	message.SetString(language.English, "You can check the backup log file for details: %s\n", "You can check the backup log file for details: %s\n")
	message.SetString(language.SimplifiedChinese, "You can check the backup log file for details: %s\n", "你可以查看本地日志文件获取详细信息: %s\n")

	// AI诊断prompt
	message.SetString(language.SimplifiedChinese, "AI_DIAG_PROMPT", "你是MySQL备份专家。请根据提供的日志错误信息，给出简洁、明确的中文修复建议。输出内容应适合在命令行中展示，避免使用Markdown格式，使用清晰的文本结构。\n\n示例输出格式：\n错误: [错误关键词]\n原因: [简要分析原因]\n修复: [具体修复步骤]")
	message.SetString(language.English, "AI_DIAG_PROMPT", "You are a MySQL backup expert. Based on the provided log error information, give concise and clear repair suggestions in English. The output should be suitable for display in the command line, avoid using Markdown format, and use a clear text structure.\n\nSample output format:\nERROR: [Error keyword]\nCAUSE: [Brief analysis of the cause]\nFIX: [Specific repair steps]")
	message.SetString(language.English, "AI diagnosis failed: %v\n", "AI diagnosis failed: %v\n")
	message.SetString(language.SimplifiedChinese, "AI diagnosis failed: %v\n", "AI诊断失败: %v\n")
	message.SetString(language.English, "AI diagnosis suggestion:\n", "AI diagnosis suggestion:\n")
	message.SetString(language.SimplifiedChinese, "AI diagnosis suggestion:\n", "AI诊断建议:\n")
	message.SetString(language.English, "Would you like to use AI diagnosis? (y/n): ", "Would you like to use AI diagnosis? (y/n): ")
	message.SetString(language.SimplifiedChinese, "Would you like to use AI diagnosis? (y/n): ", "是否使用AI诊断？(y/n): ")

	message.SetString(language.English, "Qwen API Key is required for AI diagnosis. Please set it in config.\n", "Qwen API Key is required for AI diagnosis. Please set it in config.\n")
	message.SetString(language.SimplifiedChinese, "Qwen API Key is required for AI diagnosis. Please set it in config.\n", "AI诊断需要 Qwen API Key，请在配置文件中设置。\n")

	message.SetString(language.English, "AI diagnosis on backup failure: on/off. If not set, prompt interactively.", "AI diagnosis on backup failure: on/off. If not set, prompt interactively.")
	message.SetString(language.SimplifiedChinese, "AI diagnosis on backup failure: on/off. If not set, prompt interactively.", "备份失败时AI诊断：on/off。不设置则交互式询问。")

	message.SetString(language.English, "Backup failed (no 'completed OK!').\n", "Backup failed (no 'completed OK!').\n")
	message.SetString(language.SimplifiedChinese, "Backup failed (no 'completed OK!').\n", "备份失败（未检测到 'completed OK!'）。\n")
}
