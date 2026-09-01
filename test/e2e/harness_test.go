//go:build e2e

package e2e

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
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
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	env := []string{
		"HOME=" + t.TempDir(),
		"PATH=" + os.Getenv("PATH"),
		"LC_ALL=C",
		"LANG=C",
	}
	for key, value := range overrides {
		env = append(env, key+"="+value)
	}
	return runCommand(ctx, binaryPath, args, stdin, env)
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
	result := runCommand(context.Background(), "sh", []string{"-c", "exit 7"}, nil, []string{"PATH=" + os.Getenv("PATH")})
	if result.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", result.ExitCode)
	}
}

func TestDeadlineCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	result := runCommand(ctx, "sh", []string{"-c", "sleep 1"}, nil, []string{"PATH=" + os.Getenv("PATH")})
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
