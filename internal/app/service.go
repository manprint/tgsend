package app

import (
	"context"
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
	Silent         bool
	DryRun         bool
	MaxInputBytes  int64
}

// Planner creates message chunks from the exact input body.
type Planner interface {
	Plan(body string, silent bool) ([]message.Chunk, error)
}

// Sender is the future transport boundary. Phase one deliberately does not invoke it.
type Sender interface {
	Send(context.Context, config.Config, []message.Chunk) ([]int64, error)
}

// Service owns input, planning, configuration, and transport ordering.
type Service struct {
	Stdin           io.Reader
	StdinIsTerminal bool
	ReadInput       func(input.Source) (string, error)
	LoadConfig      func(config.LoadOptions) (config.Config, error)
	Planner         Planner
	Sender          Sender
}

// NewService returns the phase-one service with production defaults.
func NewService(stdin io.Reader, stdinIsTerminal bool) *Service {
	return &Service{
		Stdin:           stdin,
		StdinIsTerminal: stdinIsTerminal,
		ReadInput:       input.Read,
		LoadConfig:      config.Load,
		Planner:         message.BasicPlanner{},
	}
}

// Run validates and plans input before producing an offline result.
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
		planner = message.BasicPlanner{}
	}
	chunks, err := planner.Plan(body, options.Silent)
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

	return presenter.SendResult{}, apperr.New(apperr.KindTransport, apperr.CodeTelegramTransport, "sending is not available in this build phase", nil)
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
