package message

import (
	"errors"
	"strings"
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

		cut := byteEnd
		if lineFeed := strings.LastIndexByte(remainder[:byteEnd], '\n'); lineFeed >= 0 {
			var addErr error
			cut, addErr = checkedAdd(lineFeed, 1)
			if addErr != nil {
				return nil, addErr
			}
		}
		if cut <= 0 || cut > len(remainder) {
			return nil, errSplitNoProgress
		}

		segments = append(segments, remainder[:cut])
		remainder = remainder[cut:]
		budget = laterBudget
	}
}
