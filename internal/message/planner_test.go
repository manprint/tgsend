package message

import (
	"errors"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/manprint/tgsend/internal/apperr"
)

func TestHeaderShapes(t *testing.T) {
	cases := []struct {
		name    string
		options Options
		want    string
	}{
		{name: "neither", want: "body"},
		{name: "title", options: Options{Title: "Deploy"}, want: "Deploy\n\nbody"},
		{name: "type", options: Options{Type: "WARNING"}, want: "⚠️ WARNING\n\nbody"},
		{name: "both", options: Options{Title: "Deploy", Type: "WARNING"}, want: "⚠️ WARNING\nDeploy\n\nbody"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			chunks, err := (Planner{}).Plan("body", testCase.options)
			if err != nil {
				t.Fatalf("Plan() error = %v", err)
			}
			if len(chunks) != 1 || chunks[0].Text != testCase.want {
				t.Fatalf("Plan() = %#v; want text %q", chunks, testCase.want)
			}
		})
	}
}

func TestSeverityCaseNormalizationAndCodePoints(t *testing.T) {
	cases := map[string]string{
		"info":     "ℹ️ INFO",
		"warning":  "⚠️ WARNING",
		"error":    "❌ ERROR",
		"critical": "🚨 CRITICAL",
	}
	for input, wantLine := range cases {
		chunks, err := (Planner{}).Plan("body", Options{Type: input})
		if err != nil {
			t.Fatalf("Plan(type %q) error = %v", input, err)
		}
		if got := strings.Split(chunks[0].Text, "\n\n")[0]; got != wantLine {
			t.Errorf("Plan(type %q) header = %q; want %q", input, got, wantLine)
		}
	}
}

func TestUnknownSeverityRejected(t *testing.T) {
	_, err := (Planner{}).Plan("body", Options{Type: "NOTICE"})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.KindUsage || appErr.Code != apperr.CodeInvalidFlag || appErr.Message == "" {
		t.Fatalf("Plan(unknown type) error = %v; want safe invalid_flag usage error", err)
	}
}

func TestTitleBoldUTF16Offset(t *testing.T) {
	const title = "😀 Deploy"
	chunks, err := (Planner{}).Plan("body", Options{Title: title, Type: "INFO"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(chunks) != 1 || len(chunks[0].Entities) != 1 {
		t.Fatalf("Plan() entities = %#v", chunks)
	}
	entity := chunks[0].Entities[0]
	wantOffset := 2 + 1 + len("INFO") + 1
	wantLength, _ := utf16Len(title)
	if entity != (Entity{Type: "bold", Offset: wantOffset, Length: wantLength}) {
		t.Fatalf("title entity = %#v; want %#v", entity, Entity{Type: "bold", Offset: wantOffset, Length: wantLength})
	}
}

func TestPreCoversBodyOnly(t *testing.T) {
	const body = "😀 body"
	chunks, err := (Planner{}).Plan(body, Options{Title: "Deploy", Type: "WARNING", Monospace: true})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(chunks) != 1 || len(chunks[0].Entities) != 2 {
		t.Fatalf("Plan() entities = %#v", chunks)
	}
	titleUnits, _ := utf16Len("Deploy")
	headerUnits, _ := utf16Len("⚠️ WARNING\nDeploy\n\n")
	bodyUnits, _ := utf16Len(body)
	want := []Entity{
		{Type: "bold", Offset: headerUnits - 2 - titleUnits, Length: titleUnits},
		{Type: "pre", Offset: headerUnits, Length: bodyUnits},
	}
	if !reflect.DeepEqual(chunks[0].Entities, want) {
		t.Fatalf("entities = %#v; want %#v", chunks[0].Entities, want)
	}
}

func TestEntitiesDoNotOverlap(t *testing.T) {
	chunks, err := (Planner{}).Plan("body", Options{Title: "Deploy", Type: "ERROR", Monospace: true})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	entities := chunks[0].Entities
	for index := 1; index < len(entities); index++ {
		previousEnd := entities[index-1].Offset + entities[index-1].Length
		if previousEnd > entities[index].Offset {
			t.Fatalf("entities overlap: %#v", entities)
		}
	}
}

func TestHeaderOnlyFirstChunk(t *testing.T) {
	chunks, err := (Planner{}).Plan(strings.Repeat("x", 5000), Options{Title: "Deploy", Type: "INFO"})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("Plan() returned %d chunks; want multiple", len(chunks))
	}
	if !strings.HasPrefix(chunks[0].Text, "ℹ️ INFO\nDeploy\n\n") {
		t.Fatalf("first chunk = %q; missing header", chunks[0].Text[:min(len(chunks[0].Text), 40)])
	}
	for index, chunk := range chunks[1:] {
		if strings.Contains(chunk.Text, "ℹ️ INFO\nDeploy\n\n") {
			t.Fatalf("chunk %d unexpectedly contains header", index+2)
		}
	}
}

func TestHeaderReservesBodyRune(t *testing.T) {
	title := strings.Repeat("t", 4092)
	chunks, err := (Planner{}).Plan("😀x", Options{Title: title})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(chunks) != 2 || !strings.HasSuffix(chunks[0].Text, "😀") || chunks[1].Text != "x" {
		t.Fatalf("Plan() = %d chunks, suffixes %q/%q; want astral rune reserved in first chunk", len(chunks), chunks[0].Text[len(chunks[0].Text)-4:], chunks[1].Text)
	}
}

func TestTitleTooLongRejected(t *testing.T) {
	_, err := (Planner{}).Plan("body", Options{Title: strings.Repeat("t", 4093)})
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeTitleTooLong || appErr.Kind != apperr.KindUsage {
		t.Fatalf("Plan(long title) error = %v; want title_too_long usage error", err)
	}
}

func TestPlannerNoChunkLabels(t *testing.T) {
	chunks, err := (Planner{}).Plan(strings.Repeat("message ", 700), Options{})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	for _, chunk := range chunks {
		if strings.Contains(chunk.Text, "chunk ") {
			t.Fatalf("planner synthesized a chunk label in %q", chunk.Text)
		}
	}
}

func TestPlannerRawPathMatchesPhase1Golden(t *testing.T) {
	const body = "  first\r\nsecond\n"
	chunks, err := (Planner{}).Plan(body, Options{Silent: true})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	want := []Chunk{{Text: body, Entities: []Entity{}, DisableNotification: true}}
	if !reflect.DeepEqual(chunks, want) {
		t.Fatalf("Plan(raw) = %#v; want %#v", chunks, want)
	}
}

func TestPlannerPlansEntireInputBeforeReturn(t *testing.T) {
	body := strings.Repeat("😀", 5000)
	first, firstErr := (Planner{}).Plan(body, Options{Monospace: true})
	second, secondErr := (Planner{}).Plan(body, Options{Monospace: true})
	if firstErr != nil || secondErr != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("repeated complete planning differs: first %d/%v, second %d/%v", len(first), firstErr, len(second), secondErr)
	}
	if len(first) < 3 {
		t.Fatalf("Plan() returned %d chunks; want the complete long body plan", len(first))
	}
}

func TestEveryEntityInBounds(t *testing.T) {
	chunks, err := (Planner{}).Plan("😀 body", Options{Title: "🚀 Deploy", Type: "CRITICAL", Monospace: true})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	for index, chunk := range chunks {
		units, unitsErr := utf16Len(chunk.Text)
		if unitsErr != nil || units > messageLimit {
			t.Fatalf("chunk %d uses %d UTF-16 units, error = %v", index, units, unitsErr)
		}
		if err := validateEntities(chunk.Text, chunk.Entities); err != nil {
			t.Fatalf("chunk %d entities invalid: %v", index, err)
		}
	}
}

func FuzzPlannerInvariants(f *testing.F) {
	f.Add("first\nsecond", false)
	f.Add("😀"+strings.Repeat("x", 5000), true)
	f.Add("e\u0301\r\nnext", false)

	f.Fuzz(func(t *testing.T, body string, monospace bool) {
		chunks, err := (Planner{}).Plan(body, Options{Monospace: monospace})
		if !utf8.ValidString(body) {
			if !errors.Is(err, errInvalidUTF8) {
				t.Fatalf("Plan(invalid) error = %v; want invalid UTF-8", err)
			}
			return
		}
		if body == "" {
			if !errors.Is(err, errEmptyBody) {
				t.Fatalf("Plan(empty) error = %v; want empty body", err)
			}
			return
		}
		if err != nil {
			t.Fatalf("Plan(%q) error = %v", body, err)
		}
		if len(chunks) == 0 || strings.Join(chunkTexts(chunks), "") != body {
			t.Fatalf("chunks do not reconstruct body %q", body)
		}
		for index, chunk := range chunks {
			units, unitsErr := utf16Len(chunk.Text)
			if unitsErr != nil || units > messageLimit {
				t.Fatalf("chunk %d exceeds UTF-16 bound: %d/%v", index, units, unitsErr)
			}
			if err := validateEntities(chunk.Text, chunk.Entities); err != nil {
				t.Fatalf("chunk %d entities invalid: %v", index, err)
			}
		}
		again, againErr := (Planner{}).Plan(body, Options{Monospace: monospace})
		if againErr != nil || !reflect.DeepEqual(chunks, again) {
			t.Fatalf("Plan() is not deterministic: %v/%#v vs %v/%#v", err, chunks, againErr, again)
		}
	})
}

func chunkTexts(chunks []Chunk) []string {
	texts := make([]string, len(chunks))
	for index, chunk := range chunks {
		texts[index] = chunk.Text
	}
	return texts
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}
