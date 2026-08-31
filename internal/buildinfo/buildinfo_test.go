package buildinfo

import "testing"

func TestCurrentDefaults(t *testing.T) {
	got := Current()
	if got.Version != "dev" || got.Commit != "none" || got.Date != "unknown" {
		t.Fatalf("Current() = %#v, want dev/none/unknown", got)
	}
}

func TestCurrentReflectsLinkVariables(t *testing.T) {
	originalVersion, originalCommit, originalDate := version, commit, date
	t.Cleanup(func() {
		version, commit, date = originalVersion, originalCommit, originalDate
	})

	version, commit, date = "1.2.3", "abc123", "2026-08-31T23:49:00Z"
	got := Current()
	if got.Version != version || got.Commit != commit || got.Date != date {
		t.Fatalf("Current() = %#v, want linked values", got)
	}
}
