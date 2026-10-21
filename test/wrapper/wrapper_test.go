//go:build !windows

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

	"github.com/manprint/tgsend/internal/cli"
	"github.com/spf13/pflag"
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

func TestWrapperMissingExplicitConfigDefersToBinary(t *testing.T) {
	home := t.TempDir()
	missing := filepath.Join(home, "missing.toml")
	cases := [][]string{
		{"--config", missing, "-m", "message"},
		{"--dry-run", "--config", missing, "-m", "message"},
		{"--config=", "-m", "message"},
		{"-c", "", "-m", "message"},
	}
	for index, args := range cases {
		t.Run(string(rune('a'+index)), func(t *testing.T) {
			result, dockerArgs, _ := invokeWrapper(t, args, nil, home, t.TempDir(), nil)
			if result.exitCode != 0 || len(result.stderr) != 0 {
				t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
			}
			if containsArg(dockerArgs, "--mount") {
				t.Fatalf("missing config was mounted: %#v", dockerArgs)
			}
			assertForwardedArgs(t, dockerArgs, "ghcr.io/manprint/tgsend:latest", args)
		})
	}
}

func TestWrapperDirectoryConfigJSONExit3(t *testing.T) {
	home := t.TempDir()
	result, dockerArgs, _ := invokeWrapper(t, []string{"--config", home, "-m", "message"}, nil, home, t.TempDir(), nil)
	if result.exitCode != 3 || len(result.stdout) != 0 || len(dockerArgs) != 0 {
		t.Fatalf("directory config result = %#v, docker args = %#v", result, dockerArgs)
	}
	if bytes.Count(result.stderr, []byte("\n")) != 1 {
		t.Fatalf("directory config output cardinality = %q", result.stderr)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(result.stderr, &envelope); err != nil {
		t.Fatalf("decode directory config error: %v", err)
	}
	if envelope.Error.Code != "config_unreadable" {
		t.Fatalf("directory config code = %q", envelope.Error.Code)
	}
}

func TestWrapperIgnoresConfigLikeValuesOfOtherFlags(t *testing.T) {
	home := t.TempDir()
	var valueFlags []*pflag.Flag
	cli.NewRoot(cli.Dependencies{}).Flags().VisitAll(func(flag *pflag.Flag) {
		if flag.Name != "config" && flag.Value.Type() != "bool" {
			valueFlags = append(valueFlags, flag)
		}
	})
	if len(valueFlags) == 0 {
		t.Fatal("no value-taking flags found")
	}
	for _, flag := range valueFlags {
		spellings := []string{"--" + flag.Name}
		if flag.Shorthand != "" {
			spellings = append(spellings, "-"+flag.Shorthand)
		}
		for _, spelling := range spellings {
			for _, value := range []string{"-c", "-critical failure", "--config", "--config=x"} {
				args := []string{"--dry-run", spelling, value}
				t.Run(spelling+" "+value, func(t *testing.T) {
					result, dockerArgs, _ := invokeWrapper(t, args, nil, home, t.TempDir(), nil)
					if result.exitCode != 0 || len(result.stderr) != 0 {
						t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
					}
					if containsArg(dockerArgs, "--mount") {
						t.Fatalf("flag value was treated as a config path: %#v", dockerArgs)
					}
					assertForwardedArgs(t, dockerArgs, "ghcr.io/manprint/tgsend:latest", args)
				})
			}
		}
	}
}

func TestWrapperShortConfigWithEquals(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "equals.toml")
	writeWrapperConfig(t, configPath)
	result, dockerArgs, _ := invokeWrapper(t, []string{"-c=" + configPath, "--dry-run", "-m", "message"}, nil, home, t.TempDir(), nil)
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	assertConfigMount(t, dockerArgs, configPath)
}

func TestWrapperSymlinkedDefaultConfigMountedWhereBinaryLooks(t *testing.T) {
	home := t.TempDir()
	target := filepath.Join(t.TempDir(), "dotfiles", "tgsend.toml")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir dotfiles: %v", err)
	}
	writeWrapperConfig(t, target)
	link := filepath.Join(home, ".tgsend")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink config: %v", err)
	}
	result, dockerArgs, _ := invokeWrapper(t, []string{"-m", "message"}, nil, home, t.TempDir(), nil)
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	assertMount(t, dockerArgs, resolvedPath(t, target), link)
}

func TestWrapperRelativeConfigMountedUnderWorkdir(t *testing.T) {
	workdir := t.TempDir()
	if err := os.Mkdir(filepath.Join(workdir, "conf"), 0o755); err != nil {
		t.Fatalf("mkdir conf: %v", err)
	}
	configPath := filepath.Join(workdir, "conf", "relative.toml")
	writeWrapperConfig(t, configPath)
	result, dockerArgs, _ := invokeWrapper(t, []string{"-c", "conf/relative.toml", "-m", "message"}, nil, t.TempDir(), workdir, nil)
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	assertMount(t, dockerArgs, resolvedPath(t, configPath), workdir+"/conf/relative.toml")
}

func TestWrapperQuotesCommaInConfigMount(t *testing.T) {
	home := t.TempDir()
	configPath := filepath.Join(home, "a,b.toml")
	writeWrapperConfig(t, configPath)
	result, dockerArgs, _ := invokeWrapper(t, []string{"-c", configPath, "-m", "message"}, nil, home, t.TempDir(), nil)
	if result.exitCode != 0 {
		t.Fatalf("exit/stderr = %d/%q", result.exitCode, result.stderr)
	}
	mountIndex := indexOf(t, dockerArgs, "--mount")
	want := `type=bind,"src=` + resolvedPath(t, configPath) + `","dst=` + configPath + `",readonly`
	if string(dockerArgs[mountIndex+1]) != want {
		t.Fatalf("mount = %q, want %q", dockerArgs[mountIndex+1], want)
	}
}

func TestWrapperDoesNotAttachTerminalStdin(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("pseudo-terminal harness uses util-linux script")
	}
	scriptPath, err := exec.LookPath("script")
	if err != nil {
		t.Skip("script is not available")
	}
	env, argsPath := fakeDockerEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, scriptPath, "-qec", "'"+filepath.Join(projectRoot(t), "tgsend.sh")+"' --dry-run -m message", "/dev/null")
	command.Env = env
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("wrapper under a terminal failed: %v\n%s", err, output)
	}
	assertStdinNotAttached(t, readNULArgs(t, argsPath))
}

func TestWrapperDoesNotAttachDevNullStdin(t *testing.T) {
	env, argsPath := fakeDockerEnv(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// A nil Stdin connects the wrapper to the null device, as cron does.
	command := exec.CommandContext(ctx, filepath.Join(projectRoot(t), "tgsend.sh"), "--dry-run", "-m", "message")
	command.Env = env
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("wrapper with /dev/null stdin failed: %v\n%s", err, output)
	}
	assertStdinNotAttached(t, readNULArgs(t, argsPath))
}

func fakeDockerEnv(t *testing.T) ([]string, string) {
	t.Helper()
	temp := t.TempDir()
	fakeBin := filepath.Join(temp, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatalf("mkdir fake bin: %v", err)
	}
	fakeScript, err := os.ReadFile(filepath.Join(projectRoot(t), "test", "wrapper", "testdata", "fake-docker.sh"))
	if err != nil {
		t.Fatalf("read fake Docker: %v", err)
	}
	if err := os.WriteFile(filepath.Join(fakeBin, "docker"), fakeScript, 0o755); err != nil {
		t.Fatalf("write fake Docker: %v", err)
	}
	argsPath := filepath.Join(temp, "args.bin")
	return []string{
		"PATH=" + fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"HOME=" + t.TempDir(),
		"FAKE_DOCKER_ARGS=" + argsPath,
		"FAKE_DOCKER_STDIN=" + filepath.Join(temp, "stdin.bin"),
	}, argsPath
}

func assertStdinNotAttached(t *testing.T, dockerArgs [][]byte) {
	t.Helper()
	if len(dockerArgs) == 0 {
		t.Fatal("fake Docker was not invoked")
	}
	if containsArg(dockerArgs, "-i") {
		t.Fatalf("stdin was attached to the container: %#v", dockerArgs)
	}
	assertForwardedArgs(t, dockerArgs, "ghcr.io/manprint/tgsend:latest", []string{"--dry-run", "-m", "message"})
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
	absPath, err := filepath.Abs(configPath)
	if err != nil {
		t.Fatalf("make config path absolute: %v", err)
	}
	assertMount(t, args, resolvedPath(t, absPath), absPath)
}

// assertMount checks that the physical config file is mounted at the path the
// binary opens inside the container.
func assertMount(t *testing.T, args [][]byte, source, destination string) {
	t.Helper()
	mountIndex := indexOf(t, args, "--mount")
	want := "type=bind,src=" + source + ",dst=" + destination + ",readonly"
	if string(args[mountIndex+1]) != want {
		t.Fatalf("mount = %q, want %q; args=%#v", args[mountIndex+1], want, args)
	}
}

func resolvedPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("resolve %q: %v", path, err)
	}
	return resolved
}

func assertForwardedArgs(t *testing.T, args [][]byte, image string, want []string) {
	t.Helper()
	imageIndex := indexOf(t, args, image)
	got := make([]string, len(args[imageIndex+1:]))
	for index, arg := range args[imageIndex+1:] {
		got[index] = string(arg)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forwarded args = %#v, want %#v", got, want)
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
