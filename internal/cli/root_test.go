package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/manprint/tgsend/internal/buildinfo"
)

func TestVersionEnvelope(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), Dependencies{
		Stdout:    &stdout,
		Stderr:    &stderr,
		BuildInfo: buildinfo.Info{Version: "1.2.3", Commit: "abc", Date: "today"},
	}, []string{"--version"})
	if code != 0 {
		t.Fatalf("Execute() = %d, want 0", code)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var envelope map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("stdout is not JSON: %v", err)
	}
	if envelope["schema_version"] != "1" || envelope["ok"] != true || envelope["command"] != "version" {
		t.Fatalf("unexpected version envelope: %#v", envelope)
	}
	if !bytes.HasSuffix(stdout.Bytes(), []byte("\n")) || bytes.Count(stdout.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("stdout newline contract failed: %q", stdout.String())
	}
}

func TestVersionLinkMetadata(t *testing.T) {
	var stdout, stderr bytes.Buffer
	Execute(context.Background(), Dependencies{
		Stdout:    &stdout,
		Stderr:    &stderr,
		BuildInfo: buildinfo.Info{Version: "linked", Commit: "commit", Date: "date"},
	}, []string{"--version"})
	if !strings.Contains(stdout.String(), `"version":"linked"`) || !strings.Contains(stdout.String(), `"commit":"commit"`) || !strings.Contains(stdout.String(), `"date":"date"`) {
		t.Fatalf("linked metadata missing from %q", stdout.String())
	}
}

func TestHelpIsText(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), Dependencies{Stdout: &stdout, Stderr: &stderr}, []string{"--help"})
	if code != 0 {
		t.Fatalf("Execute() = %d, want 0", code)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("help = %q, want textual usage", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestUnknownFlagIsSingleJSONError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), Dependencies{Stdout: &stdout, Stderr: &stderr}, []string{"--unknown"})
	if code != 2 {
		t.Fatalf("Execute() = %d, want 2", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if bytes.Count(stderr.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("stderr is not one JSON document: %q", stderr.String())
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not JSON: %v", err)
	}
	if envelope.Error.Code != "invalid_flag" {
		t.Fatalf("error code = %q, want invalid_flag", envelope.Error.Code)
	}
	if strings.Contains(stderr.String(), "Usage:") {
		t.Fatal("stderr contains Cobra usage")
	}
}

func TestPositionalArgumentRejected(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Execute(context.Background(), Dependencies{Stdout: &stdout, Stderr: &stderr}, []string{"unexpected"})
	if code != 2 {
		t.Fatalf("Execute() = %d, want 2", code)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &envelope); err != nil {
		t.Fatalf("stderr is not JSON: %v", err)
	}
	if envelope.Error.Code != "invalid_arguments" {
		t.Fatalf("error code = %q, want invalid_arguments", envelope.Error.Code)
	}
}

func TestUnknownErrorClassificationIsSafe(t *testing.T) {
	const token = "123456:secret-token"
	err := classifyCobraError(errors.New(token))
	if strings.Contains(err.Error(), token) {
		t.Fatal("classified error exposed its cause")
	}
}
