package utils

import (
	"encoding/json"
	"os"
)

type Config struct {
	Endpoint        string  `json:"endpoint"`
	AccessKeyId     string  `json:"accessKeyId"`
	AccessKeySecret string  `json:"accessKeySecret"`
	SecurityToken   string  `json:"securityToken"`
	BucketName      string  `json:"bucketName"`
	ObjectName      string  `json:"objectName"`
	Size            int     `json:"size"`
	Buffer          int     `json:"buffer"`
	MysqlHost       string  `json:"mysqlHost"`
	MysqlPort       int     `json:"mysqlPort"`
	MysqlUser       string  `json:"mysqlUser"`
	MysqlPassword   string  `json:"mysqlPassword"`
	Compress        bool    `json:"compress"`
	CompressType    string  `json:"compressType"`
	Mode            string  `json:"mode"`
	StreamPort      int     `json:"streamPort"`
	StreamHost      string  `json:"streamHost"`
	MysqlVersion    Version `json:"mysqlVersion"`
	QwenAPIKey      string  `json:"qwenAPIKey"`
	EnableHandshake bool    `json:"enableHandshake"`
	StreamKey       string  `json:"streamKey"`
	ExistedBackup   string  `json:"existedBackup"`
	LogDir          string  `json:"logDir"`
	EstimatedSize   int64   `json:"estimatedSize"`
	IOLimit         int64   `json:"ioLimit"`
	DownloadOutput  string  `json:"downloadOutput"`
	RemoteOutput    string  `json:"remoteOutput"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	err = json.Unmarshal(data, &cfg)
	return &cfg, err
}

func (c *Config) SetDefaults() {
	if c.Size == 0 {
		c.Size = 1024 * 1024 * 100 // 100MB
	}
	if c.Buffer == 0 {
		c.Buffer = 10
	}
	// Note: StreamPort 0 means auto-find available port, don't set default to 9999
	if c.MysqlPort == 0 {
		c.MysqlPort = 3306
	}
	if c.LogDir == "" {
		c.LogDir = "/var/log/mysql-backup-helper"
	}
}
