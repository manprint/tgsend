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

func TestVersionFromCompiledBinary(t *testing.T) {
	result := run(t, []string{"--version"}, nil, nil, 5*time.Second)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if len(result.Stderr) != 0 {
		t.Fatalf("stderr = %q, want empty", result.Stderr)
	}
	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		OK            bool   `json:"ok"`
		Command       string `json:"command"`
		Result        struct {
			Version string `json:"version"`
		} `json:"result"`
	}
	if err := testutil.DecodeOneJSON(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode version response: %v; raw=%q", err, result.Stdout)
	}
	if envelope.SchemaVersion != "1" || !envelope.OK || envelope.Command != "version" || envelope.Result.Version != "dev" {
		t.Fatalf("unexpected version envelope: %#v", envelope)
	}
	if bytes.Count(result.Stdout, []byte("\n")) != 1 {
		t.Fatalf("version response has unexpected newlines: %q", result.Stdout)
	}
}

func TestHelpFromCompiledBinary(t *testing.T) {
	result := run(t, []string{"--help"}, nil, nil, 5*time.Second)
	if result.ExitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(string(result.Stdout), "Usage:") {
		t.Fatalf("help = %q, want textual usage", result.Stdout)
	}
	if len(result.Stderr) != 0 {
		t.Fatalf("stderr = %q, want empty", result.Stderr)
	}
}

func TestTE2E10VersionAndHelpBypassInputConfigAndNetwork(t *testing.T) {
	const token = "123:version-help-sentinel"
	for _, args := range [][]string{{"--version"}, {"--help"}} {
		name := args[0]
		t.Run(name, func(t *testing.T) {
			result := run(t, args, nil, map[string]string{
				"TGSEND_API_BASE_URL": "https://external.invalid",
				"TGSEND_TOKEN":        token,
			}, 5*time.Second)
			if result.ExitCode != 0 || len(result.Stderr) != 0 || bytes.Contains(result.Stdout, []byte(token)) {
				t.Fatalf("%s result = exit %d/stdout %q/stderr %q", name, result.ExitCode, result.Stdout, result.Stderr)
			}
			if name == "--help" && !strings.Contains(string(result.Stdout), "Usage:") {
				t.Fatalf("help output = %q", result.Stdout)
			}
		})
	}
}

func TestUnknownFlagIsSingleJSONErrorFromCompiledBinary(t *testing.T) {
	const token = "123456:secret-token"
	result := run(t, []string{"--unknown"}, nil, map[string]string{"TGSEND_TOKEN": token}, 5*time.Second)
	if result.ExitCode != 2 {
		t.Fatalf("exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
	if len(result.Stdout) != 0 {
		t.Fatalf("stdout = %q, want empty", result.Stdout)
	}
	if bytes.Contains(result.Stderr, []byte(token)) {
		t.Fatal("stderr contains a token-like environment value")
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := testutil.DecodeOneJSON(result.Stderr, &envelope); err != nil {
		t.Fatalf("decode error response: %v; raw=%q", err, result.Stderr)
	}
	if envelope.Error.Code != "invalid_flag" {
		t.Fatalf("error code = %q, want invalid_flag", envelope.Error.Code)
	}
	if strings.Contains(string(result.Stderr), "Usage:") {
		t.Fatal("stderr contains Cobra usage")
	}
	var raw map[string]any
	if err := json.Unmarshal(result.Stderr, &raw); err != nil {
		t.Fatalf("error response is not JSON: %v", err)
	}
}
