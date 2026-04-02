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

	n, err := w.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != 5 {
		t.Fatalf("unexpected write count: got %d want %d", n, 5)
	}
	if got := w.BytesWritten(); got != 5 {
		t.Fatalf("unexpected bytes written: got %d want %d", got, 5)
	}

	n, err = w.Write([]byte(" world"))
	if err != nil {
		t.Fatalf("unexpected write error: %v", err)
	}
	if n != 6 {
		t.Fatalf("unexpected write count: got %d want %d", n, 6)
	}
	if got := w.BytesWritten(); got != 11 {
		t.Fatalf("unexpected bytes written: got %d want %d", got, 11)
	}
	if got := w.CRC32(); got == 0 {
		t.Fatal("expected crc32 to be updated")
	}
	if got := sink.String(); got != "hello world" {
		t.Fatalf("unexpected sink contents: got %q want %q", got, "hello world")
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
