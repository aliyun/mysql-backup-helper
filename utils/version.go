package utils

import (
	"os"
	"strings"
)

// 编译时注入的变量
var (
	BuildVersion = "unknown"
	BuildTime    = "unknown"
	GitCommit    = "unknown"
)

// AppVersion 结构体存储应用版本信息
type AppVersion struct {
	Version   string
	BuildTime string
	GitCommit string
}

// GetVersion 获取版本号，优先使用编译时注入的版本
func GetVersion() string {
	if BuildVersion != "unknown" {
		return BuildVersion
	}

	// 回退到从 VERSION 文件读取
	data, err := os.ReadFile("VERSION")
	if err != nil {
		return "0.0.0"
	}
	return strings.TrimSpace(string(data))
}

// GetVersionInfo 获取完整的版本信息
func GetVersionInfo() AppVersion {
	return AppVersion{
		Version:   GetVersion(),
		BuildTime: BuildTime,
		GitCommit: GitCommit,
	}
}

// PrintVersion 打印版本信息
func PrintVersion() {
	version := GetVersion()
	println("MySQL Backup Helper v" + version)
}
