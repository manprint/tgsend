package message

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestSplitNoopWithinBudget(t *testing.T) {
	const body = "first\r\nsecond"
	got, err := splitBody(body, 4096, 4096)
	if err != nil || !reflect.DeepEqual(got, []string{body}) {
		t.Fatalf("splitBody() = %#v, %v; want one unchanged segment", got, err)
	}
}

func TestSplitAtLastFittingLF(t *testing.T) {
	got, err := splitBody("one\n1234567890\nend", 10, 4096)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	want := []string{"one\n", "1234567890\nend"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBody() = %#v; want %#v", got, want)
	}
}

func TestSplitPreservesCRLF(t *testing.T) {
	got, err := splitBody("a\r\nb", 3, 4096)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	want := []string{"a\r\n", "b"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBody() = %#v; want %#v", got, want)
	}
}

func TestSplitFallsBackAtRuneBoundary(t *testing.T) {
	got, err := splitBody("😀abc", 2, 2)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	want := []string{"😀", "ab", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBody() = %#v; want %#v", got, want)
	}
	for _, segment := range got {
		if !utf8.ValidString(segment) {
			t.Fatalf("segment %q is invalid UTF-8", segment)
		}
	}
}

func TestSplitAstralBudget(t *testing.T) {
	body := strings.Repeat("😀", 3)
	got, err := splitBody(body, 4, 4)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	want := []string{"😀😀", "😀"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBody() = %#v; want %#v", got, want)
	}
}

func TestSplitConsecutiveAndLeadingLF(t *testing.T) {
	got, err := splitBody("\n\nabc", 1, 1)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	want := []string{"\n", "\n", "a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBody() = %#v; want %#v", got, want)
	}
}

func TestSplitFinalLF(t *testing.T) {
	got, err := splitBody("123\n456\n", 5, 5)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	want := []string{"123\n", "456\n"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBody() = %#v; want %#v", got, want)
	}
}

func TestSplitUsesFirstBudgetOnce(t *testing.T) {
	got, err := splitBody("abcdefghij", 3, 5)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	want := []string{"abc", "defgh", "ij"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("splitBody() = %#v; want %#v", got, want)
	}
}

func TestSplitRejectsImpossibleBudget(t *testing.T) {
	for name, testCase := range map[string]struct {
		body        string
		firstBudget int
		laterBudget int
	}{
		"empty body":        {"", 1, 1},
		"invalid UTF-8":     {string([]byte{0xff}), 1, 1},
		"zero first":        {"body", 0, 1},
		"zero later":        {"body", 1, 0},
		"astral cannot fit": {"😀", 1, 1},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := splitBody(testCase.body, testCase.firstBudget, testCase.laterBudget); err == nil {
				t.Fatal("splitBody() error = nil; want rejection")
			}
		})
	}
}

func TestSplitEveryChunkWithinBudget(t *testing.T) {
	const firstBudget = 5
	const laterBudget = 4
	body := "ab😀cd\nefgh😀ij"
	chunks, err := splitBody(body, firstBudget, laterBudget)
	if err != nil {
		t.Fatalf("splitBody() error = %v", err)
	}
	for index, chunk := range chunks {
		units, unitsErr := utf16Len(chunk)
		if unitsErr != nil {
			t.Fatalf("utf16Len(chunk %d) error = %v", index, unitsErr)
		}
		budget := laterBudget
		if index == 0 {
			budget = firstBudget
		}
		if units > budget {
			t.Errorf("chunk %d uses %d UTF-16 units; budget = %d", index, units, budget)
		}
	}
}

func TestTMSG07UnicodeCases(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{name: "ASCII", body: "abcdef"},
		{name: "BMP", body: "città 世界"},
		{name: "astral", body: "😀🦄🐳"},
		{name: "combining", body: "e\u0301e\u0301"},
		{name: "CRLF", body: "first\r\nsecond"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			chunks, err := splitBody(testCase.body, 3, 3)
			if err != nil {
				t.Fatalf("splitBody() error = %v", err)
			}
			if strings.Join(chunks, "") != testCase.body {
				t.Fatalf("rejoined chunks = %q; want %q", strings.Join(chunks, ""), testCase.body)
			}
			for _, chunk := range chunks {
				units, unitsErr := utf16Len(chunk)
				if unitsErr != nil || units > 3 {
					t.Fatalf("chunk %q uses %d units, error = %v", chunk, units, unitsErr)
				}
			}
		})
	}
}

func FuzzSplitBodyPreservesInputAndBounds(f *testing.F) {
	f.Add("first\nsecond", uint8(4), uint8(5))
	f.Add("😀abc", uint8(3), uint8(4))
	f.Add("e\u0301\r\nnext", uint8(4), uint8(4))

	f.Fuzz(func(t *testing.T, body string, firstByte, laterByte uint8) {
		firstBudget := int(firstByte%16) + 2
		laterBudget := int(laterByte%16) + 2
		chunks, err := splitBody(body, firstBudget, laterBudget)
		if !utf8.ValidString(body) {
			if !errors.Is(err, errInvalidUTF8) {
				t.Fatalf("splitBody(invalid) error = %v; want invalid UTF-8", err)
			}
			return
		}
		if body == "" {
			if !errors.Is(err, errEmptyBody) {
				t.Fatalf("splitBody(empty) error = %v; want empty body", err)
			}
			return
		}
		if err != nil {
			t.Fatalf("splitBody(%q, %d, %d) error = %v", body, firstBudget, laterBudget, err)
		}
		if len(chunks) == 0 || strings.Join(chunks, "") != body {
			t.Fatalf("chunks = %#v; do not reconstruct body %q", chunks, body)
		}
		for index, chunk := range chunks {
			if chunk == "" || !utf8.ValidString(chunk) {
				t.Fatalf("chunk %d = %q; want non-empty valid UTF-8", index, chunk)
			}
			units, unitsErr := utf16Len(chunk)
			if unitsErr != nil {
				t.Fatalf("utf16Len(chunk %d) error = %v", index, unitsErr)
			}
			budget := laterBudget
			if index == 0 {
				budget = firstBudget
			}
			if units > budget {
				t.Fatalf("chunk %d uses %d UTF-16 units; budget = %d", index, units, budget)
			}
		}
		again, againErr := splitBody(body, firstBudget, laterBudget)
		if againErr != nil || !reflect.DeepEqual(chunks, again) {
			t.Fatalf("splitBody() is not deterministic: first %#v/%v, second %#v/%v", chunks, err, again, againErr)
		}
	})
}
