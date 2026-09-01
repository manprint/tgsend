// Package input acquires one exact message from a flag or piped stdin.
package input

import (
	"errors"
	"io"
	"math"
	"unicode/utf8"

	"github.com/manprint/tgsend/internal/apperr"
)

// Source describes the two mutually exclusive message sources.
type Source struct {
	Message         string
	MessageSet      bool
	Stdin           io.Reader
	StdinIsTerminal bool
	MaxBytes        int64
}

// Read returns the message bytes exactly as supplied by the selected source.
func Read(source Source) (string, error) {
	if source.MaxBytes <= 0 {
		return "", apperr.New(apperr.KindUsage, apperr.CodeInvalidFlag, "max input bytes must be positive", nil)
	}

	if source.MessageSet {
		if !source.StdinIsTerminal {
			if err := checkStdinUnused(source.Stdin); err != nil {
				return "", err
			}
		}
		return validateMessage(source.Message, source.MaxBytes)
	}

	if source.StdinIsTerminal {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputUnreadable, "stdin must be piped when no message flag is set", nil)
	}
	if source.Stdin == nil {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputUnreadable, "stdin is unavailable", nil)
	}

	limit := source.MaxBytes
	probeOverLimit := source.MaxBytes < math.MaxInt64
	if probeOverLimit {
		limit++
	}
	data, err := io.ReadAll(io.LimitReader(source.Stdin, limit))
	if err != nil {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputUnreadable, "unable to read stdin", err)
	}
	if len(data) == 0 {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputEmpty, "input is empty", nil)
	}
	if probeOverLimit && int64(len(data)) > source.MaxBytes {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputTooLarge, "input exceeds the configured limit", nil)
	}
	if !utf8.Valid(data) {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputInvalidUTF8, "input is not valid UTF-8", nil)
	}
	return string(data), nil
}

func checkStdinUnused(stdin io.Reader) error {
	if stdin == nil {
		return nil
	}
	var one [1]byte
	n, err := stdin.Read(one[:])
	if n > 0 {
		return apperr.New(apperr.KindUsage, apperr.CodeConflictingInput, "message flag conflicts with stdin input", nil)
	}
	if err != nil && !errors.Is(err, io.EOF) {
		return apperr.New(apperr.KindInput, apperr.CodeInputUnreadable, "unable to inspect stdin", err)
	}
	return nil
}

func validateMessage(message string, maxBytes int64) (string, error) {
	data := []byte(message)
	if len(data) == 0 {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputEmpty, "input is empty", nil)
	}
	if int64(len(data)) > maxBytes {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputTooLarge, "input exceeds the configured limit", nil)
	}
	if !utf8.Valid(data) {
		return "", apperr.New(apperr.KindInput, apperr.CodeInputInvalidUTF8, "input is not valid UTF-8", nil)
	}
	return message, nil
}
