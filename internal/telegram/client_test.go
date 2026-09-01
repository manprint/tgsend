package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/message"
)

type doerFunc func(*http.Request) (*http.Response, error)

func (function doerFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func newTestClient(t *testing.T, doer Doer) *Client {
	t.Helper()
	client, err := NewClient(Options{Token: "123:ABC", BaseURL: "https://api.telegram.org", Doer: doer})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func appError(t *testing.T, err error, wantKind apperr.Kind, wantCode apperr.Code) *apperr.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	var got *apperr.Error
	if !errors.As(err, &got) {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	if got.Kind != wantKind || got.Code != wantCode {
		t.Fatalf("error = (%s, %s), want (%s, %s)", got.Kind, got.Code, wantKind, wantCode)
	}
	return got
}

func TestSendRequestExactJSON(t *testing.T) {
	client := newTestClient(t, doerFunc(func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", request.Method)
		}
		if request.URL.Path != "/bot123:ABC/sendMessage" {
			t.Errorf("path = %s, want /bot123:ABC/sendMessage", request.URL.Path)
		}
		if got := request.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type = %q, want application/json", got)
		}
		var got map[string]any
		if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if _, ok := got["parse_mode"]; ok {
			t.Error("request contains parse_mode")
		}
		want := map[string]any{"chat_id": "@chat", "text": "hello", "disable_notification": false}
		if fmt.Sprint(got) != fmt.Sprint(want) {
			t.Errorf("request = %#v, want %#v", got, want)
		}
		return response(http.StatusOK, `{"ok":true,"result":{"message_id":42}}`), nil
	}))

	if _, err := client.Send(context.Background(), "@chat", message.Chunk{Text: "hello", Entities: []message.Entity{}}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestSendPreservesTextAndEntities(t *testing.T) {
	want := message.Chunk{Text: "🚀 body", Entities: []message.Entity{{Type: "pre", Offset: 2, Length: 7}}, DisableNotification: true}
	client := newTestClient(t, doerFunc(func(request *http.Request) (*http.Response, error) {
		var got sendMessageRequest
		if err := json.NewDecoder(request.Body).Decode(&got); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if got.ChatID != "-100123" || got.Text != want.Text || got.DisableNotification != want.DisableNotification || len(got.Entities) != 1 || got.Entities[0] != want.Entities[0] {
			t.Errorf("request = %#v, want text/entities preserved", got)
		}
		return response(http.StatusOK, `{"ok":true,"result":{"message_id":7}}`), nil
	}))
	if _, err := client.Send(context.Background(), "-100123", want); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}

func TestSendSuccessMessageID(t *testing.T) {
	client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"ok":true,"result":{"message_id":987654321}}`), nil
	}))
	got, err := client.Send(context.Background(), "chat", message.Chunk{Text: "ok"})
	if err != nil || got != 987654321 {
		t.Fatalf("Send() = (%d, %v), want (987654321, nil)", got, err)
	}
}

func TestSendTelegramRejection(t *testing.T) {
	client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"ok":false,"error_code":400,"description":"bad request"}`), nil
	}))
	err := func() error {
		_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "bad"})
		return err
	}()
	got := appError(t, err, apperr.KindTelegram, apperr.CodeTelegramRejected)
	if got.Message != "Telegram API rejected request (code 400)" {
		t.Errorf("safe message = %q", got.Message)
	}
}

func TestSendHTTP5xxNotSuccess(t *testing.T) {
	client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusBadGateway, "upstream failure"), nil
	}))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "bad"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramTransport)
}

func TestSendMalformedAndTrailingJSON(t *testing.T) {
	for _, body := range []string{"not-json", `{"ok":true,"result":{"message_id":1}} {"extra":true}`} {
		t.Run(body, func(t *testing.T) {
			client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
				return response(http.StatusOK, body), nil
			}))
			_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "bad"})
			_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramProtocol)
		})
	}
}

func TestSendOversizedResponse(t *testing.T) {
	client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, strings.Repeat("x", maxResponseBytes+1)), nil
	}))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "bad"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramProtocol)
}

type trackingBody struct {
	io.Reader
	closed bool
}

func (body *trackingBody) Close() error {
	body.closed = true
	return nil
}

func TestSendClosesBody(t *testing.T) {
	body := &trackingBody{Reader: strings.NewReader(`{"ok":true,"result":{"message_id":1}}`)}
	client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: body}, nil
	}))
	if _, err := client.Send(context.Background(), "chat", message.Chunk{Text: "ok"}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if !body.closed {
		t.Error("response body was not closed")
	}
}

func TestSendContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	client := newTestClient(t, doerFunc(func(request *http.Request) (*http.Response, error) {
		return nil, request.Context().Err()
	}))
	_, err := client.Send(ctx, "chat", message.Chunk{Text: "cancelled"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramTransport)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error does not preserve context cancellation: %v", err)
	}
}

func TestTokenAbsentFromEveryError(t *testing.T) {
	token := "123:SECRET_TOKEN"
	client, err := NewClient(Options{Token: token, BaseURL: "https://api.telegram.org", Doer: doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New(token)
	})})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Send(context.Background(), "chat", message.Chunk{Text: token})
	if err == nil || strings.Contains(err.Error(), token) || strings.Contains(fmt.Sprint(err), token) {
		t.Fatalf("token leaked in error: %v", err)
	}
}

func TestBaseURLValidation(t *testing.T) {
	for _, base := range []string{
		"ftp://api.telegram.org",
		"https://api.telegram.org?token=secret",
		"https://user:pass@api.telegram.org",
		"://broken",
	} {
		t.Run(base, func(t *testing.T) {
			_, err := NewClient(Options{Token: "123:ABC", BaseURL: base})
			_ = appError(t, err, apperr.KindUsage, apperr.CodeInvalidArguments)
		})
	}
}

func TestHTTP429IncludesRetryFacts(t *testing.T) {
	client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusTooManyRequests, `{"ok":false,"error_code":429,"parameters":{"retry_after":2}}`), nil
	}))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "rate"})
	_ = appError(t, err, apperr.KindRateLimit, apperr.CodeTelegramRateLimited)
	var failure *attemptFailure
	if !errors.As(err, &failure) || failure.retryAfter != 2 || failure.statusCode != http.StatusTooManyRequests {
		t.Fatalf("retry facts missing from error: %v", err)
	}
}

func TestAPI429IncludesRetryFacts(t *testing.T) {
	client := newTestClient(t, doerFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"ok":false,"error_code":429,"parameters":{"retry_after":3}}`), nil
	}))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "rate"})
	_ = appError(t, err, apperr.KindRateLimit, apperr.CodeTelegramRateLimited)
	var failure *attemptFailure
	if !errors.As(err, &failure) || failure.retryAfter != 3 || failure.errorCode != http.StatusTooManyRequests {
		t.Fatalf("retry facts missing from error: %v", err)
	}
}

func TestRequestBodyIsNotModifiedBySend(t *testing.T) {
	client := newTestClient(t, doerFunc(func(request *http.Request) (*http.Response, error) {
		data, _ := io.ReadAll(request.Body)
		if !bytes.Contains(data, []byte(`"text":"body"`)) {
			t.Errorf("request does not contain original text: %s", data)
		}
		return response(http.StatusOK, `{"ok":true,"result":{"message_id":1}}`), nil
	}))
	chunk := message.Chunk{Text: "body", Entities: []message.Entity{{Type: "bold", Offset: 0, Length: 4}}}
	if _, err := client.Send(context.Background(), "chat", chunk); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
}
