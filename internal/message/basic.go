package message

import (
	"unicode/utf16"

	"github.com/manprint/tgsend/internal/apperr"
)

const basicMessageLimit = 4096

// BasicPlanner creates one raw-text chunk for phase-one dry-runs.
type BasicPlanner struct{}

// Plan preserves raw text and measures Telegram's limit in UTF-16 code units.
func (BasicPlanner) Plan(body string, silent bool) ([]Chunk, error) {
	if len(utf16.Encode([]rune(body))) > basicMessageLimit {
		return nil, apperr.New(apperr.KindInput, apperr.CodeInputTooLarge, "message exceeds the Telegram message limit", nil)
	}
	return []Chunk{{
		Text:                body,
		Entities:            []Entity{},
		DisableNotification: silent,
	}}, nil
}
