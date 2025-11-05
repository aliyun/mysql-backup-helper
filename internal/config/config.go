package config

import (
	"encoding/json"
	"os"
)

// MySQLVersion represents MySQL version
type MySQLVersion struct {
	Major int
	Minor int
	Micro int
}

// Config holds all configuration for the application
type Config struct {
	// OSS configuration
	Endpoint        string `json:"endpoint"`
	AccessKeyId     string `json:"accessKeyId"`
	AccessKeySecret string `json:"accessKeySecret"`
	SecurityToken   string `json:"securityToken"`
	BucketName      string `json:"bucketName"`
	ObjectName      string `json:"objectName"`

	// Upload configuration
	Size    int   `json:"size"`    // Buffer size for multipart upload
	Buffer  int   `json:"buffer"`  // Buffer count
	Traffic int64 `json:"traffic"` // Bandwidth limit in bytes/second (deprecated: use IOLimit instead)

	// MySQL configuration
	MysqlHost     string       `json:"mysqlHost"`
	MysqlPort     int          `json:"mysqlPort"`
	MysqlUser     string       `json:"mysqlUser"`
	MysqlPassword string       `json:"mysqlPassword"`
	MysqlVersion  MySQLVersion `json:"mysqlVersion"`

	// Compression configuration
	Compress     bool   `json:"compress"`
	CompressType string `json:"compressType"` // "zstd" or "qp" or empty

	// Mode configuration
	Mode       string `json:"mode"`       // "oss" or "stream"
	StreamPort int    `json:"streamPort"` // TCP port for streaming

	// Stream authentication
	EnableAuth bool   `json:"enableAuth"`
	AuthKey    string `json:"authKey"`

	// Backup file configuration
	ExistedBackup string `json:"existedBackup"` // Path to existing backup file

	// Log configuration
	LogDir string `json:"logDir"` // Log directory path

	// Performance configuration
	IOLimit int64 `json:"ioLimit"` // IO bandwidth limit in bytes/second

	// Download configuration
	DownloadOutput string `json:"downloadOutput"` // Output path for download mode

	// AI diagnosis configuration
	QwenAPIKey string `json:"qwenAPIKey"` // Qwen API key for AI diagnosis

	// Runtime flags (not from config file)
	Quiet   bool `json:"-"` // Quiet mode (minimal output)
	Verbose bool `json:"-"` // Verbose mode (detailed output)
}

// LoadConfig loads configuration from a JSON file
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}

// SetDefaults sets default values for configuration fields
func (c *Config) SetDefaults() {
	if c.Size == 0 {
		c.Size = 1024 * 1024 * 100 // 100MB
	}
	if c.Buffer == 0 {
		c.Buffer = 10
	}

	// Priority: IOLimit > Traffic, for backward compatibility
	if c.IOLimit > 0 {
		c.Traffic = c.IOLimit
	} else if c.Traffic == 0 {
		c.Traffic = 209715200 // 200MB/s default
	}

	// Note: StreamPort 0 means auto-find available port, don't set default to 9999
	if c.MysqlPort == 0 {
		c.MysqlPort = 3306
	}
	if c.LogDir == "" {
		c.LogDir = "/var/log/mysql-backup-helper"
	}
}
