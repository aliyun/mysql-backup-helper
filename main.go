package main

import (
	"backup-helper/utils"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/gioco-play/easy-i18n/i18n"
	"golang.org/x/term"
	"golang.org/x/text/language"
)

func main() {
	utils.InitI18nAuto()

	var doBackup bool
	var configPath string
	var host, user, password string
	var port int
	var streamPort int
	var mode string
	var compressType string
	var langFlag string
	var aiDiagnoseFlag string
	var enableHandshake bool
	var streamKey string
	var existedBackup string

	flag.BoolVar(&doBackup, "backup", false, "Run xtrabackup and upload to OSS")
	flag.StringVar(&existedBackup, "existed-backup", "", "Path to existing xtrabackup backup file to upload (use '-' for stdin)")
	flag.StringVar(&configPath, "config", "", "config file path (optional)")
	flag.StringVar(&host, "host", "", "Connect to host")
	flag.IntVar(&port, "port", 0, "Port number to use for connection")
	flag.StringVar(&user, "user", "", "User for login")
	flag.StringVar(&password, "password", "", "Password to use when connecting to server. If password is not given it's asked from the tty.")
	flag.IntVar(&streamPort, "stream-port", 0, "If set, stream backup to this local TCP port for remote pulling")
	flag.StringVar(&mode, "mode", "oss", "Backup mode: oss (upload to OSS) or stream (push to TCP port)")
	flag.StringVar(&compressType, "compress-type", "", "Compress type: qp(qpress)/zstd, priority is higher than config file")
	flag.StringVar(&langFlag, "lang", "", "Language: zh (Chinese) or en (English), auto-detect if unset")
	flag.StringVar(&aiDiagnoseFlag, "ai-diagnose", "", "AI diagnosis on backup failure: on/off. If not set, prompt interactively.")
	flag.BoolVar(&enableHandshake, "enable-handshake", false, "Enable handshake for TCP streaming (default: false, can be set in config)")
	flag.StringVar(&streamKey, "stream-key", "", "Handshake key for TCP streaming (default: empty, can be set in config)")

	flag.Parse()

	// Set language if --lang is specified
	switch langFlag {
	case "cn", "zh":
		i18n.SetLang(language.SimplifiedChinese)
	case "en":
		i18n.SetLang(language.English)
	default:
		// use auto setting
	}

	// Load config
	var cfg *utils.Config
	if configPath != "" {
		var err error
		cfg, err = utils.LoadConfig(configPath)
		if err != nil {
			i18n.Printf("Load config error: %v\n", err)
			os.Exit(1)
		}
		cfg.SetDefaults()
	} else {
		cfg = &utils.Config{}
		cfg.SetDefaults()
	}

	// Fill parameters not specified by command line with config
	if host == "" {
		host = cfg.MysqlHost
	}
	if port == 0 {
		port = cfg.MysqlPort
	}
	if user == "" {
		user = cfg.MysqlUser
	}
	if password == "" {
		password = cfg.MysqlPassword
	}
	if compressType == "" && cfg.CompressType != "" {
		compressType = cfg.CompressType
	}

	if password == "" {
		i18n.Printf("Please input mysql-server password: ")
		pwd, _ := term.ReadPassword(0)
		i18n.Printf("\n")
		password = string(pwd)
	}

	// 3. MySQL param check (always run first)
	i18n.Printf("connect to mysql-server host=%s port=%d user=%s\n", host, port, user)
	outputHeader()
	db := utils.GetConnection(host, port, user, password)
	defer db.Close()
	options := utils.CollectVariableFromMySQLServer(db)
	utils.Check(options, cfg)

	// 4. If --backup, run backup/upload
	if doBackup {
		// Check xtrabackup version (run early)
		mysqlVer := cfg.MysqlVersion
		utils.CheckXtraBackupVersion(mysqlVer)

		i18n.Printf("[backup-helper] Running xtrabackup...\n")
		cfg.MysqlHost = host
		cfg.MysqlPort = port
		cfg.MysqlUser = user
		cfg.MysqlPassword = password

		// 1. Decide objectName suffix and compression param
		ossObjectName := cfg.ObjectName
		objectSuffix := ".xb"
		// compressType default is empty
		if mode == "stream" {
			cfg.Compress = false
			cfg.CompressType = ""
			objectSuffix = ".xb"
		} else if cfg.Compress {
			switch compressType {
			case "zstd":
				objectSuffix = ".xb.zst"
				cfg.CompressType = "zstd"
			default:
				objectSuffix = "_qp.xb"
				cfg.CompressType = ""
			}
		} else {
			objectSuffix = ".xb"
			cfg.CompressType = ""
		}
		timestamp := time.Now().Format("_20060102150405")
		fullObjectName := ossObjectName + timestamp + objectSuffix

		reader, cmd, logFileName, err := utils.RunXtraBackup(cfg)
		if err != nil {
			i18n.Printf("Run xtrabackup error: %v\n", err)
			os.Exit(1)
		}

		switch mode {
		case "oss":
			i18n.Printf("[backup-helper] Uploading to OSS...\n")
			err = utils.UploadReaderToOSS(cfg, fullObjectName, reader)
			if err != nil {
				i18n.Printf("OSS upload error: %v\n", err)
				cmd.Process.Kill()
				os.Exit(1)
			}
		case "stream":
			if streamPort == 0 {
				streamPort = cfg.StreamPort
			}
			// handshake priority：command line > config > default
			if !isFlagPassed("enable-handshake") {
				enableHandshake = cfg.EnableHandshake
			}
			if streamKey == "" {
				streamKey = cfg.StreamKey
			}
			if streamPort == 0 {
				i18n.Printf("You must specify --stream-port when mode=stream\n")
				os.Exit(1)
			}
			tcpWriter, closer, err := utils.StartStreamServer(streamPort, enableHandshake, streamKey)
			if err != nil {
				i18n.Printf("Stream server error: %v\n", err)
				os.Exit(1)
			}
			defer closer()
			_, err = io.Copy(tcpWriter, reader)
			if err != nil {
				i18n.Printf("TCP stream error: %v\n", err)
				cmd.Process.Kill()
				os.Exit(1)
			}
		default:
			i18n.Printf("Unknown mode: %s\n", mode)
			os.Exit(1)
		}

		cmd.Wait()
		// backup log file needs to be closed
		utils.CloseBackupLogFile(cmd)
		// Check backup log
		logContent, err := os.ReadFile(logFileName)
		if err != nil {
			i18n.Printf("Backup log read error.\n")
			os.Exit(1)
		}
		if !strings.Contains(string(logContent), "completed OK!") {
			i18n.Printf("Backup failed (no 'completed OK!').\n")
			i18n.Printf("You can check the backup log file for details: %s\n", logFileName)

			switch aiDiagnoseFlag {
			case "on":
				if cfg.QwenAPIKey == "" {
					i18n.Printf("Qwen API Key is required for AI diagnosis. Please set it in config.\n")
					os.Exit(1)
				}
				aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, string(logContent))
				if err != nil {
					i18n.Printf("AI diagnosis failed: %v\n", err)
				} else {
					fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
					fmt.Println(color.YellowString(aiSuggestion))
				}
			case "off":
				// do nothing, skip ai diagnose
			default:
				var input string
				i18n.Printf("Would you like to use AI diagnosis? (y/n): ")
				fmt.Scanln(&input)
				if input == "y" || input == "Y" || input == "yes" || input == "Yes" {
					aiSuggestion, err := utils.DiagnoseWithAliQwen(cfg, string(logContent))
					if err != nil {
						i18n.Printf("AI diagnosis failed: %v\n", err)
					} else {
						fmt.Print(color.YellowString(i18n.Sprintf("AI diagnosis suggestion:\n")))
						fmt.Println(color.YellowString(aiSuggestion))
					}
				}
			}
			os.Exit(1)
		}
		i18n.Printf("[backup-helper] Backup and upload completed!\n")
		return
	} else if existedBackup != "" {
		// upload existed backup file to OSS or stream via TCP
		i18n.Printf("[backup-helper] Processing existing backup file...\n")

		// Get reader from existing backup file or stdin
		var reader io.Reader
		if existedBackup == "-" {
			// Read from stdin (for cat command)
			reader = os.Stdin
			i18n.Printf("[backup-helper] Reading backup data from stdin...\n")
		} else {
			// Read from file
			file, err := os.Open(existedBackup)
			if err != nil {
				i18n.Printf("Open backup file error: %v\n", err)
				os.Exit(1)
			}
			defer file.Close()
			reader = file
			i18n.Printf("[backup-helper] Reading backup data from file: %s\n", existedBackup)
		}

		// Determine object name suffix based on compression type
		ossObjectName := cfg.ObjectName
		objectSuffix := ".xb"
		if mode == "stream" {
			cfg.Compress = false
			cfg.CompressType = ""
			objectSuffix = ".xb"
		} else if cfg.Compress {
			switch compressType {
			case "zstd":
				objectSuffix = ".xb.zst"
				cfg.CompressType = "zstd"
			default:
				objectSuffix = "_qp.xb"
				cfg.CompressType = ""
			}
		} else {
			objectSuffix = ".xb"
			cfg.CompressType = ""
		}
		timestamp := time.Now().Format("_20060102150405")
		fullObjectName := ossObjectName + timestamp + objectSuffix

		switch mode {
		case "oss":
			i18n.Printf("[backup-helper] Uploading existing backup to OSS...\n")
			err := utils.UploadReaderToOSS(cfg, fullObjectName, reader)
			if err != nil {
				i18n.Printf("OSS upload error: %v\n", err)
				os.Exit(1)
			}
			i18n.Printf("[backup-helper] OSS upload completed!\n")
		case "stream":
			// Set stream port
			if streamPort == 0 {
				streamPort = cfg.StreamPort
			}

			// handshake priority：command line > config > default
			if !isFlagPassed("enable-handshake") {
				enableHandshake = cfg.EnableHandshake
			}
			if streamKey == "" {
				streamKey = cfg.StreamKey
			}

			if streamPort == 0 {
				i18n.Printf("You must specify --stream-port when mode=stream\n")
				os.Exit(1)
			}

			i18n.Printf("[backup-helper] Starting TCP stream server on port %d...\n", streamPort)
			// Show equivalent command
			equivalentSource := existedBackup
			if existedBackup == "-" {
				equivalentSource = "stdin"
			}
			i18n.Printf("[backup-helper] Equivalent command: cat %s | nc -l4 %d\n",
				equivalentSource, streamPort)

			tcpWriter, closer, err := utils.StartStreamServer(streamPort, enableHandshake, streamKey)
			if err != nil {
				i18n.Printf("Stream server error: %v\n", err)
				os.Exit(1)
			}
			defer closer()

			// Stream the backup data
			i18n.Printf("[backup-helper] Streaming backup data...\n")
			_, err = io.Copy(tcpWriter, reader)
			if err != nil {
				i18n.Printf("TCP stream error: %v\n", err)
				os.Exit(1)
			}

			i18n.Printf("[backup-helper] Stream completed!\n")
		default:
			i18n.Printf("Unknown mode: %s\n", mode)
			os.Exit(1)
		}
		return
	}
}

func outputHeader() {
	bar := strings.Repeat("#", 80)
	title := "MySQL Backup Helper"
	subtitle := "Powered by Alibaba Cloud Inc"
	version := "v1.0.0"
	timeStr := time.Now().Format("2006-01-02 15:04:05")

	i18n.Printf("%s\n", bar)
	// 居中显示
	pad := (80 - len(title)) / 2
	if pad < 0 {
		pad = 0
	}
	fmt.Printf("%s%s\n", strings.Repeat(" ", pad), title)
	pad2 := (80 - len(subtitle)) / 2
	if pad2 < 0 {
		pad2 = 0
	}
	fmt.Printf("%s%s\n", strings.Repeat(" ", pad2), subtitle)
	fmt.Printf("%sVersion: %s    Time: %s\n", strings.Repeat(" ", 10), version, timeStr)
	i18n.Printf("%s\n", bar)
}

// 判断命令行参数是否被设置
func isFlagPassed(name string) bool {
	found := false
	flag.CommandLine.Visit(func(f *flag.Flag) {
		if f.Name == name {
			found = true
		}
	})
	return found
}
