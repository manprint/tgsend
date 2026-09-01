//go:build e2e

package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type compiledSendEnvelope struct {
	OK     bool `json:"ok"`
	Result struct {
		ChunksTotal int     `json:"chunks_total"`
		ChunksSent  int     `json:"chunks_sent"`
		MessageIDs  []int64 `json:"message_ids"`
		Chunks      []any   `json:"chunks"`
	} `json:"result"`
}

func TestTE2E01DefaultConfigAndStdin(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(801))
	home := t.TempDir()
	writeConfig(t, home, ".tgsend", "123:default", "-100001")
	result := runWithHome(t, nil, []byte("stdin message"), map[string]string{"TGSEND_API_BASE_URL": fake.URL()}, home, 5*time.Second)
	assertSuccessfulSend(t, result)
	requests, paths := fake.Requests()
	if len(requests) != 1 || requests[0].ChatID != "-100001" || requests[0].Text != "stdin message" || paths[0] != "/bot123:default/sendMessage" {
		t.Fatalf("default config request/path = %#v/%#v", requests, paths)
	}
}

func TestTE2E02ExplicitConfigAndMessageFlag(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(802))
	home := t.TempDir()
	configPath := writeConfig(t, home, "explicit.toml", "123:explicit", "@explicit_room")
	result := runWithHome(t, []string{"--config", configPath, "-m", "flag message"}, nil, map[string]string{"TGSEND_API_BASE_URL": fake.URL()}, home, 5*time.Second)
	assertSuccessfulSend(t, result)
	requests, _ := fake.Requests()
	if len(requests) != 1 || requests[0].ChatID != "@explicit_room" || requests[0].Text != "flag message" {
		t.Fatalf("explicit config request = %#v", requests)
	}
}

func TestTE2E03EnvironmentPrecedence(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(803))
	home := t.TempDir()
	writeConfig(t, home, ".tgsend", "123:file-token", "-100file")
	const token = "123:environment-token"
	result := runWithHome(t, []string{"-m", "precedence"}, nil, map[string]string{
		"TGSEND_API_BASE_URL": fake.URL(),
		"TGSEND_TOKEN":        token,
		"TGSEND_CHAT_ID":      "@environment_room",
	}, home, 5*time.Second)
	assertSuccessfulSend(t, result)
	if bytes.Contains(result.Stdout, []byte(token)) || bytes.Contains(result.Stderr, []byte(token)) {
		t.Fatal("environment token leaked into process output")
	}
	requests, paths := fake.Requests()
	if len(requests) != 1 || requests[0].ChatID != "@environment_room" || paths[0] != "/bot"+token+"/sendMessage" {
		t.Fatalf("environment precedence request/path = %#v/%#v", requests, paths)
	}
}

func TestTE2E04SilentFlag(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(804))
	result := run(t, []string{"-m", "silent", "--silent"}, nil, sendEnvironment(fake), 5*time.Second)
	assertSuccessfulSend(t, result)
	requests, _ := fake.Requests()
	if len(requests) != 1 || !requests[0].DisableNotification {
		t.Fatalf("silent request = %#v", requests)
	}
}

func TestTE2E06ExitCategoriesTwoThroughSeven(t *testing.T) {
	cases := []struct {
		name      string
		exitCode  int
		errorCode string
		args      []string
		stdin     []byte
		responses []scriptedTelegramResponse
	}{
		{name: "usage", exitCode: 2, errorCode: "invalid_flag", args: []string{"--unknown"}},
		{name: "config", exitCode: 3, errorCode: "config_incomplete", args: []string{"-m", "message"}},
		{name: "input", exitCode: 4, errorCode: "input_empty", args: []string{"--dry-run"}},
		{name: "telegram", exitCode: 5, errorCode: "telegram_rejected", args: []string{"-m", "message"}, responses: []scriptedTelegramResponse{telegramRejection(400)}},
		{name: "transport", exitCode: 6, errorCode: "telegram_transport", args: []string{"-m", "message"}, responses: []scriptedTelegramResponse{{Status: 500, Body: `{"ok":false,"error_code":500}`}}},
		{name: "rate-limit", exitCode: 7, errorCode: "telegram_rate_limited", args: []string{"-m", "message"}, responses: []scriptedTelegramResponse{{Status: 429, Body: `{"ok":false,"error_code":429,"parameters":{"retry_after":0}}`}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var overrides map[string]string
			if testCase.responses != nil {
				fake := newFakeTelegramServer(t, testCase.responses...)
				overrides = sendEnvironment(fake)
				overrides["TGSEND_TOKEN"] = "123:" + strings.ReplaceAll(testCase.name, "-", "_")
			}
			result := run(t, testCase.args, testCase.stdin, overrides, 5*time.Second)
			assertE2EError(t, result, testCase.exitCode, testCase.errorCode)
			if bytes.Count(result.Stderr, []byte("\n")) != 1 {
				t.Fatalf("stderr JSON cardinality = %q", result.Stderr)
			}
		})
	}
}

func writeConfig(t *testing.T, home, name, token, chatID string) string {
	t.Helper()
	path := filepath.Join(home, name)
	contents := fmt.Sprintf("token = %q\nchat_id = %q\n", token, chatID)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config %q: %v", path, err)
	}
	return path
}

func assertSuccessfulSend(t *testing.T, result processResult) compiledSendEnvelope {
	t.Helper()
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.ExitCode, result.Stderr)
	}
	if bytes.Count(result.Stdout, []byte("\n")) != 1 {
		t.Fatalf("success JSON cardinality = %q", result.Stdout)
	}
	var envelope compiledSendEnvelope
	if err := json.Unmarshal(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode success: %v; raw=%q", err, result.Stdout)
	}
	if !envelope.OK || envelope.Result.ChunksTotal != 1 || envelope.Result.ChunksSent != 1 || len(envelope.Result.Chunks) != 0 {
		t.Fatalf("success envelope = %#v", envelope)
	}
	return envelope
}
