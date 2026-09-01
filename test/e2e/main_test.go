//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

var (
	repoRoot   string
	binaryPath string
)

func TestMain(m *testing.M) {
	_, sourceFile, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "cannot locate e2e test source")
		os.Exit(1)
	}
	repoRoot = filepath.Clean(filepath.Join(filepath.Dir(sourceFile), "..", ".."))

	buildDir, err := os.MkdirTemp("", "tgsend-e2e-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create e2e temp directory: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(buildDir)

	executableName := "tgsend"
	if runtime.GOOS == "windows" {
		executableName += ".exe"
	}
	binaryPath = filepath.Join(buildDir, executableName)
	build := exec.Command("go", "build", "-ldflags", "-X github.com/manprint/tgsend/internal/buildinfo.TestEndpointEnabled=true", "-o", binaryPath, "./cmd/tgsend")
	build.Dir = repoRoot
	build.Env = buildEnvironment(filepath.Join(buildDir, "home"))
	var output bytes.Buffer
	build.Stdout = &output
	build.Stderr = &output
	if err := build.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "build e2e binary: %v\n%s", err, output.String())
		os.Exit(1)
	}

	os.Exit(m.Run())
}

func buildEnvironment(home string) []string {
	env := []string{"HOME=" + home, "GO111MODULE=on"}
	if runtime.GOOS == "windows" {
		env = append(env, "USERPROFILE="+home)
	}
	for _, key := range []string{"PATH", "SYSTEMROOT", "WINDIR", "PATHEXT", "GOPATH", "GOMODCACHE", "GOCACHE", "GOTOOLCHAIN", "GOENV", "TMPDIR"} {
		if value, ok := os.LookupEnv(key); ok && value != "" {
			env = append(env, key+"="+value)
		}
	}
	return env
}
