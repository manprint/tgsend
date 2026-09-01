// Package presenter owns the user-facing JSON envelope and its stream rules.
package presenter

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/buildinfo"
)

const schemaVersion = "1"

// Envelope is the stable top-level JSON document. Field order is intentional.
type Envelope struct {
	SchemaVersion string     `json:"schema_version"`
	OK            bool       `json:"ok"`
	Command       string     `json:"command"`
	Result        any        `json:"result,omitempty"`
	Error         *ErrorBody `json:"error,omitempty"`
}

// VersionResult contains the linkable build metadata.
type VersionResult struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// ErrorBody is the safe, machine-readable error payload.
type ErrorBody struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// Encode writes exactly one JSON document and its terminating newline.
func Encode(w io.Writer, envelope Envelope) error {
	return json.NewEncoder(w).Encode(envelope)
}

// WriteVersion writes the version command response.
func WriteVersion(w io.Writer, info buildinfo.Info) error {
	return Encode(w, Envelope{
		SchemaVersion: schemaVersion,
		OK:            true,
		Command:       "version",
		Result: VersionResult{
			Version: info.Version,
			Commit:  info.Commit,
			Date:    info.Date,
		},
	})
}

// WriteError writes one safe error response to the selected stream.
func WriteError(w io.Writer, command string, body ErrorBody) error {
	return Encode(w, Envelope{
		SchemaVersion: schemaVersion,
		OK:            false,
		Command:       command,
		Error:         &body,
	})
}

// ErrorBodyFrom converts an application error without serializing its cause.
func ErrorBodyFrom(err error) ErrorBody {
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr == nil {
		return ErrorBody{
			Code:      "internal_error",
			Message:   "internal error",
			Retryable: false,
		}
	}
	return ErrorBody{
		Code:      string(appErr.Code),
		Message:   appErr.Message,
		Retryable: appErr.Kind == apperr.KindRateLimit,
	}
}
