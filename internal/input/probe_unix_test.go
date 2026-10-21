//go:build unix

package input

import (
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/manprint/tgsend/internal/apperr"
)

func TestReadMessageWithIdleOpenPipeDoesNotBlock(t *testing.T) {
	reader, writer := pipe(t)
	defer func() { _ = writer.Close() }()

	done := make(chan error, 1)
	go func() {
		_, err := Read(Source{Message: "message", MessageSet: true, Stdin: reader, MaxBytes: testLimit})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Read() blocked on an idle stdin pipe")
	}
}

func TestReadMessageConflictLeavesPipeDataUnread(t *testing.T) {
	reader, writer := pipe(t)
	if _, err := writer.Write([]byte("piped")); err != nil {
		t.Fatalf("write pipe: %v", err)
	}
	_ = writer.Close()

	_, err := Read(Source{Message: "message", MessageSet: true, Stdin: reader, MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeConflictingInput)
	rest, readErr := io.ReadAll(reader)
	if readErr != nil || string(rest) != "piped" {
		t.Fatalf("pipe data after probe = %q, %v; want it unread", rest, readErr)
	}
}

func TestReadMessageDetectsPipeDataArrivingWithinWait(t *testing.T) {
	reader, writer := pipe(t)
	go func() {
		time.Sleep(stdinProbeWait / 5)
		_, _ = writer.Write([]byte("late"))
		_ = writer.Close()
	}()
	_, err := Read(Source{Message: "message", MessageSet: true, Stdin: reader, MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeConflictingInput)
}

func TestReadMessageAcceptsClosedEmptyPipe(t *testing.T) {
	reader, writer := pipe(t)
	_ = writer.Close()
	got, err := Read(Source{Message: "message", MessageSet: true, Stdin: reader, MaxBytes: testLimit})
	if err != nil || got != "message" {
		t.Fatalf("Read() = %q, %v; want message", got, err)
	}
}

func TestReadMessageRegularFileStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stdin.txt")
	if err := os.WriteFile(path, []byte("file data"), 0o600); err != nil {
		t.Fatalf("write stdin file: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open stdin file: %v", err)
	}
	defer func() { _ = file.Close() }()

	_, err = Read(Source{Message: "message", MessageSet: true, Stdin: file, MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeConflictingInput)
	rest, readErr := io.ReadAll(file)
	if readErr != nil || string(rest) != "file data" {
		t.Fatalf("file data after probe = %q, %v; want it unread", rest, readErr)
	}

	// At end of file, nothing is left for stdin to contribute.
	got, err := Read(Source{Message: "message", MessageSet: true, Stdin: file, MaxBytes: testLimit})
	if err != nil || got != "message" {
		t.Fatalf("Read() at end of file = %q, %v; want message", got, err)
	}
}

func pipe(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = reader.Close()
		_ = writer.Close()
	})
	return reader, writer
}
