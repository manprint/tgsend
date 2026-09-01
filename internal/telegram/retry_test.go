package telegram

import (
	"context"
	"errors"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/message"
)

type retrySleeper struct {
	delays []time.Duration
	err    error
}

func (sleeper *retrySleeper) Sleep(_ context.Context, delay time.Duration) error {
	sleeper.delays = append(sleeper.delays, delay)
	return sleeper.err
}

func retryClient(t *testing.T, sleeper Sleeper, responses ...*http.Response) (*Client, *int) {
	t.Helper()
	remaining := len(responses)
	client, err := NewClient(Options{
		Token:   "123:ABC",
		BaseURL: "https://api.telegram.org",
		Sleeper: sleeper,
		Doer: doerFunc(func(*http.Request) (*http.Response, error) {
			index := len(responses) - remaining
			if remaining == 0 {
				return nil, errors.New("unexpected extra request")
			}
			remaining--
			return responses[index], nil
		}),
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	return client, &remaining
}

func retryResponse(retryAfter int) *http.Response {
	return response(http.StatusTooManyRequests, `{"ok":false,"error_code":429,"parameters":{"retry_after":`+itoa(retryAfter)+`}}`)
}

func apiRetryResponse(retryAfter int) *http.Response {
	return response(http.StatusOK, `{"ok":false,"error_code":429,"parameters":{"retry_after":`+itoa(retryAfter)+`}}`)
}

func itoa(value int) string {
	return strconv.Itoa(value)
}

func successResponse(id int64) *http.Response {
	return response(http.StatusOK, `{"ok":true,"result":{"message_id":`+strconv.FormatInt(id, 10)+`}}`)
}

func Test429ThenSuccessSleepsAndRetries(t *testing.T) {
	sleeper := &retrySleeper{}
	client, remaining := retryClient(t, sleeper, retryResponse(1), successResponse(10))
	id, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	if err != nil || id != 10 {
		t.Fatalf("Send() = (%d, %v), want (10, nil)", id, err)
	}
	if *remaining != 0 || !reflect.DeepEqual(sleeper.delays, []time.Duration{time.Second}) {
		t.Fatalf("remaining=%d delays=%v, want no remaining request and one 1s sleep", *remaining, sleeper.delays)
	}
}

func TestTwoRetriesThenSuccess(t *testing.T) {
	sleeper := &retrySleeper{}
	client, remaining := retryClient(t, sleeper, retryResponse(1), retryResponse(2), successResponse(11))
	id, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	if err != nil || id != 11 {
		t.Fatalf("Send() = (%d, %v), want (11, nil)", id, err)
	}
	if *remaining != 0 || !reflect.DeepEqual(sleeper.delays, []time.Duration{time.Second, 2 * time.Second}) {
		t.Fatalf("remaining=%d delays=%v, want two retries", *remaining, sleeper.delays)
	}
}

func TestThird429StopsAtThreeAttempts(t *testing.T) {
	sleeper := &retrySleeper{}
	client, remaining := retryClient(t, sleeper, retryResponse(1), retryResponse(1), retryResponse(1), successResponse(12))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindRateLimit, apperr.CodeTelegramRateLimited)
	if *remaining != 1 || len(sleeper.delays) != 2 {
		t.Fatalf("remaining=%d delays=%v, want one unused response and two sleeps", *remaining, sleeper.delays)
	}
}

func TestRetryAfterExceedingBudgetDoesNotSleep(t *testing.T) {
	sleeper := &retrySleeper{}
	client, remaining := retryClient(t, sleeper, retryResponse(61))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindRateLimit, apperr.CodeTelegramRateLimited)
	if *remaining != 0 || len(sleeper.delays) != 0 {
		t.Fatalf("remaining=%d delays=%v, want one request and no sleep", *remaining, sleeper.delays)
	}
}

func TestCumulativeWaitExceeding60Stops(t *testing.T) {
	sleeper := &retrySleeper{}
	client, remaining := retryClient(t, sleeper, retryResponse(60), retryResponse(1), successResponse(13))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindRateLimit, apperr.CodeTelegramRateLimited)
	if *remaining != 1 || !reflect.DeepEqual(sleeper.delays, []time.Duration{60 * time.Second}) {
		t.Fatalf("remaining=%d delays=%v, want no third request and one 60s sleep", *remaining, sleeper.delays)
	}
}

func TestMissingOrZeroRetryAfterDoesNotRetry(t *testing.T) {
	for _, body := range []string{
		`{"ok":false,"error_code":429,"parameters":{"retry_after":0}}`,
		`{"ok":false,"error_code":429}`,
	} {
		t.Run(body, func(t *testing.T) {
			sleeper := &retrySleeper{}
			client, remaining := retryClient(t, sleeper, response(http.StatusOK, body), successResponse(14))
			_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
			_ = appError(t, err, apperr.KindRateLimit, apperr.CodeTelegramRateLimited)
			if *remaining != 1 || len(sleeper.delays) != 0 {
				t.Fatalf("remaining=%d delays=%v, want no retry", *remaining, sleeper.delays)
			}
		})
	}
}

func TestHTTP429AndAPI429Equivalent(t *testing.T) {
	for _, first := range []*http.Response{retryResponse(1), apiRetryResponse(1)} {
		t.Run(first.Status, func(t *testing.T) {
			sleeper := &retrySleeper{}
			client, remaining := retryClient(t, sleeper, first, successResponse(15))
			id, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
			if err != nil || id != 15 || *remaining != 0 || !reflect.DeepEqual(sleeper.delays, []time.Duration{time.Second}) {
				t.Fatalf("Send() = (%d, %v), remaining=%d delays=%v", id, err, *remaining, sleeper.delays)
			}
		})
	}
}

func Test5xxNeverRetries(t *testing.T) {
	sleeper := &retrySleeper{}
	client, remaining := retryClient(t, sleeper, response(http.StatusBadGateway, "bad gateway"), successResponse(16))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramTransport)
	if *remaining != 1 || len(sleeper.delays) != 0 {
		t.Fatalf("remaining=%d delays=%v, want no retry", *remaining, sleeper.delays)
	}
}

func TestTransportNeverRetries(t *testing.T) {
	sleeper := &retrySleeper{}
	client, err := NewClient(Options{Token: "123:ABC", BaseURL: "https://api.telegram.org", Sleeper: sleeper, Doer: doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("connection failed")
	})})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramTransport)
	if len(sleeper.delays) != 0 {
		t.Fatalf("delays=%v, want no retry", sleeper.delays)
	}
}

func TestTimeoutNeverRetries(t *testing.T) {
	sleeper := &retrySleeper{}
	client, err := NewClient(Options{Token: "123:ABC", BaseURL: "https://api.telegram.org", Sleeper: sleeper, Doer: doerFunc(func(*http.Request) (*http.Response, error) {
		return nil, context.DeadlineExceeded
	})})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramTransport)
	if len(sleeper.delays) != 0 {
		t.Fatalf("delays=%v, want no retry", sleeper.delays)
	}
}

func TestMalformedNeverRetries(t *testing.T) {
	sleeper := &retrySleeper{}
	client, remaining := retryClient(t, sleeper, response(http.StatusOK, "malformed"), successResponse(17))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramProtocol)
	if *remaining != 1 || len(sleeper.delays) != 0 {
		t.Fatalf("remaining=%d delays=%v, want no retry", *remaining, sleeper.delays)
	}
}

func TestContextCancelledDuringSleepDoesNotRetry(t *testing.T) {
	sleeper := &retrySleeper{err: context.Canceled}
	client, remaining := retryClient(t, sleeper, retryResponse(1), successResponse(18))
	_, err := client.Send(context.Background(), "chat", message.Chunk{Text: "body"})
	_ = appError(t, err, apperr.KindTransport, apperr.CodeTelegramTransport)
	if !errors.Is(err, context.Canceled) || *remaining != 1 || len(sleeper.delays) != 1 {
		t.Fatalf("err=%v remaining=%d delays=%v, want cancellation and no retry", err, *remaining, sleeper.delays)
	}
}

func TestRetryResendsOnlyCurrentChunk(t *testing.T) {
	sleeper := &retrySleeper{}
	var bodies [][]byte
	client, remaining := retryClient(t, sleeper, retryResponse(1), successResponse(19))
	client.doer = doerFunc(func(request *http.Request) (*http.Response, error) {
		body, err := io.ReadAll(request.Body)
		if err != nil {
			return nil, err
		}
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			return retryResponse(1), nil
		}
		return successResponse(19), nil
	})
	if _, err := client.Send(context.Background(), "chat", message.Chunk{Text: "🚀 current", Entities: []message.Entity{{Type: "pre", Offset: 0, Length: 2}}, DisableNotification: true}); err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if *remaining != 2 || len(bodies) != 2 || !reflect.DeepEqual(bodies[0], bodies[1]) {
		t.Fatalf("remaining=%d bodies=%d equal=%v, want two identical current-chunk requests", *remaining, len(bodies), len(bodies) == 2 && reflect.DeepEqual(bodies[0], bodies[1]))
	}
}

func TestRetryDelayRejectsOverflow(t *testing.T) {
	if _, ok := retryDelay(0); ok {
		t.Error("retryDelay(0) accepted")
	}
	if _, ok := retryDelay(-1); ok {
		t.Error("retryDelay(-1) accepted")
	}
	if _, ok := retryDelay(int(^uint(0) >> 1)); ok && strconv.IntSize == 64 {
		t.Error("retryDelay(max int) accepted as representable duration")
	}
}
