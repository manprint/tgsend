// Package apperr defines the errors that cross the CLI process boundary.
package apperr

import (
	"errors"
	"fmt"
)

// Kind identifies the user-visible failure category and its process exit code.
type Kind string

const (
	KindUsage     Kind = "usage"
	KindConfig    Kind = "config"
	KindInput     Kind = "input"
	KindTelegram  Kind = "telegram"
	KindTransport Kind = "transport"
	KindRateLimit Kind = "rate_limit"
)

// Code is the stable machine-readable error identifier.
type Code string

const (
	CodeInvalidArguments    Code = "invalid_arguments"
	CodeConflictingInput    Code = "conflicting_input"
	CodeInvalidFlag         Code = "invalid_flag"
	CodeConfigNotFound      Code = "config_not_found"
	CodeConfigUnreadable    Code = "config_unreadable"
	CodeConfigInvalid       Code = "config_invalid"
	CodeConfigIncomplete    Code = "config_incomplete"
	CodeInputEmpty          Code = "input_empty"
	CodeInputUnreadable     Code = "input_unreadable"
	CodeInputTooLarge       Code = "input_too_large"
	CodeInputInvalidUTF8    Code = "input_invalid_utf8"
	CodeTelegramRejected    Code = "telegram_rejected"
	CodeTelegramTransport   Code = "telegram_transport"
	CodeTelegramProtocol    Code = "telegram_protocol"
	CodeTelegramRateLimited Code = "telegram_rate_limited"
)

// Progress reports how many chunks were sent before a failure.
type Progress struct {
	ChunksTotal int
	ChunksSent  int
	FailedChunk int
}

// Validate checks the one-based failed-chunk invariant.
func (p Progress) Validate() error {
	if p.ChunksTotal < 0 || p.ChunksSent < 0 || p.FailedChunk < 0 {
		return errors.New("progress counts must not be negative")
	}
	if p.ChunksSent > p.ChunksTotal {
		return errors.New("sent chunks must not exceed total chunks")
	}
	if p.FailedChunk != 0 && (p.FailedChunk < 1 || p.FailedChunk > p.ChunksTotal) {
		return errors.New("failed chunk must be a one-based index within total chunks")
	}
	return nil
}

// Error is a safe, typed application error. The underlying cause is never
// included in the user-facing message or JSON representation.
type Error struct {
	cause    error
	Code     Code
	Kind     Kind
	Message  string
	Progress *Progress
}

// New creates an application error with a caller-supplied safe message.
func New(kind Kind, code Code, message string, cause error) *Error {
	return &Error{cause: cause, Code: code, Kind: kind, Message: message}
}

// NewWithProgress creates an application error with validated progress data.
func NewWithProgress(kind Kind, code Code, message string, cause error, progress Progress) (*Error, error) {
	if err := progress.Validate(); err != nil {
		return nil, err
	}
	return &Error{
		cause:    cause,
		Code:     code,
		Kind:     kind,
		Message:  message,
		Progress: &progress,
	}, nil
}

// Error returns only the safe message selected by the caller.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// Unwrap exposes the private cause for programmatic inspection only.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// ExitCode converts an application error into the stable CLI exit status.
func ExitCode(err error) int {
	if err == nil {
		return 1
	}

	var appErr *Error
	if !errors.As(err, &appErr) || appErr == nil {
		return 1
	}

	switch appErr.Kind {
	case KindUsage:
		return 2
	case KindConfig:
		return 3
	case KindInput:
		return 4
	case KindTelegram:
		return 5
	case KindTransport:
		return 6
	case KindRateLimit:
		return 7
	default:
		return 1
	}
}

// ValidateProgress returns a descriptive validation error for progress data.
// It is kept as a small package helper for callers that validate before
// constructing an Error.
func ValidateProgress(progress Progress) error {
	if err := progress.Validate(); err != nil {
		return fmt.Errorf("invalid progress: %w", err)
	}
	return nil
}
