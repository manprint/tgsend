package main

import (
	"errors"
	"testing"

	"github.com/manprint/tgsend/internal/apperr"
)

func TestProductionRejectsTestBaseURL(t *testing.T) {
	_, err := resolveAPIBaseURL("http://127.0.0.1:8080", "false")
	var appErr *apperr.Error
	if !errors.As(err, &appErr) || appErr.Kind != apperr.KindUsage || appErr.Code != apperr.CodeInvalidArguments {
		t.Fatalf("error = %#v, want invalid arguments", err)
	}
}

func TestTestBuildAcceptsLoopbackOnly(t *testing.T) {
	for _, value := range []string{"http://127.0.0.1:8080", "http://localhost:8080", "http://[::1]:8080"} {
		t.Run(value, func(t *testing.T) {
			got, err := resolveAPIBaseURL(value, "true")
			if err != nil || got != value {
				t.Fatalf("resolveAPIBaseURL() = %q, %v; want %q", got, err, value)
			}
		})
	}
	for _, value := range []string{
		"https://127.0.0.1:8080",
		"http://example.com:8080",
		"http://user:pass@127.0.0.1:8080",
		"http://127.0.0.1:8080?token=secret",
	} {
		t.Run("reject-"+value, func(t *testing.T) {
			if _, err := resolveAPIBaseURL(value, "true"); err == nil {
				t.Fatalf("resolveAPIBaseURL(%q) accepted a non-loopback test endpoint", value)
			}
		})
	}
}
