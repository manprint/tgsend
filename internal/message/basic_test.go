package message

import (
	"errors"
	"strings"
	"testing"

	"github.com/manprint/tgsend/internal/apperr"
)

func TestBasicPlannerPreservesRawText(t *testing.T) {
	const body = "  first\r\nsecond\n"
	chunks, err := (BasicPlanner{}).Plan(body, true)
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(chunks) != 1 || chunks[0].Text != body || len(chunks[0].Entities) != 0 || chunks[0].Entities == nil || !chunks[0].DisableNotification {
		t.Fatalf("Plan() = %#v", chunks)
	}
}

func TestBasicPlannerCountsAstralUTF16(t *testing.T) {
	chunks, err := (BasicPlanner{}).Plan(strings.Repeat("😀", 2048), false)
	if err != nil || len(chunks) != 1 {
		t.Fatalf("Plan(exact UTF-16 limit) = %#v, %v", chunks, err)
	}
	_, err = (BasicPlanner{}).Plan(strings.Repeat("😀", 2049), false)
	if err == nil {
		t.Fatal("Plan(over UTF-16 limit) error = nil")
	}
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Code != apperr.CodeInputTooLarge {
		t.Fatalf("Plan(over UTF-16 limit) error = %v", err)
	}
}
