package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode"

	"github.com/gioco-play/easy-i18n/i18n"
)

// Flags represents command line flags (moved from cmd/backup-helper/flags.go to avoid circular dependency)
type Flags struct {
	DoBackup         bool
	DoDownload       bool
	DoPrepare        bool
	DoCheck          bool
	ConfigPath       string
	Host             string
	User             string
	Password         string
	Port             int
	StreamPort       int
	StreamHost       string
	Mode             string
	CompressType     string
	LangFlag         string
	AIDiagnoseFlag   string
	EnableHandshake  bool
	StreamKey        string
	ExistedBackup    string
	DownloadOutput   string
	TargetDir        string
	EstimatedSize    int64
	EstimatedSizeStr string
	IOLimitStr       string
	UseSSH           bool
	RemoteOutput     string
	Parallel         int
	UseMemory        string
	AutoYes          bool
	XtrabackupPath   string
	DefaultsFile     string
	LogFileName      string
	Timeout          int
	ShowVersion      bool
}

// MergeFlags merges command line flags with config file values
// Returns merged config and effective values for flags that need special handling
func MergeFlags(cfg *Config, flags *Flags) (*Config, *EffectiveValues, error) {
	// Fill parameters not specified by command line with config
	if flags.Host == "" {
		flags.Host = cfg.MysqlHost
	}
	if flags.Port == 0 {
		flags.Port = cfg.MysqlPort
	}
	if flags.User == "" {
		flags.User = cfg.MysqlUser
	}
	if flags.Password == "" {
		flags.Password = cfg.MysqlPassword
	}

	// Handle --compress flag
	effectiveCompressType := flags.CompressType
	if effectiveCompressType == "__NOT_SET__" {
		// Flag was not passed, use config or empty
		if cfg.CompressType != "" {
			effectiveCompressType = cfg.CompressType
		} else {
			effectiveCompressType = ""
		}
	} else {
		// Flag was passed
		if effectiveCompressType == "" {
			// --compress was passed but empty value (--compress= or --compress ""), default to qp
			effectiveCompressType = "qp"
		}
	}
	// Normalize: "no" means no compression
	if effectiveCompressType == "no" {
		effectiveCompressType = ""
	}

	if flags.ExistedBackup == "" && cfg.ExistedBackup != "" {
		flags.ExistedBackup = cfg.ExistedBackup
	}

	// Handle --xtrabackup-path flag (command-line flag overrides config)
	if flags.XtrabackupPath != "" {
		cfg.XtrabackupPath = flags.XtrabackupPath
	} else if cfg.XtrabackupPath == "" {
		// If not set in flag or config, check environment variable
		if envPath := os.Getenv("XTRABACKUP_PATH"); envPath != "" {
			cfg.XtrabackupPath = envPath
		}
	}

	// Handle --defaults-file flag (command-line flag overrides config)
	if flags.DefaultsFile != "" {
		cfg.DefaultsFile = flags.DefaultsFile
	}

	// Handle --log-file flag (command-line flag overrides config)
	if flags.LogFileName != "" {
		cfg.LogFileName = flags.LogFileName
	}

	// Parse estimatedSize from command line or config
	var estimatedSize int64
	if flags.EstimatedSizeStr != "" {
		parsedSize, err := ParseSize(flags.EstimatedSizeStr)
		if err != nil {
			return nil, nil, err
		}
		estimatedSize = parsedSize
	} else if flags.EstimatedSize == 0 && cfg.EstimatedSize > 0 {
		estimatedSize = cfg.EstimatedSize
	}

	// Parse ioLimit from command line or config
	if flags.IOLimitStr != "" {
		parsedLimit, err := ParseRateLimit(flags.IOLimitStr)
		if err != nil {
			i18n.Printf("Error parsing --io-limit '%s': %v\n", flags.IOLimitStr, err)
			return nil, nil, err
		}
		cfg.IOLimit = parsedLimit
	}

	// Parse parallel from command line or config
	if flags.Parallel > 0 {
		cfg.Parallel = flags.Parallel
	} else if flags.Parallel == 0 && cfg.Parallel == 0 {
		// Use default (4) if not specified in command line or config
		cfg.Parallel = 4
	}

	// Parse useMemory from command line or config
	if flags.UseMemory != "" {
		cfg.UseMemory = flags.UseMemory
	} else if cfg.UseMemory == "" {
		// Use default (1G) if not specified in command line or config
		cfg.UseMemory = "1G"
	}

	// Parse timeout from command line or config
	if flags.Timeout > 0 {
		cfg.Timeout = flags.Timeout
	} else if cfg.Timeout == 0 {
		// Use default (60) if not specified in command line or config
		cfg.Timeout = 60
	}
	// Enforce maximum timeout: 3600 seconds (1 hour)
	if cfg.Timeout > 3600 {
		cfg.Timeout = 3600
	}

	effective := &EffectiveValues{
		Host:            flags.Host,
		Port:            flags.Port,
		User:            flags.User,
		Password:        flags.Password,
		CompressType:    effectiveCompressType,
		ExistedBackup:   flags.ExistedBackup,
		EstimatedSize:   estimatedSize,
		StreamHost:      flags.StreamHost,
		StreamPort:      flags.StreamPort,
		EnableHandshake: flags.EnableHandshake,
		StreamKey:       flags.StreamKey,
		DownloadOutput:  flags.DownloadOutput,
		TargetDir:       flags.TargetDir,
		RemoteOutput:    flags.RemoteOutput,
		UseSSH:          flags.UseSSH,
		AutoYes:         flags.AutoYes,
		AIDiagnoseFlag:  flags.AIDiagnoseFlag,
	}

	return cfg, effective, nil
}

// ParseRateLimit parses a rate limit string with units (e.g., "100MB/s", "1GB/s", "500KB/s")
// Returns bytes per second, or -1 for unlimited speed
// Supported units: B/s, KB/s, MB/s, GB/s, TB/s (case insensitive)
// Special value: -1 means unlimited speed
func ParseRateLimit(rateStr string) (int64, error) {
	if rateStr == "" || rateStr == "0" {
		return 0, nil
	}
	if rateStr == "-1" {
		return -1, nil
	}

	rateStr = strings.TrimSpace(rateStr)
	rateStr = strings.ToUpper(rateStr)

	// Remove /s suffix if present
	rateStr = strings.TrimSuffix(rateStr, "/S")
	rateStr = strings.TrimSpace(rateStr)

	// Find where the number ends
	var numEnd int
	for i, r := range rateStr {
		if !unicode.IsDigit(r) && r != '.' {
			numEnd = i
			break
		}
		numEnd = i + 1
	}

	if numEnd == 0 {
		return 0, fmt.Errorf("invalid rate limit format: %s", rateStr)
	}

	// Parse number
	numStr := rateStr[:numEnd]
	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in rate limit: %s", numStr)
	}

	if value <= 0 {
		return 0, nil
	}

	// Parse unit
	unitStr := strings.TrimSpace(rateStr[numEnd:])
	if unitStr == "" {
		// Default to bytes if no unit specified
		return int64(value), nil
	}

	// Map units to multipliers (base 1024)
	var multiplier float64
	switch unitStr {
	case "B", "BYTE", "BYTES":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported unit: %s (supported: B, KB, MB, GB, TB)", unitStr)
	}

	return int64(value * multiplier), nil
}

// ParseSize parses a size string with units (e.g., "100MB", "1GB", "500KB")
// Returns bytes
// Supported units: B, KB, MB, GB, TB (case insensitive)
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" || sizeStr == "0" {
		return 0, nil
	}

	sizeStr = strings.TrimSpace(sizeStr)
	sizeStr = strings.ToUpper(sizeStr)

	// Find where the number ends
	var numEnd int
	for i, r := range sizeStr {
		if !unicode.IsDigit(r) && r != '.' {
			numEnd = i
			break
		}
		numEnd = i + 1
	}

	if numEnd == 0 {
		return 0, fmt.Errorf("invalid size format: %s", sizeStr)
	}

	// Parse number
	numStr := sizeStr[:numEnd]
	value, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number in size: %s", numStr)
	}

	if value <= 0 {
		return 0, nil
	}

	// Parse unit
	unitStr := strings.TrimSpace(sizeStr[numEnd:])
	if unitStr == "" {
		// Default to bytes if no unit specified
		return int64(value), nil
	}

	// Map units to multipliers (base 1024)
	var multiplier float64
	switch unitStr {
	case "B", "BYTE", "BYTES":
		multiplier = 1
	case "KB", "K":
		multiplier = 1024
	case "MB", "M":
		multiplier = 1024 * 1024
	case "GB", "G":
		multiplier = 1024 * 1024 * 1024
	case "TB", "T":
		multiplier = 1024 * 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("unsupported unit: %s (supported: B, KB, MB, GB, TB)", unitStr)
	}

	return int64(value * multiplier), nil
}

// EffectiveValues represents effective values after merging flags and config
type EffectiveValues struct {
	Host            string
	Port            int
	User            string
	Password        string
	CompressType    string
	ExistedBackup   string
	EstimatedSize   int64
	StreamHost      string
	StreamPort      int
	EnableHandshake bool
	StreamKey       string
	DownloadOutput  string
	TargetDir       string
	RemoteOutput    string
	UseSSH          bool
	AutoYes         bool
	AIDiagnoseFlag  string
}
