package utils

import (
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// initEn will init en support.
func InitEn(tag language.Tag) {
	message.SetString(tag, "\\t%s\\t%s\\t%s\\t%s", "\\t%s\\t%s\\t%s\\t%s")
	message.SetString(tag, "\\t您需要通过物理备份迁移到云上的数据库小版本较高，云上MySQL可能无法兼容该版本的数据文件，可在MySQL全量备份上云帮助文档页面确认", "\\tYour MySQL Server version may newer than version that provided On Alibaba Cloud, data file probably incompatible, read doc online for more info.")
	message.SetString(tag, "不支持多参数", "not support multiple parameters")
	message.SetString(tag, "参数", "Parameter")
	message.SetString(tag, "参数未设置", "parameter not set")
	message.SetString(tag, "可能无法兼容", "maybe incompatible")
	message.SetString(tag, "备份命令参考(Percona XtraBackup):", "Backup Command Example(Percona XtraBackup):")
	message.SetString(tag, "备份相关参数完毕...", "Finish Checking Backup Parameters...")
	message.SetString(tag, "复制参数检查完毕", "Finish Checking Replicate Parameters")
	message.SetString(tag, "建议参数: %s", "Parameter Suggestion: %s")
	message.SetString(tag, "建议参数: ibdata1", "Parameter Suggestion: ibdata1")
	message.SetString(tag, "检查MySQL版本...", "Checking MySQL Server Version...")
	message.SetString(tag, "检查备份相关参数...", "Checking Backup Parameters...")
	message.SetString(tag, "检查复制参数中(以下参数影响主备复制, 并不影响备份)...", "Checking Replicate Parameters(Following parameter not hinder backup, but hinder apply binlog)...")
	message.SetString(tag, "版本", "Version")
	message.SetString(tag, "获取参数中...", "Get parameter for checking...")
	message.SetString(tag, "解析文件:", "Parsing file:")
	message.SetString(tag, "请输入数据库密码: ", "Please input mysql-server password: ")
	message.SetString(tag, "连接数据库host=%s port=%d user=%s", "connect to mysql-server host=%s port=%d user=%s")
}
