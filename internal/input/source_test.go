package input

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/manprint/tgsend/internal/apperr"
)

const testLimit int64 = 1024

func TestReadMessageExact(t *testing.T) {
	want := " \t leading\ntrailing \t\n"
	got, err := Read(Source{Message: want, MessageSet: true, StdinIsTerminal: true, MaxBytes: testLimit})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != want {
		t.Fatalf("Read() = %q, want %q", got, want)
	}
}

func TestReadStdinExact(t *testing.T) {
	want := " \t leading\ntrailing \t\n"
	got, err := Read(Source{Stdin: strings.NewReader(want), MaxBytes: testLimit})
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != want {
		t.Fatalf("Read() = %q, want %q", got, want)
	}
}

func TestReadConflictOnSingleWhitespaceByte(t *testing.T) {
	_, err := Read(Source{Message: "message", MessageSet: true, Stdin: strings.NewReader(" "), MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeConflictingInput)
}

func TestReadMessageDoesNotReadTerminal(t *testing.T) {
	reader := failingReader{err: errors.New("stdin should not be touched")}
	got, err := Read(Source{Message: "message", MessageSet: true, Stdin: reader, StdinIsTerminal: true, MaxBytes: testLimit})
	if err != nil || got != "message" {
		t.Fatalf("terminal message read = %q, %v", got, err)
	}
}

func TestReadRejectsEmptyMessage(t *testing.T) {
	_, err := Read(Source{MessageSet: true, StdinIsTerminal: true, MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeInputEmpty)
}

func TestReadRejectsEmptyPipe(t *testing.T) {
	_, err := Read(Source{Stdin: strings.NewReader(""), MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeInputEmpty)
}

func TestReadRejectsTerminal(t *testing.T) {
	_, err := Read(Source{StdinIsTerminal: true, MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeInputUnreadable)
}

func TestReadRejectsInvalidUTF8(t *testing.T) {
	_, err := Read(Source{Stdin: bytes.NewReader([]byte{0xff}), MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeInputInvalidUTF8)
}

func TestReadRejectsLimitPlusOne(t *testing.T) {
	_, err := Read(Source{Stdin: strings.NewReader("12345"), MaxBytes: 4})
	assertCode(t, err, apperr.CodeInputTooLarge)
}

func TestReadAcceptsExactlyLimit(t *testing.T) {
	want := "1234"
	got, err := Read(Source{Stdin: strings.NewReader(want), MaxBytes: int64(len(want))})
	if err != nil || got != want {
		t.Fatalf("Read() = %q, %v; want %q", got, err, want)
	}
}

func TestReadRejectsNonPositiveLimit(t *testing.T) {
	for _, limit := range []int64{0, -1} {
		_, err := Read(Source{Message: "message", MessageSet: true, StdinIsTerminal: true, MaxBytes: limit})
		assertCode(t, err, apperr.CodeInvalidFlag)
	}
}

func TestReadClassifiesReaderFailure(t *testing.T) {
	_, err := Read(Source{Stdin: failingReader{err: errors.New("private reader failure")}, MaxBytes: testLimit})
	assertCode(t, err, apperr.CodeInputUnreadable)
}

func FuzzReadPreservesValidUTF8WithinLimit(f *testing.F) {
	f.Add("hello\n")
	f.Add("🙂 exact")
	f.Fuzz(func(t *testing.T, input string) {
		if !utf8.ValidString(input) || len(input) == 0 || len(input) > 256 {
			t.Skip()
		}
		got, err := Read(Source{Stdin: strings.NewReader(input), MaxBytes: int64(len(input))})
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if got != input {
			t.Fatalf("Read() = %q, want %q", got, input)
		}
	})
}

type failingReader struct{ err error }

func (r failingReader) Read([]byte) (int, error) { return 0, r.err }

func assertCode(t *testing.T, err error, want apperr.Code) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want %q", want)
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) {
		t.Fatalf("error %T does not unwrap to apperr.Error: %v", err, err)
	}
	if appErr.Code != want {
		t.Fatalf("error code = %q, want %q", appErr.Code, want)
	}
}

var _ io.Reader = failingReader{}
