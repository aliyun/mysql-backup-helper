package service

import (
	"backup-helper/internal/config"
	"backup-helper/internal/domain/backup"
	"backup-helper/internal/infra/storage/oss"
	"backup-helper/internal/infra/stream"
	"backup-helper/internal/pkg/format"
	"backup-helper/internal/pkg/ratelimit"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/gioco-play/easy-i18n/i18n"
)

// TransferService handles file transfer operations
type TransferService struct {
	cfg *config.Config
}

// NewTransferService creates a new transfer service
func NewTransferService(cfg *config.Config) *TransferService {
	return &TransferService{cfg: cfg}
}

// SendOptions contains options for sending files
type SendOptions struct {
	SourceFile     string // Path to source file or "-" for stdin
	Mode           string // "oss" or "stream"
	StreamPort     int
	SkipValidation bool
	ValidateOnly   bool
	EnableAuth     bool
	AuthKey        string
}

// Send sends an existing backup file to destination
func (s *TransferService) Send(opts *SendOptions) error {
	i18n.Printf("[backup-helper] Processing existing backup file...\n")

	// 1. Validate backup file
	if !opts.SkipValidation && opts.SourceFile != "-" {
		backupInfo, err := backup.ValidateFile(opts.SourceFile)
		if err != nil {
			return fmt.Errorf("validation error: %v", err)
		}
		backup.PrintValidation(opts.SourceFile, backupInfo)

		if !backupInfo.IsValid {
			return fmt.Errorf("cannot proceed with invalid backup file")
		}

		// If validate-only, exit here
		if opts.ValidateOnly {
			i18n.Printf("[backup-helper] Validation completed successfully.\n")
			return nil
		}
	} else if opts.SourceFile == "-" && !opts.SkipValidation {
		backupInfo, _ := backup.ValidateStdin()
		backup.PrintStdinValidation(backupInfo)
	}

	// 2. Display IO limit
	if s.cfg.Traffic == 0 {
		i18n.Printf("[backup-helper] Rate limiting disabled (unlimited speed)\n")
	} else {
		i18n.Printf("[backup-helper] IO rate limit set to: %s/s\n", format.Bytes(s.cfg.Traffic))
	}

	// 3. Get reader
	var reader io.Reader
	if opts.SourceFile == "-" {
		reader = os.Stdin
		i18n.Printf("[backup-helper] Reading backup data from stdin...\n")
	} else {
		file, err := os.Open(opts.SourceFile)
		if err != nil {
			return fmt.Errorf("open backup file error: %v", err)
		}
		defer file.Close()
		reader = file
		i18n.Printf("[backup-helper] Reading backup data from file: %s\n", opts.SourceFile)
	}

	// 4. Determine object name suffix
	objectSuffix := ".xb"
	if opts.Mode == "stream" {
		s.cfg.Compress = false
		s.cfg.CompressType = ""
	} else if s.cfg.Compress {
		switch s.cfg.CompressType {
		case "zstd":
			objectSuffix = ".xb.zst"
		default:
			objectSuffix = "_qp.xb"
		}
	}
	timestamp := time.Now().Format("_20060102150405")
	fullObjectName := s.cfg.ObjectName + timestamp + objectSuffix

	// 5. Calculate total size (auto-detect from file)
	var totalSize int64
	if opts.SourceFile != "-" {
		totalSize, _ = backup.GetFileSize(opts.SourceFile)
		if totalSize > 0 {
			i18n.Printf("[backup-helper] Backup file size: %s\n", format.Bytes(totalSize))
		}
	} else {
		i18n.Printf("[backup-helper] Uploading from stdin, size unknown\n")
	}

	// 6. Execute transfer
	switch opts.Mode {
	case "oss":
		i18n.Printf("[backup-helper] Uploading existing backup to OSS...\n")
		uploader := oss.NewUploader(s.cfg)
		err := uploader.Upload(nil, reader, fullObjectName, totalSize)
		if err != nil {
			return fmt.Errorf("OSS upload error: %v", err)
		}
		i18n.Printf("[backup-helper] OSS upload completed!\n")
	case "stream":
		equivalentSource := opts.SourceFile
		if opts.SourceFile == "-" {
			equivalentSource = "stdin"
		}
		if opts.StreamPort > 0 {
			i18n.Printf("[backup-helper] Starting TCP stream server on port %d...\n", opts.StreamPort)
			i18n.Printf("[backup-helper] Equivalent command: cat %s | nc -l4 %d\n",
				equivalentSource, opts.StreamPort)
		} else {
			i18n.Printf("[backup-helper] Starting TCP stream server (auto-find available port)...\n")
		}

		sender := stream.NewSender(opts.StreamPort, opts.EnableAuth, opts.AuthKey, totalSize)
		tcpWriter, _, closer, _, _, err := sender.Start()
		if err != nil {
			return fmt.Errorf("stream server error: %v", err)
		}
		defer closer()

		// Apply rate limiting
		writer := tcpWriter
		if s.cfg.Traffic > 0 {
			rateLimitedWriter := ratelimit.NewWriter(tcpWriter, s.cfg.Traffic)
			writer = rateLimitedWriter
		}

		i18n.Printf("[backup-helper] Streaming backup data...\n")
		_, err = io.Copy(writer, reader)
		if err != nil {
			return fmt.Errorf("TCP stream error: %v", err)
		}

		i18n.Printf("[backup-helper] Stream completed!\n")
	default:
		return fmt.Errorf("unknown mode: %s", opts.Mode)
	}

	return nil
}

// ReceiveOptions contains options for receiving files
type ReceiveOptions struct {
	OutputPath string // Output file path or "-" for stdout
	StreamPort int
	EnableAuth bool
	AuthKey    string
}

// Receive receives backup from TCP stream
func (s *TransferService) Receive(opts *ReceiveOptions) error {
	// Display IO limit
	outputStream := os.Stdout
	if opts.OutputPath == "-" {
		outputStream = os.Stderr
	}
	if s.cfg.Traffic == 0 {
		i18n.Fprintf(outputStream, "[backup-helper] Rate limiting disabled (unlimited speed)\n")
	} else {
		i18n.Fprintf(outputStream, "[backup-helper] IO rate limit set to: %s/s\n", format.Bytes(s.cfg.Traffic))
	}

	// Start TCP receiver (size will be determined during transfer)
	receiver := stream.NewReceiver(opts.StreamPort, opts.EnableAuth, opts.AuthKey, 0)
	streamReader, tracker, closer, _, _, err := receiver.Start()
	if err != nil {
		return fmt.Errorf("stream receiver error: %v", err)
	}
	defer closer()

	// Determine output destination
	if opts.OutputPath == "-" {
		// Stream to stdout
		if tracker != nil {
			tracker.SetOutputToStderr(true)
		}
		i18n.Fprintf(os.Stderr, "[backup-helper] Receiving backup data and streaming to stdout...\n")

		var reader io.Reader = streamReader
		if s.cfg.Traffic > 0 {
			rateLimitedReader := ratelimit.NewReader(streamReader, s.cfg.Traffic)
			reader = rateLimitedReader
		}

		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return fmt.Errorf("download error: %v", err)
		}
	} else {
		// Write to file
		i18n.Printf("[backup-helper] Receiving backup data and saving to: %s\n", opts.OutputPath)

		var reader io.Reader = streamReader
		if s.cfg.Traffic > 0 {
			rateLimitedReader := ratelimit.NewReader(streamReader, s.cfg.Traffic)
			reader = rateLimitedReader
		}

		file, err := os.Create(opts.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %v", err)
		}
		defer file.Close()

		_, err = io.Copy(file, reader)
		if err != nil {
			return fmt.Errorf("download error: %v", err)
		}

		i18n.Printf("[backup-helper] Saved to: %s\n", opts.OutputPath)
	}

	return nil
}
