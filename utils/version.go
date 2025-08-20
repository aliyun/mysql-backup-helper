package utils

import (
	"os"
	"strings"
)

// AppVersion 结构体存储应用版本信息
type AppVersion struct {
	Version   string
	BuildTime string
	GitCommit string
}

// GetVersion 从 VERSION 文件读取版本号
func GetVersion() string {
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
		BuildTime: "unknown", // 可以通过编译时注入
		GitCommit: "unknown", // 可以通过编译时注入
	}
}

// PrintVersion 打印版本信息
func PrintVersion() {
	version := GetVersion()
	println("MySQL Backup Helper v" + version)
}
