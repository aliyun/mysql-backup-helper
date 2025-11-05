package oss

import (
	"backup-helper/internal/config"
	"backup-helper/internal/pkg/progress"
	"bufio"
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/gioco-play/easy-i18n/i18n"
)

// Uploader implements storage.Uploader for Alibaba Cloud OSS
type Uploader struct {
	cfg *config.Config
}

// NewUploader creates a new OSS uploader
func NewUploader(cfg *config.Config) *Uploader {
	return &Uploader{cfg: cfg}
}

// Upload uploads data to OSS using multipart upload
func (u *Uploader) Upload(ctx context.Context, reader io.Reader, objectName string, totalSize int64) error {
	var waitSender sync.WaitGroup

	// Create progress tracker
	tracker := progress.NewTracker(totalSize)
	defer tracker.Complete()

	client, err := oss.New(u.cfg.Endpoint, u.cfg.AccessKeyId, u.cfg.AccessKeySecret)
	if err != nil {
		return err
	}
	bucket, err := client.Bucket(u.cfg.BucketName)
	if err != nil {
		return err
	}

	storageType := oss.ObjectStorageClass(oss.StorageStandard)
	imur, err := bucket.InitiateMultipartUpload(objectName, storageType)
	if err != nil {
		return err
	}

	bufferSize := u.cfg.Size
	if bufferSize == 0 {
		bufferSize = 1024 * 1024 * 100 // 100MB
	}

	// Wrap reader with progress tracker
	progressReader := progress.NewReader(reader, tracker, bufferSize)
	bufReader := bufio.NewReaderSize(progressReader, bufferSize)

	var parts []oss.UploadPart
	index := 1
	for {
		p := make([]byte, bufferSize)
		n, err := io.ReadFull(bufReader, p)
		if n > 0 {
			data := p[:n]
			waitSender.Add(1)
			part, err := uploadPart(bucket, imur, data, index, u.cfg.Traffic)
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

// uploadPart uploads a single part to OSS
func uploadPart(bucket *oss.Bucket, imur oss.InitiateMultipartUploadResult, data []byte, index int, traffic int64) (oss.UploadPart, error) {
	reader := bytes.NewReader(data)
	// If traffic is 0, don't apply rate limiting (unlimited)
	if traffic > 0 {
		part, err := bucket.UploadPart(imur, reader, int64(len(data)), index, oss.Progress(&ProgressListener{}), oss.TrafficLimitHeader(traffic))
		if err != nil {
			return oss.UploadPart{}, err
		}
		return part, nil
	} else {
		// No rate limiting
		part, err := bucket.UploadPart(imur, reader, int64(len(data)), index, oss.Progress(&ProgressListener{}))
		if err != nil {
			return oss.UploadPart{}, err
		}
		return part, nil
	}
}

// ProgressListener implements OSS progress listener
type ProgressListener struct{}

// ProgressChanged is called when upload progress changes
func (listener *ProgressListener) ProgressChanged(event *oss.ProgressEvent) {
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

// DeleteObject deletes the specified OSS object
func (u *Uploader) DeleteObject(objectName string) error {
	client, err := oss.New(u.cfg.Endpoint, u.cfg.AccessKeyId, u.cfg.AccessKeySecret)
	if err != nil {
		return err
	}
	bucket, err := client.Bucket(u.cfg.BucketName)
	if err != nil {
		return err
	}
	return bucket.DeleteObject(objectName)
}
