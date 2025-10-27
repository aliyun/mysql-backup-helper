package utils

import (
	"bufio"
	"bytes"
	"io"
	"sync"

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
			part, err := uploadPart(bucket, imur, data, index, cfg.Traffic)
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
