package app

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/config"
	"github.com/manprint/tgsend/internal/input"
	"github.com/manprint/tgsend/internal/message"
	"github.com/manprint/tgsend/internal/presenter"
)

func TestDryRunSkipsConfig(t *testing.T) {
	called := false
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "Hello", nil },
		LoadConfig: func(config.LoadOptions) (config.Config, error) {
			called = true
			return config.Config{}, errors.New("config must not be loaded")
		},
		Planner: message.Planner{},
	}
	result, err := service.Run(context.Background(), Options{DryRun: true, MaxInputBytes: 100})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if called {
		t.Fatal("dry-run loaded configuration")
	}
	if !result.DryRun || result.ChunksTotal != 1 || len(result.Chunks) != 1 || result.Chunks[0].Text != "Hello" {
		t.Fatalf("Run() result = %#v", result)
	}
}

func TestDryRunCallsPlannerOnce(t *testing.T) {
	calls := 0
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "body", nil },
		Planner: plannerFunc(func(body string, options message.Options) ([]message.Chunk, error) {
			calls++
			if body != "body" || !options.Silent {
				t.Fatalf("planner arguments = %q, %#v", body, options)
			}
			return []message.Chunk{{Text: body, Entities: []message.Entity{}}}, nil
		}),
	}
	_, err := service.Run(context.Background(), Options{DryRun: true, Silent: true, MaxInputBytes: 100})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if calls != 1 {
		t.Fatalf("planner calls = %d, want 1", calls)
	}
}

func TestApplicationValidationOrder(t *testing.T) {
	plannerCalled := false
	wantErr := apperr.New(apperr.KindInput, apperr.CodeInputEmpty, "input is empty", nil)
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "", wantErr },
		Planner: plannerFunc(func(string, message.Options) ([]message.Chunk, error) {
			plannerCalled = true
			return nil, nil
		}),
	}
	_, err := service.Run(context.Background(), Options{DryRun: true, MaxInputBytes: 100})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want input error", err)
	}
	if plannerCalled {
		t.Fatal("planner ran before input validation completed")
	}
}

func TestNonDryDoesNotAttemptNetwork(t *testing.T) {
	senderCalled := false
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return "Hello", nil },
		Planner:   message.Planner{},
		Sender: senderFunc(func(context.Context, config.Config, []message.Chunk) ([]int64, error) {
			senderCalled = true
			return nil, nil
		}),
	}
	_, err := service.Run(context.Background(), Options{MaxInputBytes: 100})
	if err == nil {
		t.Fatal("Run() error = nil, want unavailable transport")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeTelegramTransport || appErr.Message != "sending is not available in this build phase" {
		t.Fatalf("Run() error = %v", err)
	}
	if senderCalled {
		t.Fatal("non-dry phase-one execution attempted network")
	}
}

func TestPreviewPreservesEntitiesAndText(t *testing.T) {
	service := &Service{
		ReadInput: func(input.Source) (string, error) { return strings.Repeat("x", 3), nil },
		Planner: plannerFunc(func(string, message.Options) ([]message.Chunk, error) {
			return []message.Chunk{{Text: "x\r\n", Entities: []message.Entity{{Type: "bold", Offset: 0, Length: 1}}}}, nil
		}),
	}
	result, err := service.Run(context.Background(), Options{DryRun: true, MaxInputBytes: 100})
	if err != nil || len(result.Chunks) != 1 || result.Chunks[0].Text != "x\r\n" || result.Chunks[0].Entities[0].Type != "bold" {
		t.Fatalf("Run() = %#v, %v", result, err)
	}
}

type plannerFunc func(string, message.Options) ([]message.Chunk, error)

func (function plannerFunc) Plan(body string, options message.Options) ([]message.Chunk, error) {
	return function(body, options)
}

type senderFunc func(context.Context, config.Config, []message.Chunk) ([]int64, error)

func (function senderFunc) Send(ctx context.Context, cfg config.Config, chunks []message.Chunk) ([]int64, error) {
	return function(ctx, cfg, chunks)
}

var _ Planner = plannerFunc(nil)
var _ Sender = senderFunc(nil)
var _ = presenter.SendResult{}
