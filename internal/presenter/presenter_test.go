package presenter

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/buildinfo"
)

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
