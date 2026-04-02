package transfer

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestCountingWriterCountsBytes(t *testing.T) {
	var sink bytes.Buffer
	w := NewCountingWriter(&sink)

	n, err := w.Write([]byte("abc"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != 3 {
		t.Fatalf("unexpected write count: got %d want %d", n, 3)
	}
	if got := w.BytesWritten(); got != 3 {
		t.Fatalf("unexpected bytes written: got %d want %d", got, 3)
	}
	if got := w.CRC32(); got != 0x352441C2 {
		t.Fatalf("unexpected crc32: got %#x want %#x", got, 0x352441C2)
	}
	if got := sink.String(); got != "abc" {
		t.Fatalf("unexpected sink contents: got %q want %q", got, "abc")
	}
}

func TestCountingReaderCountsBytes(t *testing.T) {
	src := bytes.NewBufferString("abcdef")
	r := NewCountingReader(src)

	buf := make([]byte, 4)
	n, err := r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected read error: %v", err)
	}
	if n != 4 {
		t.Fatalf("unexpected read count: got %d want %d", n, 4)
	}
	if got := r.BytesRead(); got != 4 {
		t.Fatalf("unexpected bytes read: got %d want %d", got, 4)
	}

	n, err = r.Read(buf)
	if err != nil && !errors.Is(err, io.EOF) {
		t.Fatalf("unexpected read error: %v", err)
	}
	if n != 2 {
		t.Fatalf("unexpected read count: got %d want %d", n, 2)
	}
	if got := r.BytesRead(); got != 6 {
		t.Fatalf("unexpected bytes read: got %d want %d", got, 6)
	}
}
