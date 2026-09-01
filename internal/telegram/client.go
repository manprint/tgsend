// Package telegram contains the small, side-effecting Telegram Bot API client.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/message"
)

const (
	productionBaseURL = "https://api.telegram.org"
	maxResponseBytes  = 1 << 20
)

var (
	errInvalidClientOptions = errors.New("invalid telegram client options")
	errTransport            = errors.New("telegram transport failure")
	errProtocol             = errors.New("telegram protocol failure")
)

// Doer is the part of http.Client used by Client. It keeps protocol tests
// independent from a live Telegram endpoint.
type Doer interface {
	Do(*http.Request) (*http.Response, error)
}

// Options configures a Bot API client. BaseURL is only intended for a local
// test endpoint in tests; an empty value selects the Telegram production API.
type Options struct {
	Token   string
	BaseURL string
	Doer    Doer
}

// Client sends one already-planned message chunk to one chat.
type Client struct {
	token string
	base  *url.URL
	doer  Doer
}

type sendMessageRequest struct {
	ChatID              string           `json:"chat_id"`
	Text                string           `json:"text"`
	Entities            []message.Entity `json:"entities,omitempty"`
	DisableNotification bool             `json:"disable_notification"`
}

type botResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		MessageID int64 `json:"message_id"`
	} `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

// attemptFailure is deliberately safe: it records only numeric protocol
// facts for the retry policy and never stores URLs, response bodies, or text
// supplied by Telegram.
type attemptFailure struct {
	statusCode int
	errorCode  int
	retryAfter int
}

func (failure *attemptFailure) Error() string {
	return "telegram request attempt failed"
}

// NewClient validates the endpoint and returns a client ready for one-attempt
// sends. The retry policy is layered on top in the next phase.
func NewClient(options Options) (*Client, error) {
	if err := validateToken(options.Token); err != nil {
		return nil, apperr.New(apperr.KindConfig, apperr.CodeConfigIncomplete, "Telegram token is missing or invalid", err)
	}
	base := options.BaseURL
	if base == "" {
		base = productionBaseURL
	}
	parsed, err := url.Parse(base)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return nil, apperr.New(apperr.KindUsage, apperr.CodeInvalidArguments, "Telegram API base URL must be an HTTP(S) URL without credentials or query parameters", errInvalidClientOptions)
	}
	doer := options.Doer
	if doer == nil {
		doer = http.DefaultClient
	}
	return &Client{token: options.Token, base: parsed, doer: doer}, nil
}

func validateToken(token string) error {
	if token == "" {
		return errInvalidClientOptions
	}
	for _, r := range token {
		if unicode.IsSpace(r) || r == '/' || r == '?' || r == '#' || r == '%' || unicode.IsControl(r) {
			return errInvalidClientOptions
		}
	}
	return nil
}

// Send performs exactly one HTTP request for chunk. It returns the Telegram
// message ID on success and an apperr.Error with a stable, secret-free
// message on every failure.
func (client *Client) Send(ctx context.Context, chatID string, chunk message.Chunk) (int64, error) {
	if client == nil || client.base == nil || client.doer == nil {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "Telegram client is not configured", errInvalidClientOptions)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	body, err := json.Marshal(sendMessageRequest{
		ChatID:              chatID,
		Text:                chunk.Text,
		Entities:            chunk.Entities,
		DisableNotification: chunk.DisableNotification,
	})
	if err != nil {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramProtocol, "could not encode Telegram request", errProtocol)
	}

	endpoint := *client.base
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/bot" + client.token + "/sendMessage"
	endpoint.RawPath = ""
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "could not create Telegram request", errTransport)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.doer.Do(request)
	if err != nil {
		cause := errTransport
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			cause = err
		}
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "Telegram request failed", cause)
	}
	if response == nil {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "Telegram request failed", errTransport)
	}
	if response.Body == nil {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramProtocol, "Telegram response body is missing", errProtocol)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode >= http.StatusInternalServerError {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, fmt.Sprintf("Telegram request failed with HTTP status %d", response.StatusCode), errTransport)
	}
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramProtocol, "could not read Telegram response", errProtocol)
	}
	if len(responseBody) > maxResponseBytes {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramProtocol, "Telegram response is too large", errProtocol)
	}

	var decoded botResponse
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	if err := decoder.Decode(&decoded); err != nil {
		if response.StatusCode == http.StatusTooManyRequests {
			return 0, rateLimitError(&attemptFailure{statusCode: response.StatusCode})
		}
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramProtocol, "Telegram response is malformed", errProtocol)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramProtocol, "Telegram response contains trailing data", errProtocol)
	}

	failure := &attemptFailure{
		statusCode: response.StatusCode,
		errorCode:  decoded.ErrorCode,
		retryAfter: decoded.Parameters.RetryAfter,
	}
	if response.StatusCode == http.StatusTooManyRequests || decoded.ErrorCode == http.StatusTooManyRequests {
		return 0, rateLimitError(failure)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices || !decoded.OK {
		code := decoded.ErrorCode
		if code == 0 {
			code = response.StatusCode
		}
		return 0, apperr.New(apperr.KindTelegram, apperr.CodeTelegramRejected, fmt.Sprintf("Telegram API rejected request (code %d)", code), failure)
	}
	if decoded.Result.MessageID <= 0 {
		return 0, apperr.New(apperr.KindTransport, apperr.CodeTelegramProtocol, "Telegram response did not contain a message ID", errProtocol)
	}
	return decoded.Result.MessageID, nil
}

func rateLimitError(failure *attemptFailure) error {
	return apperr.New(apperr.KindRateLimit, apperr.CodeTelegramRateLimited, "Telegram rate limit response cannot be retried by this attempt", failure)
}
