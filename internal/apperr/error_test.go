package apperr

import (
	"errors"
	"testing"
)

func TestExitCodeByKind(t *testing.T) {
	tests := []struct {
		name string
		kind Kind
		want int
	}{
		{name: "usage", kind: KindUsage, want: 2},
		{name: "config", kind: KindConfig, want: 3},
		{name: "input", kind: KindInput, want: 4},
		{name: "telegram", kind: KindTelegram, want: 5},
		{name: "transport", kind: KindTransport, want: 6},
		{name: "rate limit", kind: KindRateLimit, want: 7},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExitCode(New(tt.kind, CodeInvalidArguments, "safe", nil)); got != tt.want {
				t.Fatalf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}

	if got := ExitCode(errors.New("unknown")); got != 1 {
		t.Fatalf("ExitCode(unknown) = %d, want 1", got)
	}
	if got := ExitCode(nil); got != 1 {
		t.Fatalf("ExitCode(nil) = %d, want 1", got)
	}
}

func TestErrorUnwrapsCause(t *testing.T) {
	cause := errors.New("private cause")
	got := New(KindInput, CodeInputEmpty, "safe message", cause)
	if !errors.Is(got, cause) {
		t.Fatal("Error does not unwrap its cause")
	}
}

func TestErrorStringUsesSafeMessage(t *testing.T) {
	const token = "123456:secret-token"
	got := New(KindTransport, CodeTelegramTransport, "request failed", errors.New(token))
	if got.Error() != "request failed" {
		t.Fatalf("Error() = %q, want safe message", got.Error())
	}
	if errors.Is(got, errors.New(token)) {
		t.Fatal("unrelated error unexpectedly matched")
	}
	if contains := got.Error(); contains == token {
		t.Fatal("safe message contains the sentinel token")
	}
}

func TestProgressUsesOneBasedFailedChunk(t *testing.T) {
	valid := Progress{ChunksTotal: 3, ChunksSent: 2, FailedChunk: 3}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid progress rejected: %v", err)
	}

	invalid := []Progress{
		{ChunksTotal: -1},
		{ChunksTotal: 3, ChunksSent: -1},
		{ChunksTotal: 3, ChunksSent: 4},
		{ChunksTotal: 3, FailedChunk: -1},
		{ChunksTotal: 3, FailedChunk: 4},
	}
	for _, progress := range invalid {
		if err := progress.Validate(); err == nil {
			t.Errorf("Progress%+v unexpectedly accepted", progress)
		}
	}
	if _, err := NewWithProgress(KindTelegram, CodeTelegramRejected, "safe", nil, invalid[4]); err == nil {
		t.Fatal("NewWithProgress accepted an invalid failed chunk")
	}
}
