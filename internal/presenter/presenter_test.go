package presenter

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/buildinfo"
)

//go:embed testdata/*.json
var goldenFS embed.FS

func TestVersionEnvelope(t *testing.T) {
	var got bytes.Buffer
	if err := WriteVersion(&got, buildinfo.Info{Version: "dev", Commit: "none", Date: "unknown"}); err != nil {
		t.Fatalf("WriteVersion() error = %v", err)
	}
	if !bytes.HasSuffix(got.Bytes(), []byte("\n")) {
		t.Fatal("version response does not end with one newline")
	}
	if bytes.Count(got.Bytes(), []byte("\n")) != 1 {
		t.Fatalf("version response has more than one newline: %q", got.String())
	}

	var envelope struct {
		SchemaVersion string `json:"schema_version"`
		OK            bool   `json:"ok"`
		Command       string `json:"command"`
		Result        struct {
			Version string `json:"version"`
			Commit  string `json:"commit"`
			Date    string `json:"date"`
		} `json:"result"`
	}
	if err := json.Unmarshal(got.Bytes(), &envelope); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if envelope.SchemaVersion != "1" || !envelope.OK || envelope.Command != "version" || envelope.Result.Version != "dev" || envelope.Result.Commit != "none" || envelope.Result.Date != "unknown" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}

func TestVersionLinkMetadata(t *testing.T) {
	var got bytes.Buffer
	info := buildinfo.Info{Version: "1.2.3", Commit: "abc", Date: "2026-09-01"}
	if err := WriteVersion(&got, info); err != nil {
		t.Fatalf("WriteVersion() error = %v", err)
	}
	if !strings.Contains(got.String(), `"version":"1.2.3"`) || !strings.Contains(got.String(), `"commit":"abc"`) || !strings.Contains(got.String(), `"date":"2026-09-01"`) {
		t.Fatalf("version metadata missing from %q", got.String())
	}
}

func TestPresenterNeverSerializesCause(t *testing.T) {
	const token = "123456:secret-token"
	var stdout, stderr bytes.Buffer
	err := apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "request failed", errors.New(token))
	if err := WriteError(&stderr, "send", ErrorBodyFrom(err)); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	if err := WriteVersion(&stdout, buildinfo.Current()); err != nil {
		t.Fatalf("WriteVersion() error = %v", err)
	}
	if strings.Contains(stdout.String()+stderr.String(), token) {
		t.Fatal("presenter serialized a cause token")
	}
}

func TestSendSuccessGolden(t *testing.T) {
	var got bytes.Buffer
	if err := WriteSend(&got, SendResult{ChunksTotal: 2, ChunksSent: 2, MessageIDs: []int64{101, 102}}); err != nil {
		t.Fatalf("WriteSend() error = %v", err)
	}
	assertGolden(t, "testdata/send_success.json", got.Bytes())
}

func TestDryRunGolden(t *testing.T) {
	var got bytes.Buffer
	if err := WriteSend(&got, SendResult{
		DryRun:      true,
		ChunksTotal: 2,
		MessageIDs:  []int64{},
		Chunks: []PreviewChunk{
			{Index: 1, Text: "Hello", Entities: []Entity{{Type: "bold", Offset: 0, Length: 5}}, DisableNotification: true},
			{Index: 2, Text: "world", Entities: []Entity{}, DisableNotification: true},
		},
	}); err != nil {
		t.Fatalf("WriteSend() error = %v", err)
	}
	assertGolden(t, "testdata/send_dry_run.json", got.Bytes())
}

func TestSendErrorWithoutProgressGolden(t *testing.T) {
	var got bytes.Buffer
	if err := WriteError(&got, "send", ErrorBody{Code: "telegram_rejected", Message: "Telegram rejected the request"}); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	assertGolden(t, "testdata/send_error.json", got.Bytes())
}

func TestSendErrorWithProgressGolden(t *testing.T) {
	var got bytes.Buffer
	if err := WriteError(&got, "send", ErrorBody{
		Code:      "telegram_transport",
		Message:   "Telegram request failed",
		Retryable: false,
		Progress:  &ProgressBody{ChunksTotal: 3, ChunksSent: 1, FailedChunk: 2},
	}); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	want := `{"schema_version":"1","ok":false,"command":"send","error":{"code":"telegram_transport","message":"Telegram request failed","retryable":false,"progress":{"chunks_total":3,"chunks_sent":1,"failed_chunk":2}}}
`
	if got.String() != want {
		t.Fatalf("error JSON = %q, want %q", got.String(), want)
	}
}

func TestArraysAreNeverNull(t *testing.T) {
	var got bytes.Buffer
	if err := WriteSend(&got, SendResult{}); err != nil {
		t.Fatalf("WriteSend() error = %v", err)
	}
	if strings.Contains(got.String(), `"message_ids":null`) {
		t.Fatal("message_ids serialized as null")
	}
	var preview bytes.Buffer
	if err := WriteSend(&preview, SendResult{DryRun: true, Chunks: []PreviewChunk{{Index: 1, Text: "x"}}}); err != nil {
		t.Fatalf("WriteSend() preview error = %v", err)
	}
	if strings.Contains(preview.String(), `"entities":null`) {
		t.Fatal("entities serialized as null")
	}
}

func TestRealSuccessOmitsMessageBodies(t *testing.T) {
	var got bytes.Buffer
	if err := WriteSend(&got, SendResult{ChunksTotal: 1, ChunksSent: 1, MessageIDs: []int64{42}}); err != nil {
		t.Fatalf("WriteSend() error = %v", err)
	}
	if strings.Contains(got.String(), `"chunks"`) || strings.Contains(got.String(), `"text"`) {
		t.Fatalf("real response contains message body: %s", got.String())
	}
}

func TestPreviewOmitsCredentials(t *testing.T) {
	const token = "123456:sentinel-token"
	const chatID = "-100987654"
	var got bytes.Buffer
	if err := WriteSend(&got, SendResult{DryRun: true, Chunks: []PreviewChunk{{Index: 1, Text: "preview"}}}); err != nil {
		t.Fatalf("WriteSend() error = %v", err)
	}
	if strings.Contains(got.String(), token) || strings.Contains(got.String(), chatID) {
		t.Fatalf("preview contains credentials: %s", got.String())
	}
}

func TestExactlyOneResultOrError(t *testing.T) {
	var success bytes.Buffer
	if err := WriteSend(&success, SendResult{MessageIDs: []int64{1}}); err != nil {
		t.Fatalf("WriteSend() error = %v", err)
	}
	var successEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(success.Bytes(), &successEnvelope); err != nil {
		t.Fatalf("success JSON error = %v", err)
	}
	if _, result := successEnvelope["result"]; !result {
		t.Fatal("success has no result")
	}
	if _, errBody := successEnvelope["error"]; errBody {
		t.Fatal("success has an error")
	}

	var failure bytes.Buffer
	if err := WriteError(&failure, "send", ErrorBody{Code: "input_empty", Message: "input is empty"}); err != nil {
		t.Fatalf("WriteError() error = %v", err)
	}
	var failureEnvelope map[string]json.RawMessage
	if err := json.Unmarshal(failure.Bytes(), &failureEnvelope); err != nil {
		t.Fatalf("error JSON error = %v", err)
	}
	if _, result := failureEnvelope["result"]; result {
		t.Fatal("failure has a result")
	}
	if _, errBody := failureEnvelope["error"]; !errBody {
		t.Fatal("failure has no error")
	}
}

func TestAllKnownErrorCodesSerialize(t *testing.T) {
	codes := []string{
		"invalid_arguments", "conflicting_input", "invalid_flag", "config_not_found", "config_unreadable", "config_invalid", "config_incomplete",
		"input_empty", "input_unreadable", "input_too_large", "input_invalid_utf8", "telegram_rejected", "telegram_transport", "telegram_protocol", "telegram_rate_limited",
	}
	for _, code := range codes {
		var got bytes.Buffer
		if err := WriteError(&got, "send", ErrorBody{Code: code, Message: "safe message"}); err != nil {
			t.Fatalf("WriteError(%q) error = %v", code, err)
		}
		var envelope Envelope
		if err := json.Unmarshal(got.Bytes(), &envelope); err != nil {
			t.Fatalf("code %q JSON error = %v", code, err)
		}
		if envelope.Error == nil || envelope.Error.Code != code {
			t.Fatalf("code %q not serialized: %#v", code, envelope.Error)
		}
	}
}

func assertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	want, err := goldenFS.ReadFile(name)
	if err != nil {
		t.Fatalf("read golden %q: %v", name, err)
	}
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if !bytes.Equal(got, want) {
		t.Fatalf("golden %q mismatch:\n got %s want %s", name, got, want)
	}
	if !bytes.HasSuffix(got, []byte("\n")) || bytes.Count(got, []byte("\n")) != 1 {
		t.Fatalf("golden %q does not contain exactly one trailing newline", name)
	}
	var decoded any
	if err := json.Unmarshal(got, &decoded); err != nil {
		t.Fatalf("golden %q is invalid JSON: %v", name, err)
	}
	if !reflect.DeepEqual(decoded, mustDecodeJSON(t, want)) {
		t.Fatalf("golden %q semantic mismatch", name)
	}
}

func mustDecodeJSON(t *testing.T, value []byte) any {
	t.Helper()
	var decoded any
	if err := json.Unmarshal(value, &decoded); err != nil {
		t.Fatalf("invalid fixture JSON: %v", err)
	}
	return decoded
}
