package storage

import (
	"context"
	"io"
)

// Uploader defines the interface for uploading data to storage
type Uploader interface {
	// Upload uploads data from reader to storage with the given object name
	Upload(ctx context.Context, reader io.Reader, objectName string, totalSize int64) error
}

// UploadOptions contains options for upload operations
type UploadOptions struct {
	// TotalSize is the total size of data to upload (0 if unknown)
	TotalSize int64

	// BufferSize is the size of each upload part
	BufferSize int

	// RateLimit is the bandwidth limit in bytes per second (0 for unlimited)
	RateLimit int64
}
