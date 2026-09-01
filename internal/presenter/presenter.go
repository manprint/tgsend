// Package presenter owns the user-facing JSON envelope and its stream rules.
package presenter

import (
	"encoding/json"
	"errors"
	"io"

	"github.com/manprint/tgsend/internal/apperr"
	"github.com/manprint/tgsend/internal/buildinfo"
	"github.com/manprint/tgsend/internal/message"
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
	Code      string        `json:"code"`
	Message   string        `json:"message"`
	Retryable bool          `json:"retryable"`
	Progress  *ProgressBody `json:"progress,omitempty"`
}

// ProgressBody reports the completed portion of a partially sent plan.
type ProgressBody struct {
	ChunksTotal int `json:"chunks_total"`
	ChunksSent  int `json:"chunks_sent"`
	FailedChunk int `json:"failed_chunk"`
}

// SendResult is the result of a real send or an offline dry-run preview.
type SendResult struct {
	DryRun      bool           `json:"dry_run"`
	ChunksTotal int            `json:"chunks_total"`
	ChunksSent  int            `json:"chunks_sent"`
	MessageIDs  []int64        `json:"message_ids"`
	Chunks      []PreviewChunk `json:"chunks,omitempty"`
}

// Entity aliases the shared message entity used in previews.
type Entity = message.Entity

// PreviewChunk is one planned message in a dry-run response.
type PreviewChunk struct {
	Index               int      `json:"index"`
	Text                string   `json:"text"`
	Entities            []Entity `json:"entities"`
	DisableNotification bool     `json:"disable_notification"`
}

// MarshalJSON keeps arrays present and non-null in all serialized results.
func (result SendResult) MarshalJSON() ([]byte, error) {
	type sendResult SendResult
	if result.MessageIDs == nil {
		result.MessageIDs = []int64{}
	}
	return json.Marshal(sendResult(result))
}

// MarshalJSON keeps the preview entity array present and non-null.
func (chunk PreviewChunk) MarshalJSON() ([]byte, error) {
	type previewChunk PreviewChunk
	if chunk.Entities == nil {
		chunk.Entities = []Entity{}
	}
	return json.Marshal(previewChunk(chunk))
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

// WriteSend writes one stable send or dry-run response.
func WriteSend(w io.Writer, result SendResult) error {
	return Encode(w, Envelope{
		SchemaVersion: schemaVersion,
		OK:            true,
		Command:       "send",
		Result:        result,
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
	body := ErrorBody{
		Code:      string(appErr.Code),
		Message:   appErr.Message,
		Retryable: appErr.Kind == apperr.KindRateLimit,
	}
	if appErr.Progress != nil {
		body.Progress = &ProgressBody{
			ChunksTotal: appErr.Progress.ChunksTotal,
			ChunksSent:  appErr.Progress.ChunksSent,
			FailedChunk: appErr.Progress.FailedChunk,
		}
	}
	return body
}
