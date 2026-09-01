//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

type previewEnvelope struct {
	OK     bool `json:"ok"`
	Result struct {
		Chunks []previewChunk `json:"chunks"`
	} `json:"result"`
}

type previewChunk struct {
	Index               int             `json:"index"`
	Text                string          `json:"text"`
	Entities            []previewEntity `json:"entities"`
	DisableNotification bool            `json:"disable_notification"`
}

type previewEntity struct {
	Type   string `json:"type"`
	Offset int    `json:"offset"`
	Length int    `json:"length"`
}

func TestMessageNewlinePreferredSplit(t *testing.T) {
	body := strings.Repeat("a", 3000) + "\n" + strings.Repeat("b", 2000)
	result := run(t, []string{"--dry-run", "-m", body}, nil, nil, 5*time.Second)
	preview := decodePreview(t, result)
	if len(preview.Result.Chunks) != 2 {
		t.Fatalf("chunks = %d; want 2", len(preview.Result.Chunks))
	}
	if preview.Result.Chunks[0].Text != strings.Repeat("a", 3000)+"\n" || preview.Result.Chunks[1].Text != strings.Repeat("b", 2000) {
		t.Fatalf("newline-preferred chunks have lengths %d/%d", len(preview.Result.Chunks[0].Text), len(preview.Result.Chunks[1].Text))
	}
}

func TestMessageAstralFallbackSplit(t *testing.T) {
	body := strings.Repeat("😀", 3000)
	result := run(t, []string{"--dry-run", "-m", body}, nil, nil, 5*time.Second)
	preview := decodePreview(t, result)
	if len(preview.Result.Chunks) != 2 {
		t.Fatalf("chunks = %d; want 2", len(preview.Result.Chunks))
	}
	var reconstructed strings.Builder
	for index, chunk := range preview.Result.Chunks {
		if units := len(utf16.Encode([]rune(chunk.Text))); units > 4096 {
			t.Fatalf("chunk %d uses %d UTF-16 units", index, units)
		}
		if !utf8.ValidString(chunk.Text) {
			t.Fatalf("chunk %d is invalid UTF-8", index)
		}
		reconstructed.WriteString(chunk.Text)
	}
	if reconstructed.String() != body {
		t.Fatal("astral chunks do not reconstruct the input")
	}
}

func TestMessagePreviewHasNoGeneratedChunkLabels(t *testing.T) {
	body := strings.Repeat("message\n", 800)
	result := run(t, []string{"--dry-run", "-m", body}, nil, nil, 5*time.Second)
	preview := decodePreview(t, result)
	for index, chunk := range preview.Result.Chunks {
		if strings.Contains(chunk.Text, "chunk ") {
			t.Fatalf("chunk %d contains a generated chunk label", index)
		}
	}
}

func TestMessageTitlePreview(t *testing.T) {
	result := run(t, []string{"--dry-run", "-m", "body", "--title", "Deploy"}, nil, nil, 5*time.Second)
	preview := decodePreview(t, result)
	if len(preview.Result.Chunks) != 1 || preview.Result.Chunks[0].Text != "Deploy\n\nbody" {
		t.Fatalf("preview = %#v; want title-only composition", preview.Result.Chunks)
	}
	want := previewEntity{Type: "bold", Offset: 0, Length: 6}
	if len(preview.Result.Chunks[0].Entities) != 1 || preview.Result.Chunks[0].Entities[0] != want {
		t.Fatalf("entities = %#v; want %#v", preview.Result.Chunks[0].Entities, want)
	}
}

func TestMessageSeverityCodePoints(t *testing.T) {
	cases := map[string]string{
		"info":     "ℹ️ INFO",
		"warning":  "⚠️ WARNING",
		"error":    "❌ ERROR",
		"critical": "🚨 CRITICAL",
	}
	for input, wantLine := range cases {
		t.Run(input, func(t *testing.T) {
			result := run(t, []string{"--dry-run", "-m", "body", "--type", input}, nil, nil, 5*time.Second)
			preview := decodePreview(t, result)
			if got := strings.Split(preview.Result.Chunks[0].Text, "\n\n")[0]; got != wantLine {
				t.Fatalf("header = %q; want %q", got, wantLine)
			}
		})
	}
}

func TestMessageMonospaceUTF16Offsets(t *testing.T) {
	result := run(t, []string{"--dry-run", "-m", "🚀 body", "--title", "😀", "--type", "CRITICAL", "--monospace"}, nil, nil, 5*time.Second)
	preview := decodePreview(t, result)
	if len(preview.Result.Chunks) != 1 {
		t.Fatalf("chunks = %d; want 1", len(preview.Result.Chunks))
	}
	wantText := "🚨 CRITICAL\n😀\n\n🚀 body"
	if preview.Result.Chunks[0].Text != wantText {
		t.Fatalf("text = %q; want %q", preview.Result.Chunks[0].Text, wantText)
	}
	want := []previewEntity{
		{Type: "bold", Offset: 12, Length: 2},
		{Type: "pre", Offset: 16, Length: 7},
	}
	if len(preview.Result.Chunks[0].Entities) != len(want) {
		t.Fatalf("entities = %#v; want %#v", preview.Result.Chunks[0].Entities, want)
	}
	for index := range want {
		if preview.Result.Chunks[0].Entities[index] != want[index] {
			t.Fatalf("entity %d = %#v; want %#v", index, preview.Result.Chunks[0].Entities[index], want[index])
		}
	}
}

func TestMessageHeaderOnlyFirstChunk(t *testing.T) {
	result := run(t, []string{"--dry-run", "-m", strings.Repeat("x", 5000), "--title", "Deploy", "--type", "WARNING"}, nil, nil, 5*time.Second)
	preview := decodePreview(t, result)
	if len(preview.Result.Chunks) < 2 {
		t.Fatalf("chunks = %d; want multiple", len(preview.Result.Chunks))
	}
	if !strings.HasPrefix(preview.Result.Chunks[0].Text, "⚠️ WARNING\nDeploy\n\n") {
		t.Fatalf("first chunk has no header: %q", preview.Result.Chunks[0].Text[:30])
	}
	for index, chunk := range preview.Result.Chunks[1:] {
		if strings.Contains(chunk.Text, "⚠️ WARNING\nDeploy\n\n") {
			t.Fatalf("chunk %d contains the first-chunk header", index+2)
		}
	}
}

func TestMessageInvalidFormattingRejectedBeforeConfig(t *testing.T) {
	tests := []struct {
		name string
		args []string
		code string
	}{
		{name: "unknown type", args: []string{"--dry-run", "-m", "body", "--type", "NOTICE", "-c", "/missing/config"}, code: "invalid_flag"},
		{name: "title too long", args: []string{"--dry-run", "-m", "body", "--title", strings.Repeat("t", 4093), "-c", "/missing/config"}, code: "title_too_long"},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			result := run(t, testCase.args, nil, nil, 5*time.Second)
			assertE2EError(t, result, 2, testCase.code)
		})
	}
}

func decodePreview(t *testing.T, result processResult) previewEnvelope {
	t.Helper()
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.ExitCode, result.Stderr)
	}
	var envelope previewEnvelope
	if err := json.Unmarshal(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode preview: %v; raw=%q", err, result.Stdout)
	}
	if !envelope.OK {
		t.Fatalf("preview envelope is not successful: %q", result.Stdout)
	}
	return envelope
}
