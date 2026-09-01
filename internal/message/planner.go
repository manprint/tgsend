package message

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/manprint/tgsend/internal/apperr"
)

const (
	messageLimit        = 4096
	maxHeaderWithBreaks = 4094
)

var (
	errUnknownType  = errors.New("unknown message type")
	errEntityBounds = errors.New("message entity is outside the message text")
	errEntityShape  = errors.New("message entity must have a positive UTF-16 length")
)

// Options controls the optional header and entity formatting of a message.
type Options struct {
	Title     string
	Type      string
	Monospace bool
	Silent    bool
}

// Planner composes exact Telegram message chunks without performing I/O.
type Planner struct{}

var messageTypes = map[string]string{
	"INFO":     "ℹ️",
	"WARNING":  "⚠️",
	"ERROR":    "❌",
	"CRITICAL": "🚨",
}

// Plan validates options, composes the header, and splits the complete body
// before returning any chunk to a future sender.
func (Planner) Plan(body string, options Options) ([]Chunk, error) {
	header, titleStart, titleUnits, err := composeHeader(options)
	if err != nil {
		return nil, err
	}

	if !utf8.ValidString(body) {
		return nil, errInvalidUTF8
	}
	segments, err := splitBody(body, messageLimit-headerUnits(header), messageLimit)
	if err != nil {
		return nil, err
	}

	chunks := make([]Chunk, 0, len(segments))
	for index, segment := range segments {
		text := segment
		bodyStart := 0
		entities := make([]Entity, 0, 2)
		if index == 0 && header != "" {
			text = header + segment
			bodyStart = len(header)
			if options.Title != "" {
				titleOffset, offsetErr := utf16Offset(text, titleStart)
				if offsetErr != nil {
					return nil, offsetErr
				}
				entities = append(entities, Entity{Type: "bold", Offset: titleOffset, Length: titleUnits})
			}
		}
		if options.Monospace {
			bodyOffset, offsetErr := utf16Offset(text, bodyStart)
			if offsetErr != nil {
				return nil, offsetErr
			}
			bodyUnits, unitsErr := utf16Len(segment)
			if unitsErr != nil {
				return nil, unitsErr
			}
			entities = append(entities, Entity{Type: "pre", Offset: bodyOffset, Length: bodyUnits})
		}

		sort.Slice(entities, func(left, right int) bool {
			if entities[left].Offset != entities[right].Offset {
				return entities[left].Offset < entities[right].Offset
			}
			return entities[left].Length < entities[right].Length
		})
		if err := validateEntities(text, entities); err != nil {
			return nil, err
		}
		chunks = append(chunks, Chunk{
			Text:                text,
			Entities:            entities,
			DisableNotification: options.Silent,
		})
	}
	return chunks, nil
}

func composeHeader(options Options) (header string, titleStart, titleUnits int, err error) {
	if !utf8.ValidString(options.Title) || !utf8.ValidString(options.Type) {
		return "", 0, 0, apperr.New(apperr.KindUsage, apperr.CodeInvalidFlag, "title and type must be valid UTF-8", nil)
	}

	typeName := strings.ToUpper(options.Type)
	if typeName != "" {
		prefix, ok := messageTypes[typeName]
		if !ok {
			return "", 0, 0, apperr.New(apperr.KindUsage, apperr.CodeInvalidFlag, "type must be INFO, WARNING, ERROR, or CRITICAL", errUnknownType)
		}
		typeLine := prefix + " " + typeName
		if options.Title != "" {
			header = typeLine + "\n" + options.Title + "\n\n"
			titleStart = len(typeLine) + 1
		} else {
			header = typeLine + "\n\n"
		}
	} else if options.Title != "" {
		header = options.Title + "\n\n"
		titleStart = 0
	}

	if options.Title != "" {
		titleUnits, err = utf16Len(options.Title)
		if err != nil {
			return "", 0, 0, err
		}
	}
	if headerUnits(header) > maxHeaderWithBreaks {
		return "", 0, 0, apperr.New(apperr.KindUsage, apperr.CodeTitleTooLong, "title and header exceed the Telegram message limit", nil)
	}
	return header, titleStart, titleUnits, nil
}

func headerUnits(header string) int {
	if header == "" {
		return 0
	}
	units, err := utf16Len(header)
	if err != nil {
		return 0
	}
	return units
}

func validateEntities(text string, entities []Entity) error {
	textUnits, err := utf16Len(text)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		if entity.Offset < 0 || entity.Length <= 0 {
			return errEntityShape
		}
		end, addErr := checkedAdd(entity.Offset, entity.Length)
		if addErr != nil || end > textUnits {
			return errEntityBounds
		}
	}
	return nil
}
