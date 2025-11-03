package utils

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"sync"
	"time"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gioco-play/easy-i18n/i18n"
)

type OssProgressListener struct{}

func (listener *OssProgressListener) ProgressChanged(event *oss.ProgressEvent) {
	switch event.EventType {
	case oss.TransferStartedEvent:
		i18n.Printf("Transfer Started, ConsumedBytes: %d, TotalBytes %d.\n", event.ConsumedBytes, event.TotalBytes)
	case oss.TransferDataEvent:
		i18n.Printf("\rTransfer Data, ConsumedBytes: %d, TotalBytes %d, %d%%.", event.ConsumedBytes, event.TotalBytes, event.ConsumedBytes*100/event.TotalBytes)
	case oss.TransferCompletedEvent:
		i18n.Printf("\nTransfer Completed, ConsumedBytes: %d, TotalBytes %d.\n", event.ConsumedBytes, event.TotalBytes)
	case oss.TransferFailedEvent:
		i18n.Printf("\nTransfer Failed, ConsumedBytes: %d, TotalBytes %d.\n", event.ConsumedBytes, event.TotalBytes)
	default:
	}
}

// UploadReaderToOSS supports fragmenting upload from io.Reader to OSS, objectName is passed by the caller
func UploadReaderToOSS(cfg *Config, objectName string, reader io.Reader, totalSize int64) error {
	var waitSender sync.WaitGroup

	// Create progress tracker
	tracker := NewProgressTracker(totalSize)
	defer tracker.Complete()

	// Start IO monitoring if auto-limit-rate is enabled
	var ioMonitor *IOMonitor
	var rateLimitedReader *RateLimitedReader
	var monitorCtx context.Context
	var cancelMonitor context.CancelFunc
	if cfg.AutoLimitRate {
		monitorCtx, cancelMonitor = context.WithCancel(context.Background())
		defer cancelMonitor()

		// Create IO monitor with 80% threshold and dynamic rate adjustment
		ioMonitor = NewIOMonitor(80.0, cfg.Traffic, func(stats *IOStats) {
			// This callback is called when high IO is detected, but rate is already adjusted
			// Just log the stats for reference
			i18n.Printf("  Current IO stats - Read: %.1f MB/s, Write: %.1f MB/s\n",
				stats.ReadBW, stats.WriteBW)
		})
		ioMonitor.Start(monitorCtx, 2*time.Second)
		defer ioMonitor.Stop()

		i18n.Printf("[backup-helper] Real-time IO monitoring active (threshold: 80%%, auto-adjusting rate limit)\n")
	}

	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyId, cfg.AccessKeySecret)
	if err != nil {
		return err
	}
	bucket, err := client.Bucket(cfg.BucketName)
	if err != nil {
		return err
	}

	storageType := oss.ObjectStorageClass(oss.StorageStandard)
	imur, err := bucket.InitiateMultipartUpload(objectName, storageType)
	if err != nil {
		return err
	}

	bufferSize := cfg.Size
	if bufferSize == 0 {
		bufferSize = 1024 * 1024 * 100 // 100MB
	}

	// Apply client-side rate limiting if Traffic is set
	if cfg.Traffic > 0 {
		rateLimitedReader = NewRateLimitedReader(reader, cfg.Traffic)
		reader = rateLimitedReader
		i18n.Printf("[backup-helper] OSS upload rate limiting active: %s/s\n", FormatBytes(cfg.Traffic))

		// Update rate limiter if IO monitoring is active
		if cfg.AutoLimitRate && ioMonitor != nil {
			go func() {
				ticker := time.NewTicker(2 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-monitorCtx.Done():
						return
					case <-ticker.C:
						if ioMonitor != nil && rateLimitedReader != nil {
							currentLimit := ioMonitor.GetCurrentLimit()
							rateLimitedReader.UpdateRateLimit(currentLimit)
						}
					}
				}
			}()
		}
	}

	// Wrap reader with progress tracker
	progressReader := NewProgressReader(reader, tracker, bufferSize)
	bufReader := bufio.NewReaderSize(progressReader, bufferSize)

	var parts []oss.UploadPart
	index := 1
	for {
		p := make([]byte, bufferSize)
		n, err := io.ReadFull(bufReader, p)
		if n > 0 {
			data := p[:n]
			waitSender.Add(1)

			// Update rate limiter if IO monitoring is active
			if ioMonitor != nil && rateLimitedReader != nil {
				currentLimit := ioMonitor.GetCurrentLimit()
				rateLimitedReader.UpdateRateLimit(currentLimit)
			}

			// OSS SDK TrafficLimitHeader is for server-side limiting
			// We use client-side rate limiting instead, but still pass it for reference
			trafficLimit := cfg.Traffic
			if ioMonitor != nil {
				trafficLimit = ioMonitor.GetCurrentLimit()
			}

			part, err := uploadPart(bucket, imur, data, index, trafficLimit)
			if err != nil {
				bucket.AbortMultipartUpload(imur)
				return err
			}
			parts = append(parts, part)
			index++
			waitSender.Done()
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			break
		}
		if err != nil {
			bucket.AbortMultipartUpload(imur)
			return err
		}
	}
	waitSender.Wait()
	objectAcl := oss.ObjectACL(oss.ACLPrivate)
	_, err = bucket.CompleteMultipartUpload(imur, parts, objectAcl)
	if err != nil {
		return err
	}
	return nil
}

func uploadPart(bucket *oss.Bucket, imur oss.InitiateMultipartUploadResult, data []byte, index int, traffic int64) (oss.UploadPart, error) {
	reader := bytes.NewReader(data)
	part, err := bucket.UploadPart(imur, reader, int64(len(data)), index, oss.Progress(&OssProgressListener{}), oss.TrafficLimitHeader(traffic))
	if err != nil {
		return oss.UploadPart{}, err
	}
	return part, nil
}

// DeleteOSSObject deletes the specified OSS object, objectName is passed by the caller
func DeleteOSSObject(cfg *Config, objectName string) error {
	client, err := oss.New(cfg.Endpoint, cfg.AccessKeyId, cfg.AccessKeySecret)
	if err != nil {
		return err
	}
	bucket, err := client.Bucket(cfg.BucketName)
	if err != nil {
		return err
	}
	return bucket.DeleteObject(objectName)
}
