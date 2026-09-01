package app

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/config"
	"github.com/manprint/tgsend/internal/input"
	"github.com/manprint/tgsend/internal/message"
	"github.com/manprint/tgsend/internal/presenter"
)

const defaultMaxInputBytes int64 = 1 << 20

// Options are the validated command options consumed by the application service.
type Options struct {
	Message        string
	MessageSet     bool
	ConfigPath     string
	ConfigExplicit bool
	Title          string
	Type           string
	Monospace      bool
	Silent         bool
	DryRun         bool
	MaxInputBytes  int64
}

// ConfigLoader reads and validates the selected application configuration.
type ConfigLoader interface {
	Load(config.LoadOptions) (config.Config, error)
}

type configLoaderFunc func(config.LoadOptions) (config.Config, error)

func (function configLoaderFunc) Load(options config.LoadOptions) (config.Config, error) {
	return function(options)
}

// Planner creates message chunks from the exact input body.
type Planner interface {
	Plan(body string, options message.Options) ([]message.Chunk, error)
}

// Sender sends one already-planned chunk and returns its Telegram message ID.
type Sender interface {
	Send(context.Context, config.Config, message.Chunk) (int64, error)
}

// Service owns input, planning, configuration, and transport ordering.
type Service struct {
	Stdin           io.Reader
	StdinIsTerminal bool
	ReadInput       func(input.Source) (string, error)
	ConfigLoader    ConfigLoader
	Planner         Planner
	Sender          Sender
	PreflightError  error
}

// NewService returns the application service with production input, config, and planner defaults.
func NewService(stdin io.Reader, stdinIsTerminal bool) *Service {
	return &Service{
		Stdin:           stdin,
		StdinIsTerminal: stdinIsTerminal,
		ReadInput:       input.Read,
		ConfigLoader:    configLoaderFunc(config.Load),
		Planner:         message.Planner{},
	}
}

// Run validates and plans the complete input before loading credentials or sending chunks.
func (service *Service) Run(ctx context.Context, options Options) (presenter.SendResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return presenter.SendResult{}, err
	}
	readInput := service.ReadInput
	if readInput == nil {
		readInput = input.Read
	}
	maxBytes := options.MaxInputBytes
	if maxBytes == 0 {
		maxBytes = defaultMaxInputBytes
	}
	body, err := readInput(input.Source{
		Message:         options.Message,
		MessageSet:      options.MessageSet,
		Stdin:           service.Stdin,
		StdinIsTerminal: service.StdinIsTerminal,
		MaxBytes:        maxBytes,
	})
	if err != nil {
		return presenter.SendResult{}, err
	}

	planner := service.Planner
	if planner == nil {
		planner = message.Planner{}
	}
	chunks, err := planner.Plan(body, message.Options{
		Title:     options.Title,
		Type:      options.Type,
		Monospace: options.Monospace,
		Silent:    options.Silent,
	})
	if err != nil {
		return presenter.SendResult{}, err
	}
	if options.DryRun {
		return presenter.SendResult{
			DryRun:      true,
			ChunksTotal: len(chunks),
			ChunksSent:  0,
			MessageIDs:  []int64{},
			Chunks:      preview(chunks),
		}, nil
	}

	if service.PreflightError != nil {
		return presenter.SendResult{}, service.PreflightError
	}
	loader := service.ConfigLoader
	if loader == nil {
		loader = configLoaderFunc(config.Load)
	}
	cfg, err := loader.Load(config.LoadOptions{
		Path:     options.ConfigPath,
		Explicit: options.ConfigExplicit,
	})
	if err != nil {
		return presenter.SendResult{}, err
	}
	sender := service.Sender
	if sender == nil {
		return presenter.SendResult{}, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "application sender is not configured", nil)
	}

	messageIDs := make([]int64, 0, len(chunks))
	for index, chunk := range chunks {
		messageID, sendErr := sender.Send(ctx, cfg, chunk)
		if sendErr != nil {
			return presenter.SendResult{}, withSendProgress(sendErr, len(chunks), len(messageIDs), index+1)
		}
		messageIDs = append(messageIDs, messageID)
	}
	return presenter.SendResult{
		DryRun:      false,
		ChunksTotal: len(chunks),
		ChunksSent:  len(messageIDs),
		MessageIDs:  messageIDs,
	}, nil
}

func withSendProgress(err error, total, sent, failed int) error {
	var appErr *apperr.Error
	if errors.As(err, &appErr) && appErr != nil {
		withProgress, progressErr := apperr.NewWithProgress(appErr.Kind, appErr.Code, appErr.Message, err, apperr.Progress{
			ChunksTotal: total,
			ChunksSent:  sent,
			FailedChunk: failed,
		})
		if progressErr == nil {
			return withProgress
		}
	}
	withProgress, progressErr := apperr.NewWithProgress(apperr.KindTransport, apperr.CodeTelegramTransport, "Telegram sending failed", err, apperr.Progress{
		ChunksTotal: total,
		ChunksSent:  sent,
		FailedChunk: failed,
	})
	if progressErr != nil {
		return apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "Telegram sending failed", err)
	}
	return withProgress
}

func preview(chunks []message.Chunk) []presenter.PreviewChunk {
	result := make([]presenter.PreviewChunk, 0, len(chunks))
	for index, chunk := range chunks {
		entities := make([]presenter.Entity, len(chunk.Entities))
		copy(entities, chunk.Entities)
		result = append(result, presenter.PreviewChunk{
			Index:               index + 1,
			Text:                chunk.Text,
			Entities:            entities,
			DisableNotification: chunk.DisableNotification,
		})
	}
	return result
}

// IsTerminal reports whether a file is a character device without requiring platform dependencies.
func IsTerminal(file *os.File) bool {
	if file == nil {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
