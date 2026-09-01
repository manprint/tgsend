package wrapper

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

type wrapperResult struct {
	stdout   []byte
	stderr   []byte
	exitCode int
}

func TestWrapperForwardsArgumentsIncludingSpacesAndEmpty(t *testing.T) {
	home := t.TempDir()
	workdir := t.TempDir()
	want := []string{"--dry-run", "-m", "two words", "", "--title", "title with spaces"}
	result, dockerArgs, stdin := invokeWrapper(t, want, []byte("payload\n"), home, workdir, nil)
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	if string(stdin) != "payload\n" {
		t.Fatalf("stdin = %q", stdin)
	}
	assertDockerCommonArgs(t, dockerArgs, "ghcr.io/manprint/tgsend:latest", home, workdir)
	imageIndex := indexOf(t, dockerArgs, "ghcr.io/manprint/tgsend:latest")
	got := make([]string, len(dockerArgs[imageIndex+1:]))
	for index, arg := range dockerArgs[imageIndex+1:] {
		got[index] = string(arg)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forwarded args = %#v, want %#v", got, want)
	}
}

func TestWrapperForwardsStdin(t *testing.T) {
	result, _, stdin := invokeWrapper(t, []string{"--dry-run", "-m", "message"}, []byte("exact\r\nstdin\n"), t.TempDir(), t.TempDir(), nil)
	if result.exitCode != 0 || string(stdin) != "exact\r\nstdin\n" {
		t.Fatalf("exit/stdin = %d/%q", result.exitCode, stdin)
	}
}

func TestWrapperDefaultConfigMountReadOnly(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, ".tgsend")
	writeWrapperConfig(t, configPath)
	result, dockerArgs, _ := invokeWrapper(t, []string{"--dry-run", "-m", "message"}, nil, home, t.TempDir(), nil)
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	assertConfigMount(t, dockerArgs, configPath)
}

func TestWrapperExplicitConfigForms(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "explicit config.toml")
	writeWrapperConfig(t, configPath)
	cases := [][]string{
		{"-c", configPath, "--dry-run", "-m", "message"},
		{"--config", configPath, "--dry-run", "-m", "message"},
		{"--config=" + configPath, "--dry-run", "-m", "message"},
		{"-c" + configPath, "--dry-run", "-m", "message"},
	}
	for index, args := range cases {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			result, dockerArgs, _ := invokeWrapper(t, args, nil, home, t.TempDir(), nil)
			if result.exitCode != 0 {
				t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
			}
			assertConfigMount(t, dockerArgs, configPath)
		})
	}
}

func TestWrapperEnvOnlyNoMount(t *testing.T) {
	result, dockerArgs, _ := invokeWrapper(t, []string{"--dry-run", "-m", "message"}, nil, t.TempDir(), t.TempDir(), map[string]string{
		"TGSEND_TOKEN":   "123:environment-token",
		"TGSEND_CHAT_ID": "-100123",
	})
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	if containsArg(dockerArgs, "--mount") {
		t.Fatalf("env-only invocation unexpectedly mounted config: %#v", dockerArgs)
	}
}

func TestWrapperForwardsEnvNamesNotValues(t *testing.T) {
	const token = "123:sentinel-token"
	result, dockerArgs, _ := invokeWrapper(t, []string{"--dry-run", "-m", "message"}, nil, t.TempDir(), t.TempDir(), map[string]string{
		"TGSEND_TOKEN":   token,
		"TGSEND_CHAT_ID": "-100123",
	})
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	if !containsArg(dockerArgs, "TGSEND_TOKEN") || !containsArg(dockerArgs, "TGSEND_CHAT_ID") {
		t.Fatalf("environment names were not forwarded: %#v", dockerArgs)
	}
	if bytes.Contains(bytes.Join(dockerArgs, nil), []byte(token)) {
		t.Fatalf("secret value appeared in Docker argv: %#v", dockerArgs)
	}
}

func TestWrapperImageOverride(t *testing.T) {
	const image = "local/tgsend:acceptance"
	result, dockerArgs, _ := invokeWrapper(t, []string{"--dry-run", "-m", "message"}, nil, t.TempDir(), t.TempDir(), map[string]string{"TGSEND_IMAGE": image})
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	if !containsArg(dockerArgs, image) {
		t.Fatalf("image override not forwarded: %#v", dockerArgs)
	}
}

func TestWrapperNoDockerExit(t *testing.T) {
	root := projectRoot(t)
	result := runWrapperCommand(t, filepath.Join(root, "tgsend.sh"), []string{"--dry-run", "-m", "message"}, nil, []string{"PATH=" + t.TempDir(), "HOME=" + t.TempDir()})
	if result.exitCode != 127 || !bytes.Contains(result.stderr, []byte("docker is required")) {
		t.Fatalf("no-docker result = %#v", result)
	}
}

func TestWrapperMissingConfigJSONExit3(t *testing.T) {
	home := t.TempDir()
	missing := filepath.Join(home, "missing.toml")
	result, dockerArgs, _ := invokeWrapper(t, []string{"--config", missing, "-m", "message"}, nil, home, t.TempDir(), nil)
	if result.exitCode != 3 || len(result.stdout) != 0 || len(dockerArgs) != 0 {
		t.Fatalf("missing config result = %#v, docker args = %#v", result, dockerArgs)
	}
	if bytes.Count(result.stderr, []byte("\n")) != 1 {
		t.Fatalf("missing config output cardinality = %q", result.stderr)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(result.stderr, &envelope); err != nil {
		t.Fatalf("decode missing config error: %v", err)
	}
	if envelope.Error.Code != "config_not_found" {
		t.Fatalf("missing config code = %q", envelope.Error.Code)
	}
}

func TestWrapperPreservesContainerExit(t *testing.T) {
	result, _, _ := invokeWrapper(t, []string{"--dry-run", "-m", "message"}, nil, t.TempDir(), t.TempDir(), map[string]string{"FAKE_DOCKER_EXIT": "23"})
	if result.exitCode != 23 {
		t.Fatalf("container exit code = %d, want 23; stderr=%q", result.exitCode, result.stderr)
	}
}

func TestWrapperNoBashisms(t *testing.T) {
	root := projectRoot(t)
	wrapperPath := filepath.Join(root, "tgsend.sh")
	if _, err := exec.LookPath("dash"); err == nil {
		result := exec.Command("dash", "-n", wrapperPath).Run()
		if result != nil {
			t.Fatalf("dash syntax check failed: %v", result)
		}
	}
	if shellcheck, err := exec.LookPath("shellcheck"); err == nil {
		result := exec.Command(shellcheck, "-s", "sh", wrapperPath)
		output, runErr := result.CombinedOutput()
		if runErr != nil {
			t.Fatalf("shellcheck failed: %v\n%s", runErr, output)
		}
		if version := shellcheckVersion(t, shellcheck); version != "0.10.0" {
			t.Logf("ShellCheck %s available locally; CI requirement is v0.10.0", version)
		}
	}
}

func assertDockerCommonArgs(t *testing.T, args [][]byte, image, home, workdir string) {
	t.Helper()
	if len(args) < 10 || string(args[0]) != "run" || !containsArg(args, "--rm") || !containsArg(args, "-i") {
		t.Fatalf("Docker common args = %#v", args)
	}
	userIndex := indexOf(t, args, "--user")
	if !strings.Contains(string(args[userIndex+1]), ":") {
		t.Fatalf("user argument = %#v", args[userIndex:userIndex+2])
	}
	workdirIndex := indexOf(t, args, "--workdir")
	if string(args[workdirIndex+1]) != workdir {
		t.Fatalf("workdir = %q, want %q", args[workdirIndex+1], workdir)
	}
	envIndex := indexOf(t, args, "--env")
	if string(args[envIndex+1]) != "HOME="+home {
		t.Fatalf("HOME env = %q, want HOME=%q", args[envIndex+1], home)
	}
	if !containsArg(args, image) {
		t.Fatalf("image %q not found in %#v", image, args)
	}
}

func assertConfigMount(t *testing.T, args [][]byte, configPath string) {
	t.Helper()
	mountIndex := indexOf(t, args, "--mount")
	want := "type=bind,src=" + configPath + ",dst=" + configPath + ",readonly"
	if string(args[mountIndex+1]) != want {
		t.Fatalf("mount = %q, want %q; args=%#v", args[mountIndex+1], want, args)
	}
}

func invokeWrapper(t *testing.T, args []string, stdin []byte, home, workdir string, overrides map[string]string) (wrapperResult, [][]byte, []byte) {
	t.Helper()
	root := projectRoot(t)
	temp := t.TempDir()
	fakeBin := filepath.Join(temp, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	fakeScript, err := os.ReadFile(filepath.Join(root, "test", "wrapper", "testdata", "fake-docker.sh"))
	if err != nil {
		t.Fatalf("read fake Docker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "docker"), fakeScript, 0o755); err != nil {
		t.Fatalf("write fake Docker: %v", err)
	}
	argsPath := filepath.Join(temp, "args.bin")
	stdinPath := filepath.Join(temp, "stdin.bin")
	env := map[string]string{
		"PATH":              fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME":              home,
		"PWD":               workdir,
		"FAKE_DOCKER_ARGS":  argsPath,
		"FAKE_DOCKER_STDIN": stdinPath,
	}
	for key, value := range overrides {
		env[key] = value
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	processEnv := make([]string, 0, len(keys))
	for _, key := range keys {
		processEnv = append(processEnv, key+"="+env[key])
	}
	result := runWrapperCommand(t, filepath.Join(root, "tgsend.sh"), args, stdin, processEnv)
	dockerArgs := readNULArgs(t, argsPath)
	capturedStdin, err := os.ReadFile(stdinPath)
	if errors.Is(err, os.ErrNotExist) {
		capturedStdin = nil
	} else if err != nil {
		t.Fatalf("read fake Docker stdin: %v", err)
	}
	return result, dockerArgs, capturedStdin
}

func runWrapperCommand(t *testing.T, path string, args []string, stdin []byte, env []string) wrapperResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, path, args...)
	command.Env = env
	for _, value := range env {
		if strings.HasPrefix(value, "PWD=") {
			command.Dir = strings.TrimPrefix(value, "PWD=")
			break
		}
	}
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	err := command.Run()
	exitCode := 0
	if err != nil {
		exitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			exitCode = exitErr.ExitCode()
		}
	}
	return wrapperResult{stdout: stdout.Bytes(), stderr: stderr.Bytes(), exitCode: exitCode}
}

func readNULArgs(t *testing.T, path string) [][]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatalf("read fake Docker args: %v", err)
	}
	parts := bytes.Split(data, []byte{0})
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func writeWrapperConfig(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("token = \"123:file\"\nchat_id = \"-100123\"\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func projectRoot(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate wrapper test source")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(source), "..", ".."))
}

func indexOf(t *testing.T, args [][]byte, want string) int {
	t.Helper()
	for index, arg := range args {
		if string(arg) == want {
			return index
		}
	}
	t.Fatalf("argument %q not found in %#v", want, args)
	return -1
}

func containsArg(args [][]byte, want string) bool {
	for _, arg := range args {
		if string(arg) == want {
			return true
		}
	}
	return false
}

func shellcheckVersion(t *testing.T, path string) string {
	t.Helper()
	output, err := exec.Command(path, "--version").Output()
	if err != nil {
		return "unknown"
	}
	for _, line := range strings.Split(string(output), "\n") {
		if strings.HasPrefix(line, "version: ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "version: "))
		}
	}
	return "unknown"
}
