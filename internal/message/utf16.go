package message

import (
	"errors"
	"unicode/utf8"
)

var (
	errInvalidUTF8      = errors.New("invalid UTF-8")
	errNegativeBudget   = errors.New("UTF-16 budget must not be negative")
	errInvalidByteIndex = errors.New("byte index is not a UTF-8 rune boundary")
	errIntegerOverflow  = errors.New("integer overflow")
)

// utf16Len returns the number of UTF-16 code units in s.
func utf16Len(s string) (int, error) {
	if !utf8.ValidString(s) {
		return 0, errInvalidUTF8
	}

	units := 0
	for _, r := range s {
		var addErr error
		units, addErr = checkedAdd(units, runeUTF16Units(r))
		if addErr != nil {
			return 0, addErr
		}
	}
	return units, nil
}

// prefixWithin returns the largest complete-rune prefix that fits budget
// UTF-16 code units. byteEnd is a byte index into s.
func prefixWithin(s string, budget int) (byteEnd, units int, err error) {
	if budget < 0 {
		return 0, 0, errNegativeBudget
	}
	if !utf8.ValidString(s) {
		return 0, 0, errInvalidUTF8
	}

	for byteStart, r := range s {
		runeUnits := runeUTF16Units(r)
		nextUnits, addErr := checkedAdd(units, runeUnits)
		if addErr != nil {
			return 0, 0, addErr
		}
		if nextUnits > budget {
			break
		}

		runeEnd, addErr := checkedAdd(byteStart, utf8.RuneLen(r))
		if addErr != nil {
			return 0, 0, addErr
		}
		byteEnd = runeEnd
		units = nextUnits
	}
	return byteEnd, units, nil
}

// utf16Offset converts a UTF-8 byte boundary to a UTF-16 code-unit offset.
func utf16Offset(s string, byteIndex int) (int, error) {
	if !utf8.ValidString(s) {
		return 0, errInvalidUTF8
	}
	if byteIndex < 0 || byteIndex > len(s) {
		return 0, errInvalidByteIndex
	}
	if byteIndex == 0 {
		return 0, nil
	}

	units := 0
	for byteStart, r := range s {
		if byteStart == byteIndex {
			return units, nil
		}
		runeEnd, addErr := checkedAdd(byteStart, utf8.RuneLen(r))
		if addErr != nil {
			return 0, addErr
		}
		if byteIndex < runeEnd {
			return 0, errInvalidByteIndex
		}
		units, addErr = checkedAdd(units, runeUTF16Units(r))
		if addErr != nil {
			return 0, addErr
		}
	}
	if byteIndex == len(s) {
		return units, nil
	}
	return 0, errInvalidByteIndex
}

func runeUTF16Units(r rune) int {
	if r > 0xffff {
		return 2
	}
	return 1
}

func checkedAdd(a, b int) (int, error) {
	maxInt := int(^uint(0) >> 1)
	if b > 0 && a > maxInt-b {
		return 0, errIntegerOverflow
	}
	if b < 0 && a < -maxInt-1-b {
		return 0, errIntegerOverflow
	}
	return a + b, nil
}
