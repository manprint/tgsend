package app

import (
	"context"
	"errors"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/config"
	"github.com/manprint/tgsend/internal/input"
	"github.com/manprint/tgsend/internal/message"
)

func TestServiceValidationBeforeConfig(t *testing.T) {
	wantErr := apperr.New(apperr.KindInput, apperr.CodeInputEmpty, "input is empty", nil)
	loaded := false
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "", wantErr },
		ConfigLoader: configLoaderFunc(func(config.LoadOptions) (config.Config, error) {
			loaded = true
			return config.Config{}, nil
		}),
	}
	_, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want input error", err)
	}
	if loaded {
		t.Fatal("configuration loaded before input validation")
	}
}

func TestServicePlanBeforeConfig(t *testing.T) {
	wantErr := apperr.New(apperr.KindUsage, apperr.CodeInvalidFlag, "invalid plan", nil)
	loaded := false
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "body", nil },
		Planner: plannerFunc(func(string, message.Options) ([]message.Chunk, error) {
			return nil, wantErr
		}),
		ConfigLoader: configLoaderFunc(func(config.LoadOptions) (config.Config, error) {
			loaded = true
			return config.Config{}, nil
		}),
	}
	_, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want planner error", err)
	}
	if loaded {
		t.Fatal("configuration loaded before planning completed")
	}
}

func TestDryRunSkipsConfigAndSender(t *testing.T) {
	loaded := false
	sent := false
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "Hello", nil },
		Planner:   message.Planner{},
		ConfigLoader: configLoaderFunc(func(config.LoadOptions) (config.Config, error) {
			loaded = true
			return config.Config{}, errors.New("dry-run must not load config")
		}),
		Sender: senderFunc(func(context.Context, config.Config, message.Chunk) (int64, error) {
			sent = true
			return 1, nil
		}),
	}
	result, err := service.Run(context.Background(), Options{DryRun: true, MaxInputBytes: 100})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if loaded || sent {
		t.Fatalf("dry-run side effects: config=%t sender=%t", loaded, sent)
	}
	if !result.DryRun || result.ChunksTotal != 1 || result.ChunksSent != 0 || len(result.MessageIDs) != 0 || len(result.Chunks) != 1 {
		t.Fatalf("Run() result = %#v", result)
	}
}

func TestPlanFailureSendsNothing(t *testing.T) {
	sent := false
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "body", nil },
		Planner: plannerFunc(func(string, message.Options) ([]message.Chunk, error) {
			return nil, errors.New("planner failed")
		}),
		Sender: senderFunc(func(context.Context, config.Config, message.Chunk) (int64, error) {
			sent = true
			return 0, nil
		}),
	}
	if _, err := service.Run(context.Background(), Options{MaxInputBytes: 100}); err == nil {
		t.Fatal("Run() error = nil, want planner failure")
	}
	if sent {
		t.Fatal("sender called after plan failure")
	}
}

func TestSequentialSendOrder(t *testing.T) {
	chunks := []message.Chunk{{Text: "one"}, {Text: "two"}, {Text: "three"}}
	var got []string
	service := testService(chunks, func(config.LoadOptions) (config.Config, error) {
		return config.Config{Token: "token", ChatID: "-100"}, nil
	}, func(_ context.Context, cfg config.Config, chunk message.Chunk) (int64, error) {
		if cfg.ChatID != "-100" {
			t.Errorf("chat ID = %q", cfg.ChatID)
		}
		got = append(got, chunk.Text)
		return int64(len(got) + 10), nil
	})
	result, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !reflect.DeepEqual(got, []string{"one", "two", "three"}) {
		t.Fatalf("send order = %#v", got)
	}
	if !reflect.DeepEqual(result.MessageIDs, []int64{11, 12, 13}) || result.ChunksTotal != 3 || result.ChunksSent != 3 || result.Chunks != nil {
		t.Fatalf("result = %#v", result)
	}
}

func TestStopAtFirstFailure(t *testing.T) {
	chunks := []message.Chunk{{Text: "one"}, {Text: "two"}, {Text: "three"}}
	calls := 0
	wantErr := apperr.New(apperr.KindTelegram, apperr.CodeTelegramRejected, "Telegram API rejected request", nil)
	service := testService(chunks, nil, func(_ context.Context, _ config.Config, _ message.Chunk) (int64, error) {
		calls++
		if calls == 2 {
			return 0, wantErr
		}
		return int64(calls), nil
	})
	_, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
	if !errors.Is(err, wantErr) || calls != 2 {
		t.Fatalf("error/calls = %v/%d", err, calls)
	}
	appErr := requireAppError(t, err)
	if appErr.Kind != wantErr.Kind || appErr.Code != wantErr.Code || appErr.Message != wantErr.Message || appErr.Progress == nil {
		t.Fatalf("wrapped error = %#v", appErr)
	}
	if *appErr.Progress != (apperr.Progress{ChunksTotal: 3, ChunksSent: 1, FailedChunk: 2}) {
		t.Fatalf("progress = %#v", appErr.Progress)
	}
}

func TestPartialProgressZeroFirstMiddleLast(t *testing.T) {
	for _, testCase := range []struct {
		name      string
		failed    int
		expected  apperr.Progress
		callCount int
	}{
		{name: "first", failed: 0, expected: apperr.Progress{ChunksTotal: 3, ChunksSent: 0, FailedChunk: 1}, callCount: 1},
		{name: "middle", failed: 1, expected: apperr.Progress{ChunksTotal: 3, ChunksSent: 1, FailedChunk: 2}, callCount: 2},
		{name: "last", failed: 2, expected: apperr.Progress{ChunksTotal: 3, ChunksSent: 2, FailedChunk: 3}, callCount: 3},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			calls := 0
			service := testService([]message.Chunk{{Text: "one"}, {Text: "two"}, {Text: "three"}}, nil, func(_ context.Context, _ config.Config, _ message.Chunk) (int64, error) {
				calls++
				if calls-1 == testCase.failed {
					return 0, apperr.New(apperr.KindRateLimit, apperr.CodeTelegramRateLimited, "rate limited", nil)
				}
				return int64(calls), nil
			})
			_, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
			if calls != testCase.callCount {
				t.Fatalf("calls = %d, want %d", calls, testCase.callCount)
			}
			appErr := requireAppError(t, err)
			if *appErr.Progress != testCase.expected {
				t.Fatalf("progress = %#v, want %#v", appErr.Progress, testCase.expected)
			}
		})
	}
}

func TestSuccessIDsInOrder(t *testing.T) {
	service := testService([]message.Chunk{{Text: "one"}, {Text: "two"}}, nil, func(_ context.Context, _ config.Config, chunk message.Chunk) (int64, error) {
		if chunk.Text == "one" {
			return 101, nil
		}
		return 202, nil
	})
	result, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
	if err != nil || !reflect.DeepEqual(result.MessageIDs, []int64{101, 202}) {
		t.Fatalf("result/error = %#v/%v", result, err)
	}
}

func TestConfigNotLoadedUntilPlanComplete(t *testing.T) {
	planned := false
	loaded := false
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "body", nil },
		Planner: plannerFunc(func(string, message.Options) ([]message.Chunk, error) {
			planned = true
			return []message.Chunk{{Text: "planned"}}, nil
		}),
		ConfigLoader: configLoaderFunc(func(config.LoadOptions) (config.Config, error) {
			loaded = true
			if !planned {
				t.Fatal("configuration loaded during planning")
			}
			return config.Config{Token: "token", ChatID: "-100"}, nil
		}),
		Sender: senderFunc(func(context.Context, config.Config, message.Chunk) (int64, error) { return 1, nil }),
	}
	if _, err := service.Run(context.Background(), Options{MaxInputBytes: 100}); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !loaded {
		t.Fatal("configuration was not loaded")
	}
}

func TestNoConcurrentSend(t *testing.T) {
	started := make(chan struct{}, 3)
	release := make(chan struct{})
	var active int32
	var maximum int32
	service := testService([]message.Chunk{{Text: "one"}, {Text: "two"}, {Text: "three"}}, nil, func(_ context.Context, _ config.Config, _ message.Chunk) (int64, error) {
		current := atomic.AddInt32(&active, 1)
		for {
			old := atomic.LoadInt32(&maximum)
			if current <= old || atomic.CompareAndSwapInt32(&maximum, old, current) {
				break
			}
		}
		started <- struct{}{}
		if current == 1 {
			<-release
		}
		atomic.AddInt32(&active, -1)
		return int64(current), nil
	})
	done := make(chan error, 1)
	go func() {
		_, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
		done <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first send did not start")
	}
	select {
	case <-started:
		t.Fatal("second send started before first completed")
	case <-time.After(20 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if maximum != 1 {
		t.Fatalf("maximum concurrent sends = %d, want 1", maximum)
	}
}

func testService(chunks []message.Chunk, load func(config.LoadOptions) (config.Config, error), send func(context.Context, config.Config, message.Chunk) (int64, error)) *Service {
	if load == nil {
		load = func(config.LoadOptions) (config.Config, error) {
			return config.Config{Token: "token", ChatID: "-100"}, nil
		}
	}
	return &Service{
		ReadInput: func(input.Source) (string, error) { return "body", nil },
		Planner: plannerFunc(func(string, message.Options) ([]message.Chunk, error) {
			return chunks, nil
		}),
		ConfigLoader: configLoaderFunc(load),
		Sender:       senderFunc(send),
	}
}

func requireAppError(t *testing.T, err error) *apperr.Error {
	t.Helper()
	if err == nil {
		t.Fatal("expected application error")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr == nil {
		t.Fatalf("error type = %T, want *apperr.Error", err)
	}
	return appErr
}

type plannerFunc func(string, message.Options) ([]message.Chunk, error)

func (function plannerFunc) Plan(body string, options message.Options) ([]message.Chunk, error) {
	return function(body, options)
}

type senderFunc func(context.Context, config.Config, message.Chunk) (int64, error)

func (function senderFunc) Send(ctx context.Context, cfg config.Config, chunk message.Chunk) (int64, error) {
	return function(ctx, cfg, chunk)
}

var _ Planner = plannerFunc(nil)
var _ Sender = senderFunc(nil)
