//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/manprint/tgsend/internal/testutil"
)

func TestInputExactWhitespaceAndNewline(t *testing.T) {
	result := run(t, []string{"--dry-run"}, []byte("  first\r\nsecond\n"), nil, 5*time.Second)
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.ExitCode, result.Stderr)
	}
	var envelope struct {
		Result struct {
			Chunks []struct {
				Text string `json:"text"`
			} `json:"chunks"`
		} `json:"result"`
	}
	if err := testutil.DecodeOneJSON(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if len(envelope.Result.Chunks) != 1 || envelope.Result.Chunks[0].Text != "  first\r\nsecond\n" {
		t.Fatalf("preview chunks = %#v", envelope.Result.Chunks)
	}
	if bytes.Count(result.Stdout, []byte("\n")) != 1 {
		t.Fatalf("dry-run JSON cardinality = %q", result.Stdout)
	}
}

func TestInputMessageStdinConflict(t *testing.T) {
	result := run(t, []string{"--dry-run", "-m", "flag"}, []byte("pipe"), nil, 5*time.Second)
	assertE2EError(t, result, 2, "conflicting_input")
}

func TestInputEmptyPipe(t *testing.T) {
	result := run(t, []string{"--dry-run"}, nil, nil, 5*time.Second)
	assertE2EError(t, result, 4, "input_empty")
}

func TestInputLimitRejection(t *testing.T) {
	result := run(t, []string{"--dry-run", "--max-input-bytes", "3"}, []byte("four"), nil, 5*time.Second)
	assertE2EError(t, result, 4, "input_too_large")
}

func TestDryRunJSONIsOfflineAndCredentialFree(t *testing.T) {
	const token = "123456:sentinel-token"
	const chatID = "-100987654"
	result := run(t, []string{"--dry-run", "-m", "Hello", "-c", "/explicitly-missing-config"}, nil, map[string]string{
		"TGSEND_TOKEN":   token,
		"TGSEND_CHAT_ID": chatID,
	}, 5*time.Second)
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.ExitCode, result.Stderr)
	}
	if bytes.Contains(result.Stdout, []byte(token)) || bytes.Contains(result.Stdout, []byte(chatID)) {
		t.Fatal("dry-run output contains credentials")
	}
	var envelope map[string]json.RawMessage
	if err := testutil.DecodeOneJSON(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode dry-run: %v", err)
	}
	if _, ok := envelope["result"]; !ok {
		t.Fatal("dry-run output has no result")
	}
	if strings.Contains(string(result.Stdout), "sending is not available") {
		t.Fatal("dry-run took non-dry transport path")
	}
}

func assertE2EError(t *testing.T, result processResult, exitCode int, errorCode string) {
	t.Helper()
	if result.ExitCode != exitCode || len(result.Stdout) != 0 {
		t.Fatalf("exit/stdout/stderr = %d/%q/%q", result.ExitCode, result.Stdout, result.Stderr)
	}
	var envelope struct {
		OK    bool `json:"ok"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := testutil.DecodeOneJSON(result.Stderr, &envelope); err != nil {
		t.Fatalf("decode error: %v; raw=%q", err, result.Stderr)
	}
	if envelope.OK || envelope.Error.Code != errorCode {
		t.Fatalf("error envelope = %#v", envelope)
	}
}
