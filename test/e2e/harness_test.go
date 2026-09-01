//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/manprint/tgsend/internal/testutil"
)

type processResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
	Err      error
	TimedOut bool
}

func run(t *testing.T, args []string, stdin []byte, overrides map[string]string, timeout time.Duration) processResult {
	t.Helper()
	return runWithHome(t, args, stdin, overrides, t.TempDir(), timeout)
}

func runWithHome(t *testing.T, args []string, stdin []byte, overrides map[string]string, home string, timeout time.Duration) processResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	env := isolatedEnvironment(home, overrides)
	return runCommand(ctx, binaryPath, args, stdin, env)
}

func isolatedEnvironment(home string, overrides map[string]string) []string {
	values := map[string]string{
		"HOME":   home,
		"PATH":   os.Getenv("PATH"),
		"LC_ALL": "C",
		"LANG":   "C",
	}
	if runtime.GOOS == "windows" {
		values["USERPROFILE"] = home
		for _, key := range []string{"SYSTEMROOT", "WINDIR", "PATHEXT"} {
			if value, ok := os.LookupEnv(key); ok {
				values[key] = value
			}
		}
	}
	for key, value := range overrides {
		values[key] = value
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	env := make([]string, 0, len(keys))
	for _, key := range keys {
		env = append(env, key+"="+values[key])
	}
	return env
}

func runCommand(ctx context.Context, path string, args []string, stdin []byte, env []string) processResult {
	command := exec.CommandContext(ctx, path, args...)
	command.Env = env
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	result := processResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: commandExitCode(err),
		Err:      err,
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.TimedOut = true
		result.ExitCode = -1
	}
	return result
}

func commandExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return -1
}

func TestDecodeOneJSONRejectsConcatenatedDocuments(t *testing.T) {
	var target map[string]any
	if err := testutil.DecodeOneJSON([]byte("{}\n{}\n"), &target); err == nil {
		t.Fatal("concatenated JSON documents were accepted")
	}
}

func TestDecodeOneJSONRequiresTrailingNewline(t *testing.T) {
	var target map[string]any
	if err := testutil.DecodeOneJSON([]byte("{}"), &target); err == nil {
		t.Fatal("JSON without a trailing newline was accepted")
	}
}

func TestExitCodeExtraction(t *testing.T) {
	path, args := "sh", []string{"-c", "exit 7"}
	if runtime.GOOS == "windows" {
		path, args = "cmd", []string{"/C", "exit 7"}
	}
	result := runCommand(context.Background(), path, args, nil, []string{"PATH=" + os.Getenv("PATH")})
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", result.ExitCode)
	}
}

func TestDeadlineCancellation(t *testing.T) {
	t.Setenv("TGSEND_E2E_HELPER", "1")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := runCommand(ctx, os.Args[0], []string{"-test.run=TestE2EHelperProcess", "--"}, nil, []string{
		"PATH=" + os.Getenv("PATH"),
		"TGSEND_E2E_HELPER=1",
	})
	if !result.TimedOut {
		t.Fatalf("TimedOut = false, result: %+v", result)
	}
	if result.ExitCode != -1 {
		t.Fatalf("exit code = %d, want -1 for timeout", result.ExitCode)
	}
	if strings.Contains(string(result.Stderr), "secret") {
		t.Fatal("timeout output unexpectedly contains secret text")
	}
}

func TestE2EHelperProcess(t *testing.T) {
	if os.Getenv("TGSEND_E2E_HELPER") != "1" {
		return
	}
	time.Sleep(time.Second)
}

func TestRunCommandReplacesEnvironment(t *testing.T) {
	t.Setenv("TGSEND_ENV_SENTINEL", "must-not-inherit")
	path, args := "sh", []string{"-c", "test -z \"$TGSEND_ENV_SENTINEL\""}
	if runtime.GOOS == "windows" {
		path, args = "cmd", []string{"/C", "if defined TGSEND_ENV_SENTINEL exit /b 1"}
	}
	result := runCommand(context.Background(), path, args, nil, []string{"PATH=" + os.Getenv("PATH")})
	if result.ExitCode != 0 {
		t.Fatalf("environment replacement exit code = %d, stderr = %q", result.ExitCode, result.Stderr)
	}
}
