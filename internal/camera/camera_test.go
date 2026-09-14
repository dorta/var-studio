//go:build linux

package camera

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"
)

func TestReadFirstChunk(t *testing.T) {
	want := []byte("--frame\r\n")
	got, err := readFirstChunk(
		context.Background(),
		bytes.NewReader(want),
		time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("readFirstChunk() = %q, want %q", got, want)
	}
}

func TestReadFirstChunkEOF(t *testing.T) {
	_, err := readFirstChunk(
		context.Background(),
		bytes.NewReader(nil),
		time.Second,
	)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("readFirstChunk() error = %v, want EOF", err)
	}
}

func TestReadFirstChunkTimeout(t *testing.T) {
	reader, writer := io.Pipe()
	defer reader.Close()
	defer writer.Close()
	_, err := readFirstChunk(
		context.Background(),
		reader,
		10*time.Millisecond,
	)
	if !errors.Is(err, errFirstFrameTimeout) {
		t.Fatalf(
			"readFirstChunk() error = %v, want first-frame timeout",
			err,
		)
	}
}
