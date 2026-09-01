package workflow_test

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func readWorkflow(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repoRoot(t), ".github", "workflows", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return strings.ReplaceAll(string(data), "\r\n", "\n")
}

func parseYAML(t *testing.T, name string) map[string]any {
	t.Helper()
	var document map[string]any
	if err := yaml.Unmarshal([]byte(readWorkflow(t, name)), &document); err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	if len(document) == 0 {
		t.Fatalf("parse %s: empty document", name)
	}
	return document
}

func assertContains(t *testing.T, content, want string) {
	t.Helper()
	if !strings.Contains(content, want) {
		t.Errorf("workflow does not contain %q", want)
	}
}

func TestWorkflowYAMLAndPolicy(t *testing.T) {
	ci := readWorkflow(t, "ci.yml")
	release := readWorkflow(t, "release.yml")
	parseYAML(t, "ci.yml")
	parseYAML(t, "release.yml")

	for _, workflow := range []string{ci, release} {
		if regexp.MustCompile(`(?m)^\s*uses:.*@(main|master|latest)\s*$`).MatchString(workflow) {
			t.Error("workflow uses a floating branch or latest action reference")
		}
		for _, forbidden := range []string{"pull_request_target", "TGSEND_TOKEN", "TGSEND_CHAT_ID", "api.telegram.org", "TELEGRAM_BOT_TOKEN"} {
			if strings.Contains(workflow, forbidden) {
				t.Errorf("workflow contains forbidden live/secret value %q", forbidden)
			}
		}
	}

	for _, event := range []string{"pull_request:", "push:", "branches:\n      - main"} {
		assertContains(t, ci, event)
	}
	for _, action := range []string{
		"actions/checkout@v7.0.1",
		"actions/setup-go@v7.0.0",
		"goreleaser/goreleaser-action@v7.2.3",
		"anchore/sbom-action/download-syft@v0.24.2",
		"docker/login-action@v4.6.0",
		"docker/setup-buildx-action@v4.3.0",
	} {
		if !strings.Contains(ci+release, action) {
			t.Errorf("pinned action %q missing", action)
		}
	}
	assertContains(t, ci, "GO_VERSION: '1.27.0'")
	assertContains(t, ci, "GORELEASER_VERSION: v2.18.0")
	assertContains(t, ci, "SYFT_VERSION: v1.51.1")
	assertContains(t, ci, "permissions:\n  contents: read")
	assertContains(t, release, "tags:\n      - 'v[0-9]+.[0-9]+.[0-9]+'")
	assertContains(t, release, "needs: quality")
	assertContains(t, release, "contents: write")
	assertContains(t, release, "packages: write")
	assertContains(t, release, "password: ${{ secrets.GITHUB_TOKEN }}")
	assertContains(t, release, "args: release --clean")
}

func TestWorkflowCommandsMatchMakeTargets(t *testing.T) {
	makefile, err := os.ReadFile(filepath.Join(repoRoot(t), "Makefile"))
	if err != nil {
		t.Fatal(err)
	}
	makeText := string(makefile)
	ci := readWorkflow(t, "ci.yml")
	release := readWorkflow(t, "release.yml")
	for _, target := range []string{"fmt-check", "lint", "test", "test-e2e", "vuln", "verify", "release-check", "test-container"} {
		if !strings.Contains(makeText, target) {
			t.Errorf("Makefile target %q missing", target)
		}
	}
	for _, command := range []string{"make fmt-check", "make lint", "make test", "make test-e2e", "make vuln", "make release-check", "make test-container", "make verify"} {
		if !strings.Contains(ci+release, command) {
			t.Errorf("workflow command %q missing", command)
		}
	}
}
