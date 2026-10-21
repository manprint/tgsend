package message

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	errEmptyBody         = errors.New("message body must not be empty")
	errInvalidSplitLimit = errors.New("message split budgets must be positive")
	errSplitNoProgress   = errors.New("message splitting made no progress")
)

// splitBody divides body into byte-exact UTF-8 segments bounded by UTF-16
// budgets. A fitting line feed is preferred over the hard boundary.
func splitBody(body string, firstBudget, laterBudget int) ([]string, error) {
	if body == "" {
		return nil, errEmptyBody
	}
	if !utf8.ValidString(body) {
		return nil, errInvalidUTF8
	}
	if firstBudget <= 0 || laterBudget <= 0 {
		return nil, errInvalidSplitLimit
	}

	segments := make([]string, 0, 1)
	remainder := body
	budget := firstBudget
	for {
		byteEnd, _, err := prefixWithin(remainder, budget)
		if err != nil {
			return nil, err
		}
		if byteEnd == len(remainder) {
			segments = append(segments, remainder)
			return segments, nil
		}
		if byteEnd == 0 {
			return nil, errSplitNoProgress
		}

		cut, err := visibleCut(remainder, byteEnd)
		if err != nil {
			return nil, err
		}
		if cut <= 0 || cut > len(remainder) {
			return nil, errSplitNoProgress
		}

		segments = append(segments, remainder[:cut])
		remainder = remainder[cut:]
		budget = laterBudget
	}
}

// visibleCut chooses where to end the next segment of remainder, given that
// remainder[:byteEnd] is the longest prefix within budget. Telegram rejects a
// message that is only whitespace, so the cut keeps visible content on both
// sides whenever the text allows it: the last fitting line feed is preferred,
// then the largest rune boundary. Only when no such cut exists does it fall
// back to the plain rule, and the planner rejects any blank chunk.
func visibleCut(remainder string, byteEnd int) (int, error) {
	firstVisible := strings.IndexFunc(remainder, isVisible)
	lastVisible := strings.LastIndexFunc(remainder, isVisible)
	if firstVisible >= 0 && firstVisible < lastVisible {
		limit := min(byteEnd, lastVisible)
		if lineFeed := strings.LastIndexByte(remainder[:limit], '\n'); lineFeed > firstVisible {
			return checkedAdd(lineFeed, 1)
		}
		if limit > firstVisible {
			return limit, nil
		}
	}
	if lineFeed := strings.LastIndexByte(remainder[:byteEnd], '\n'); lineFeed >= 0 {
		return checkedAdd(lineFeed, 1)
	}
	return byteEnd, nil
}

// isVisible approximates the characters Telegram keeps when it trims a message.
func isVisible(r rune) bool {
	return !unicode.IsSpace(r)
}

func isBlank(s string) bool {
	return strings.IndexFunc(s, isVisible) < 0
}
