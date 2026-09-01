//go:build e2e

package e2e

import (
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/manprint/tgsend/internal/testutil"
)

func TestTelegramSendFromEnvironment(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(42))
	result := run(t, []string{"-m", "Hello"}, nil, sendEnvironment(fake), 5*time.Second)
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.ExitCode, result.Stderr)
	}
	var envelope struct {
		OK     bool `json:"ok"`
		Result struct {
			DryRun      bool    `json:"dry_run"`
			ChunksTotal int     `json:"chunks_total"`
			ChunksSent  int     `json:"chunks_sent"`
			MessageIDs  []int64 `json:"message_ids"`
			Chunks      []any   `json:"chunks"`
		} `json:"result"`
	}
	if err := testutil.DecodeOneJSON(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode success: %v", err)
	}
	if !envelope.OK || envelope.Result.DryRun || envelope.Result.ChunksTotal != 1 || envelope.Result.ChunksSent != 1 || !reflect.DeepEqual(envelope.Result.MessageIDs, []int64{42}) || envelope.Result.Chunks != nil {
		t.Fatalf("success envelope = %#v", envelope)
	}
	requests, paths := fake.Requests()
	if len(requests) != 1 || len(paths) != 1 || paths[0] != "/bot123:ABC/sendMessage" || requests[0].ChatID != "-100123" || requests[0].Text != "Hello" {
		t.Fatalf("fake Telegram requests/paths = %#v/%#v", requests, paths)
	}
}

func TestTelegramSendMultiChunkIDsAreOrdered(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(101), telegramSuccess(202))
	body := strings.Repeat("x", 5000)
	result := run(t, []string{"-m", body}, nil, sendEnvironment(fake), 5*time.Second)
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.ExitCode, result.Stderr)
	}
	var envelope struct {
		Result struct {
			ChunksTotal int     `json:"chunks_total"`
			ChunksSent  int     `json:"chunks_sent"`
			MessageIDs  []int64 `json:"message_ids"`
			Chunks      []any   `json:"chunks"`
		} `json:"result"`
	}
	if err := testutil.DecodeOneJSON(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode success: %v", err)
	}
	if envelope.Result.ChunksTotal != 2 || envelope.Result.ChunksSent != 2 || !reflect.DeepEqual(envelope.Result.MessageIDs, []int64{101, 202}) || envelope.Result.Chunks != nil {
		t.Fatalf("result = %#v", envelope.Result)
	}
	requests, _ := fake.Requests()
	if len(requests) != 2 || requests[0].Text != body[:4096] || requests[1].Text != body[4096:] {
		t.Fatalf("ordered requests = %#v", requests)
	}
}

func TestTelegramSendStopsWithPartialProgress(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(1), telegramRejection(400))
	result := run(t, []string{"-m", strings.Repeat("x", 5000)}, nil, sendEnvironment(fake), 5*time.Second)
	assertE2EError(t, result, 5, "telegram_rejected")
	var envelope struct {
		Error struct {
			Progress struct {
				ChunksTotal int `json:"chunks_total"`
				ChunksSent  int `json:"chunks_sent"`
				FailedChunk int `json:"failed_chunk"`
			} `json:"progress"`
		} `json:"error"`
	}
	if err := testutil.DecodeOneJSON(result.Stderr, &envelope); err != nil {
		t.Fatalf("decode partial error: %v", err)
	}
	if envelope.Error.Progress.ChunksTotal != 2 || envelope.Error.Progress.ChunksSent != 1 || envelope.Error.Progress.FailedChunk != 2 {
		t.Fatalf("progress = %#v", envelope.Error.Progress)
	}
	requests, _ := fake.Requests()
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
}

func TestTE2E09TelegramSendReportsEveryFailurePosition(t *testing.T) {
	cases := []struct {
		name       string
		responses  []scriptedTelegramResponse
		wantSent   int
		wantFailed int
		wantCalls  int
	}{
		{name: "first", responses: []scriptedTelegramResponse{telegramRejection(400)}, wantSent: 0, wantFailed: 1, wantCalls: 1},
		{name: "middle", responses: []scriptedTelegramResponse{telegramSuccess(901), telegramRejection(400)}, wantSent: 1, wantFailed: 2, wantCalls: 2},
		{name: "final", responses: []scriptedTelegramResponse{telegramSuccess(901), telegramSuccess(902), telegramRejection(400)}, wantSent: 2, wantFailed: 3, wantCalls: 3},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := newFakeTelegramServer(t, testCase.responses...)
			result := run(t, []string{"-m", strings.Repeat("x", 9000)}, nil, sendEnvironment(fake), 5*time.Second)
			assertE2EError(t, result, 5, "telegram_rejected")
			var envelope struct {
				Error struct {
					Progress struct {
						ChunksTotal int `json:"chunks_total"`
						ChunksSent  int `json:"chunks_sent"`
						FailedChunk int `json:"failed_chunk"`
					} `json:"progress"`
				} `json:"error"`
			}
			if err := testutil.DecodeOneJSON(result.Stderr, &envelope); err != nil {
				t.Fatalf("decode failure progress: %v", err)
			}
			progress := envelope.Error.Progress
			if progress.ChunksTotal != 3 || progress.ChunksSent != testCase.wantSent || progress.FailedChunk != testCase.wantFailed {
				t.Fatalf("progress = %#v", progress)
			}
			requests, _ := fake.Requests()
			if len(requests) != testCase.wantCalls {
				t.Fatalf("requests = %d, want %d", len(requests), testCase.wantCalls)
			}
		})
	}
}

func TestReferenceScenarioSendsExactUnicodePlan(t *testing.T) {
	fake := newFakeTelegramServer(t, telegramSuccess(701), telegramSuccess(702))
	body := strings.Repeat("🚀", 2500)
	const title = "😀"
	const header = "⚠️ WARNING\n😀\n\n"
	result := run(t, []string{"-m", body, "--title", title, "--type", "warning", "--monospace"}, nil, sendEnvironment(fake), 5*time.Second)
	if result.ExitCode != 0 || len(result.Stderr) != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.ExitCode, result.Stderr)
	}
	var envelope struct {
		Result struct {
			ChunksTotal int     `json:"chunks_total"`
			ChunksSent  int     `json:"chunks_sent"`
			MessageIDs  []int64 `json:"message_ids"`
			Chunks      []any   `json:"chunks"`
		} `json:"result"`
	}
	if err := testutil.DecodeOneJSON(result.Stdout, &envelope); err != nil {
		t.Fatalf("decode reference success: %v", err)
	}
	if envelope.Result.ChunksTotal != 2 || envelope.Result.ChunksSent != 2 || !reflect.DeepEqual(envelope.Result.MessageIDs, []int64{701, 702}) || envelope.Result.Chunks != nil {
		t.Fatalf("reference result = %#v", envelope.Result)
	}
	requests, _ := fake.Requests()
	if len(requests) != 2 {
		t.Fatalf("requests = %d, want 2", len(requests))
	}
	var reconstructed strings.Builder
	for index, request := range requests {
		if units := utf16Units(request.Text); units > 4096 {
			t.Fatalf("chunk %d has %d UTF-16 units", index+1, units)
		}
		if index == 0 {
			if !strings.HasPrefix(request.Text, header) {
				t.Fatalf("first request has header %q", request.Text[:min(len(request.Text), len(header))])
			}
			reconstructed.WriteString(strings.TrimPrefix(request.Text, header))
			assertReferenceEntities(t, request, utf16Units(header), utf16Units(strings.TrimPrefix(request.Text, header)))
		} else {
			if strings.Contains(request.Text, "WARNING") || strings.Contains(request.Text, title) {
				t.Fatalf("later request contains first-only header: %q", request.Text[:min(len(request.Text), 30)])
			}
			reconstructed.WriteString(request.Text)
			assertReferenceEntities(t, request, 0, utf16Units(request.Text))
		}
	}
	if reconstructed.String() != body {
		t.Fatal("reference requests do not reconstruct the exact body")
	}
}

func sendEnvironment(fake *fakeTelegramServer) map[string]string {
	return map[string]string{
		"TGSEND_TOKEN":        "123:ABC",
		"TGSEND_CHAT_ID":      "-100123",
		"TGSEND_API_BASE_URL": fake.URL(),
	}
}

func assertReferenceEntities(t *testing.T, request telegramRequest, bodyOffset, bodyLength int) {
	t.Helper()
	want := []telegramEntity{{Type: "pre", Offset: bodyOffset, Length: bodyLength}}
	if bodyOffset != 0 {
		want = []telegramEntity{
			{Type: "bold", Offset: utf16Units("⚠️ WARNING\n"), Length: utf16Units("😀")},
			{Type: "pre", Offset: bodyOffset, Length: bodyLength},
		}
	}
	if !reflect.DeepEqual(request.Entities, want) {
		t.Fatalf("entities = %#v, want %#v", request.Entities, want)
	}
}

func utf16Units(value string) int {
	return len(utf16.Encode([]rune(value)))
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
