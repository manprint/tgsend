package message

import (
	"errors"
	"testing"
	"unicode/utf8"
)

func TestUTF16LenASCII(t *testing.T) {
	got, err := utf16Len("Telegram")
	if err != nil || got != 8 {
		t.Fatalf("utf16Len(ASCII) = %d, %v; want 8, nil", got, err)
	}
}

func TestUTF16LenBMP(t *testing.T) {
	got, err := utf16Len("Caffè 世界")
	if err != nil || got != 8 {
		t.Fatalf("utf16Len(BMP) = %d, %v; want 8, nil", got, err)
	}
}

func TestUTF16LenAstral(t *testing.T) {
	got, err := utf16Len("😀🦄")
	if err != nil || got != 4 {
		t.Fatalf("utf16Len(astral) = %d, %v; want 4, nil", got, err)
	}
}

func TestUTF16LenCombiningSequence(t *testing.T) {
	got, err := utf16Len("e\u0301")
	if err != nil || got != 2 {
		t.Fatalf("utf16Len(combining sequence) = %d, %v; want 2, nil", got, err)
	}
}

func TestPrefixWithinNeverSplitsRune(t *testing.T) {
	const input = "A😀B"
	for budget, want := range map[int]struct {
		byteEnd int
		units   int
	}{
		0: {0, 0},
		1: {1, 1},
		2: {1, 1},
		3: {5, 3},
		4: {6, 4},
		5: {6, 4},
	} {
		byteEnd, units, err := prefixWithin(input, budget)
		if err != nil || byteEnd != want.byteEnd || units != want.units {
			t.Errorf("prefixWithin(%q, %d) = (%d, %d, %v); want (%d, %d, nil)", input, budget, byteEnd, units, err, want.byteEnd, want.units)
		}
		if !utf8.ValidString(input[:byteEnd]) {
			t.Errorf("prefixWithin(%q, %d) returned invalid UTF-8 prefix", input, budget)
		}
	}
}

func TestPrefixWithinZero(t *testing.T) {
	byteEnd, units, err := prefixWithin("😀", 0)
	if err != nil || byteEnd != 0 || units != 0 {
		t.Fatalf("prefixWithin(astral, 0) = (%d, %d, %v); want (0, 0, nil)", byteEnd, units, err)
	}
}

func TestOffsetRejectsMidRune(t *testing.T) {
	const input = "A😀B"
	for _, byteIndex := range []int{2, 3, 4} {
		if _, err := utf16Offset(input, byteIndex); !errors.Is(err, errInvalidByteIndex) {
			t.Errorf("utf16Offset(%q, %d) error = %v; want invalid byte index", input, byteIndex, err)
		}
	}
	if got, err := utf16Offset(input, 5); err != nil || got != 3 {
		t.Fatalf("utf16Offset(%q, 5) = (%d, %v); want (3, nil)", input, got, err)
	}
	if got, err := utf16Offset(input, len(input)); err != nil || got != 4 {
		t.Fatalf("utf16Offset(%q, len) = (%d, %v); want (4, nil)", input, got, err)
	}
}

func TestInvalidUTF8Rejected(t *testing.T) {
	invalid := string([]byte{'a', 0xff, 'b'})
	if _, err := utf16Len(invalid); !errors.Is(err, errInvalidUTF8) {
		t.Errorf("utf16Len(invalid) error = %v; want invalid UTF-8", err)
	}
	if _, _, err := prefixWithin(invalid, 3); !errors.Is(err, errInvalidUTF8) {
		t.Errorf("prefixWithin(invalid) error = %v; want invalid UTF-8", err)
	}
	if _, err := utf16Offset(invalid, 1); !errors.Is(err, errInvalidUTF8) {
		t.Errorf("utf16Offset(invalid) error = %v; want invalid UTF-8", err)
	}
}

func TestCheckedAddOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	if _, err := checkedAdd(maxInt, 1); !errors.Is(err, errIntegerOverflow) {
		t.Fatalf("checkedAdd(maxInt, 1) error = %v; want integer overflow", err)
	}
	if got, err := checkedAdd(40, 2); err != nil || got != 42 {
		t.Fatalf("checkedAdd(40, 2) = (%d, %v); want (42, nil)", got, err)
	}
}

func FuzzPrefixWithin(f *testing.F) {
	f.Add("A😀B", uint8(3))
	f.Add("line\r\nsecond", uint8(7))
	f.Add("e\u0301", uint8(1))

	f.Fuzz(func(t *testing.T, input string, budget uint8) {
		if !utf8.ValidString(input) {
			if _, _, err := prefixWithin(input, int(budget)); !errors.Is(err, errInvalidUTF8) {
				t.Fatalf("prefixWithin(invalid input) error = %v; want invalid UTF-8", err)
			}
			return
		}

		byteEnd, units, err := prefixWithin(input, int(budget))
		if err != nil {
			t.Fatalf("prefixWithin(%q, %d) error = %v", input, budget, err)
		}
		if byteEnd < 0 || byteEnd > len(input) || !utf8.ValidString(input[:byteEnd]) {
			t.Fatalf("prefixWithin(%q, %d) returned invalid prefix end %d", input, budget, byteEnd)
		}
		if units > int(budget) {
			t.Fatalf("prefixWithin(%q, %d) units = %d", input, budget, units)
		}
		measured, measureErr := utf16Len(input[:byteEnd])
		if measureErr != nil || measured != units {
			t.Fatalf("utf16Len(prefix) = (%d, %v); want (%d, nil)", measured, measureErr, units)
		}
		if byteEnd < len(input) {
			r, _ := utf8.DecodeRuneInString(input[byteEnd:])
			if units+runeUTF16Units(r) <= int(budget) {
				t.Fatalf("prefixWithin(%q, %d) was not maximal: next rune would fit", input, budget)
			}
		}
	})
}
