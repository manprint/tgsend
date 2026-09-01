package installer_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

const fixtureVersion = "1.2.3"

type releaseFixture struct {
	server            *httptest.Server
	mu                sync.Mutex
	requests          []string
	assets            map[string][]byte
	checksumText      string
	delayFirstArchive bool
	started           chan struct{}
	release           chan struct{}
}

func newReleaseFixture(t *testing.T) *releaseFixture {
	t.Helper()
	f := &releaseFixture{assets: make(map[string][]byte), started: make(chan struct{}), release: make(chan struct{})}
	binary := []byte("#!/bin/sh\nprintf '%s\\n' '{\"ok\":true,\"schema_version\":1,\"result\":{\"version\":\"1.2.3\",\"commit\":\"fixture\",\"date\":\"fixture\"}}'\n")
	for _, suffix := range []string{"linux_amd64", "linux_arm64", "linux_armv7", "darwin_amd64", "darwin_arm64"} {
		name := "tgsend_" + suffix + ".tar.gz"
		f.assets[name] = tarGz(t, map[string][]byte{
			"tgsend":    binary,
			"README.md": []byte("fixture"),
			"LICENSE":   []byte("fixture"),
		})
	}
	f.checksumText = f.archiveChecksums()
	f.assets["tgsend.sh"] = mustRead(filepath.Join(repoRoot(t), "tgsend.sh"))
	f.assets["tgsend.sh.sha256"] = []byte(hashLine("tgsend.sh", f.assets["tgsend.sh"]))
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.requests = append(f.requests, r.URL.Path)
		f.mu.Unlock()
		name := filepath.Base(r.URL.Path)
		if name == "checksums.txt" {
			_, _ = io.WriteString(w, f.checksumText)
			return
		}
		asset, ok := f.assets[name]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if f.delayFirstArchive && strings.HasPrefix(name, "tgsend_") {
			select {
			case <-f.started:
			default:
				close(f.started)
			}
			<-f.release
		}
		_, _ = w.Write(asset)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *releaseFixture) archiveChecksums() string {
	names := make([]string, 0, 5)
	for name := range f.assets {
		if strings.HasSuffix(name, ".tar.gz") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(hashLine(name, f.assets[name]))
	}
	return b.String()
}

func (f *releaseFixture) setArchive(name string, content []byte) {
	f.assets[name] = content
}

func (f *releaseFixture) setChecksum(name string, content []byte) {
	lines := strings.Split(strings.TrimSuffix(f.checksumText, "\n"), "\n")
	for i, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == name {
			lines[i] = hashLine(name, content)
		}
	}
	var b strings.Builder
	for _, line := range lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	f.checksumText = b.String()
}

func (f *releaseFixture) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.requests...)
}

func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var raw bytes.Buffer
	zw := gzip.NewWriter(&raw)
	tw := tar.NewWriter(zw)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		data := files[name]
		mode := int64(0o644)
		if name == "tgsend" {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(data))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return raw.Bytes()
}

func hashLine(name string, content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../.."))
}

func scriptPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(repoRoot(t), "scripts", name)
}

func writeExecutable(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func cleanEnvironment(extra map[string]string) []string {
	remove := map[string]bool{
		"TGSEND_VERSION": true, "TGSEND_INSTALL_TEST": true, "TGSEND_INSTALL_BASE_URL": true,
		"TGSEND_INSTALL_DIR": true, "TGSEND_IMAGE": true, "TGSEND_TOKEN": true, "TGSEND_CHAT_ID": true,
		"FAKE_UNAME_S": true, "FAKE_UNAME_M": true, "FAKE_DOCKER_ARGS": true, "HOME": true, "TMPDIR": true,
		"PATH": true,
	}
	env := make([]string, 0, len(os.Environ())+len(extra))
	for _, item := range os.Environ() {
		key, _, ok := strings.Cut(item, "=")
		if !ok || !remove[key] {
			env = append(env, item)
		}
	}
	for key, value := range extra {
		env = append(env, key+"="+value)
	}
	return env
}

func runInstaller(t *testing.T, fixture *releaseFixture, script string, extra map[string]string) (string, error, string) {
	t.Helper()
	installDir := t.TempDir()
	fakeBin := t.TempDir()
	cmd := installerCommand(t, fixture, script, installDir, fakeBin, extra)
	out, err := cmd.CombinedOutput()
	return string(out), err, installDir
}

func installerCommand(t *testing.T, fixture *releaseFixture, script, installDir, fakeBin string, extra map[string]string) *exec.Cmd {
	t.Helper()
	writeExecutable(t, filepath.Join(fakeBin, "uname"), "#!/bin/sh\ncase \"$1\" in -s) printf '%s\\n' \"${FAKE_UNAME_S-Linux}\" ;; -m) printf '%s\\n' \"${FAKE_UNAME_M-x86_64}\" ;; esac\n")
	env := map[string]string{
		"PATH":                    fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TGSEND_INSTALL_TEST":     "1",
		"TGSEND_INSTALL_BASE_URL": fixture.server.URL + "/releases",
		"TGSEND_INSTALL_DIR":      installDir,
	}
	for key, value := range extra {
		env[key] = value
	}
	cmd := exec.Command("sh", scriptPath(t, script))
	cmd.Env = cleanEnvironment(env)
	return cmd
}

func assertPath(t *testing.T, paths []string, want string) {
	t.Helper()
	for _, path := range paths {
		if path == want {
			return
		}
	}
	t.Fatalf("request path %q not found in %v", want, paths)
}

func TestInstallerOSArchMap(t *testing.T) {
	cases := []struct {
		name, osName, machine, suffix string
	}{
		{"linux amd64", "Linux", "x86_64", "linux_amd64"},
		{"linux arm64", "Linux", "aarch64", "linux_arm64"},
		{"linux armv7", "Linux", "armv7l", "linux_armv7"},
		{"darwin amd64", "Darwin", "x86_64", "darwin_amd64"},
		{"darwin arm64", "Darwin", "arm64", "darwin_arm64"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newReleaseFixture(t)
			out, err, dir := runInstaller(t, f, "install.sh", map[string]string{
				"TGSEND_VERSION": "v" + fixtureVersion,
				"FAKE_UNAME_S":   tc.osName,
				"FAKE_UNAME_M":   tc.machine,
			})
			if err != nil {
				t.Fatalf("installer failed: %v\n%s", err, out)
			}
			assertPath(t, f.paths(), "/releases/v1.2.3/tgsend_"+tc.suffix+".tar.gz")
			if _, err := os.Stat(filepath.Join(dir, "tgsend")); err != nil {
				t.Fatalf("installed binary missing: %v", err)
			}
		})
	}
}

func TestVersionNormalizationAndRejection(t *testing.T) {
	for _, version := range []string{"1.2.3", "v1.2.3"} {
		t.Run("accept_"+version, func(t *testing.T) {
			f := newReleaseFixture(t)
			if out, err, _ := runInstaller(t, f, "install.sh", map[string]string{"TGSEND_VERSION": version}); err != nil {
				t.Fatalf("installer rejected %q: %v\n%s", version, err, out)
			}
			assertPath(t, f.paths(), "/releases/v1.2.3/tgsend_linux_amd64.tar.gz")
		})
	}
	for _, version := range []string{"", "1.2", "1.2.3.4", "1.02.3", "v1.2.3-rc1", "1..3"} {
		t.Run("reject_"+strings.ReplaceAll(version, ".", "_"), func(t *testing.T) {
			f := newReleaseFixture(t)
			out, err, dir := runInstaller(t, f, "install.sh", map[string]string{"TGSEND_VERSION": version})
			if err == nil {
				t.Fatalf("installer accepted invalid version %q\n%s", version, out)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "tgsend")); !os.IsNotExist(statErr) {
				t.Fatalf("invalid version left installation behind: %v", statErr)
			}
		})
	}
}

func TestLatestAndPinnedURLs(t *testing.T) {
	t.Run("latest", func(t *testing.T) {
		f := newReleaseFixture(t)
		if out, err, _ := runInstaller(t, f, "install.sh", nil); err != nil {
			t.Fatalf("latest install failed: %v\n%s", err, out)
		}
		paths := f.paths()
		assertPath(t, paths, "/releases/latest/download/tgsend_linux_amd64.tar.gz")
		assertPath(t, paths, "/releases/latest/download/checksums.txt")
	})
	t.Run("pinned", func(t *testing.T) {
		f := newReleaseFixture(t)
		if out, err, _ := runInstaller(t, f, "install.sh", map[string]string{"TGSEND_VERSION": "1.2.3"}); err != nil {
			t.Fatalf("pinned install failed: %v\n%s", err, out)
		}
		assertPath(t, f.paths(), "/releases/v1.2.3/tgsend_linux_amd64.tar.gz")
	})
}

func TestChecksumMismatchLeavesExistingBinary(t *testing.T) {
	f := newReleaseFixture(t)
	name := "tgsend_linux_amd64.tar.gz"
	f.setArchive(name, []byte("not the signed archive"))
	dir := t.TempDir()
	destination := filepath.Join(dir, "tgsend")
	old := []byte("old executable")
	if err := os.WriteFile(destination, old, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := installerCommand(t, f, "install.sh", dir, t.TempDir(), map[string]string{"TGSEND_VERSION": fixtureVersion})
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "checksum") {
		t.Fatalf("checksum mismatch was not rejected: %v\n%s", err, out)
	}
	got, readErr := os.ReadFile(destination)
	if readErr != nil || !bytes.Equal(got, old) {
		t.Fatalf("existing binary changed after checksum failure: err=%v content=%q", readErr, got)
	}
}

func TestTruncatedDownloadFails(t *testing.T) {
	f := newReleaseFixture(t)
	name := "tgsend_linux_amd64.tar.gz"
	truncated := f.assets[name][:16]
	f.setArchive(name, truncated)
	f.setChecksum(name, truncated)
	old := []byte("old executable")
	dir := t.TempDir()
	destination := filepath.Join(dir, "tgsend")
	if err := os.WriteFile(destination, old, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := installerCommand(t, f, "install.sh", dir, t.TempDir(), map[string]string{"TGSEND_VERSION": fixtureVersion})
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "release archive") {
		t.Fatalf("truncated archive was not rejected: %v\n%s", err, out)
	}
	got, readErr := os.ReadFile(destination)
	if readErr != nil || !bytes.Equal(got, old) {
		t.Fatalf("existing binary changed after truncated archive: err=%v content=%q", readErr, got)
	}
}

func TestMissingChecksumEntryFails(t *testing.T) {
	f := newReleaseFixture(t)
	f.checksumText = hashLine("unrelated.tar.gz", []byte("unrelated"))
	out, err, dir := runInstaller(t, f, "install.sh", map[string]string{"TGSEND_VERSION": fixtureVersion})
	if err == nil || !strings.Contains(string(out), "checksum entry") {
		t.Fatalf("missing checksum entry was not rejected: %v\n%s", err, out)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "tgsend")); !os.IsNotExist(statErr) {
		t.Fatalf("missing checksum entry left an installation: %v", statErr)
	}
}

func TestUnsupportedPlatformFails(t *testing.T) {
	f := newReleaseFixture(t)
	out, err, dir := runInstaller(t, f, "install.sh", map[string]string{"FAKE_UNAME_S": "FreeBSD", "FAKE_UNAME_M": "amd64"})
	if err == nil || !strings.Contains(string(out), "unsupported platform") {
		t.Fatalf("unsupported platform was not rejected: %v\n%s", err, out)
	}
	if len(f.paths()) != 0 {
		t.Fatalf("unsupported platform made network requests: %v", f.paths())
	}
	if _, statErr := os.Stat(filepath.Join(dir, "tgsend")); !os.IsNotExist(statErr) {
		t.Fatalf("unsupported platform left an installation: %v", statErr)
	}
}

func TestAtomicReplacement(t *testing.T) {
	f := newReleaseFixture(t)
	dir := t.TempDir()
	destination := filepath.Join(dir, "tgsend")
	if err := os.WriteFile(destination, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := installerCommand(t, f, "install.sh", dir, t.TempDir(), map[string]string{"TGSEND_VERSION": fixtureVersion})
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("replacement failed: %v\n%s", err, out)
	}
	mode := fileMode(t, destination)
	if mode != 0o755 {
		t.Fatalf("installed mode = %o, want 755", mode)
	}
	if bytes.Equal(mustRead(destination), []byte("old")) {
		t.Fatal("atomic replacement retained old binary")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tgsend.tmp.") {
			t.Fatalf("temporary install file remains: %s", entry.Name())
		}
	}
}

func TestCleanupOnSignalAndFailure(t *testing.T) {
	t.Run("failure", func(t *testing.T) {
		f := newReleaseFixture(t)
		f.checksumText = hashLine("other.tar.gz", []byte("other"))
		tmpRoot := t.TempDir()
		dir := t.TempDir()
		cmd := installerCommand(t, f, "install.sh", dir, t.TempDir(), map[string]string{
			"TGSEND_VERSION": fixtureVersion,
			"TMPDIR":         tmpRoot,
		})
		if out, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("failure fixture unexpectedly passed: %s", out)
		}
		assertDirectoryEmpty(t, tmpRoot)
	})
	t.Run("signal", func(t *testing.T) {
		f := newReleaseFixture(t)
		f.delayFirstArchive = true
		tmpRoot := t.TempDir()
		dir := t.TempDir()
		cmd := installerCommand(t, f, "install.sh", dir, t.TempDir(), map[string]string{
			"TGSEND_VERSION": fixtureVersion,
			"TMPDIR":         tmpRoot,
		})
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		select {
		case <-f.started:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatal("installer did not reach the delayed request")
		}
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		close(f.release)
		wait := make(chan error, 1)
		go func() { wait <- cmd.Wait() }()
		select {
		case <-wait:
		case <-time.After(5 * time.Second):
			_ = cmd.Process.Kill()
			t.Fatal("installer did not terminate after signal")
		}
		assertDirectoryEmpty(t, tmpRoot)
	})
}

func TestWrapperSyntaxFailureDoesNotInstall(t *testing.T) {
	f := newReleaseFixture(t)
	invalid := []byte("if then\n")
	f.assets["tgsend.sh"] = invalid
	f.assets["tgsend.sh.sha256"] = []byte(hashLine("tgsend.sh", invalid))
	dir := t.TempDir()
	destination := filepath.Join(dir, "tgsend")
	old := []byte("#!/bin/sh\nprintf '%s\\n' old\n")
	if err := os.WriteFile(destination, old, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := installerCommand(t, f, "install-wrapper.sh", dir, t.TempDir(), map[string]string{"TGSEND_VERSION": fixtureVersion})
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "syntax") {
		t.Fatalf("invalid wrapper was not rejected: %v\n%s", err, out)
	}
	if got := mustRead(destination); !bytes.Equal(got, old) {
		t.Fatalf("existing wrapper changed after syntax failure: %q", got)
	}
}

func TestNoCurlPipeInsideInstaller(t *testing.T) {
	for _, name := range []string{"install.sh", "install-wrapper.sh"} {
		data := string(mustRead(scriptPath(t, name)))
		for _, forbidden := range []string{"| sh", "| bash", "curl", "wget"} {
			if (forbidden == "curl" || forbidden == "wget") && !strings.Contains(data, forbidden) {
				t.Fatalf("%s does not document a downloader", name)
			}
		}
		if strings.Contains(data, "curl ") && strings.Contains(data, "curl ") && strings.Contains(data, "| sh") {
			t.Fatalf("%s contains a curl-to-shell pipeline", name)
		}
		if strings.Contains(data, "| bash") {
			t.Fatalf("%s contains a bash pipeline", name)
		}
	}
}

func TestInstallMode0755(t *testing.T) {
	f := newReleaseFixture(t)
	if out, err, dir := runInstaller(t, f, "install.sh", nil); err != nil {
		t.Fatalf("binary install failed: %v\n%s", err, out)
	} else if mode := fileMode(t, filepath.Join(dir, "tgsend")); mode != 0o755 {
		t.Fatalf("binary mode = %o, want 755", mode)
	}
	f = newReleaseFixture(t)
	if out, err, dir := runInstaller(t, f, "install-wrapper.sh", nil); err != nil {
		t.Fatalf("wrapper install failed: %v\n%s", err, out)
	} else if mode := fileMode(t, filepath.Join(dir, "tgsend")); mode != 0o755 {
		t.Fatalf("wrapper mode = %o, want 755", mode)
	}
}

func TestWrapperInstallsAndInvokesDocker(t *testing.T) {
	f := newReleaseFixture(t)
	dir := t.TempDir()
	fakeBin := t.TempDir()
	argsPath := filepath.Join(t.TempDir(), "docker-args")
	writeExecutable(t, filepath.Join(fakeBin, "docker"), "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$FAKE_DOCKER_ARGS\"\n")
	cmd := installerCommand(t, f, "install-wrapper.sh", dir, fakeBin, nil)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("wrapper install failed: %v\n%s", err, out)
	}
	env := cleanEnvironment(map[string]string{
		"PATH":             fakeBin + string(os.PathListSeparator) + os.Getenv("PATH"),
		"FAKE_DOCKER_ARGS": argsPath,
		"TGSEND_IMAGE":     "fixture/image:v1",
		"TGSEND_TOKEN":     "fixture-token",
		"TGSEND_CHAT_ID":   "42",
		"HOME":             t.TempDir(),
	})
	invoke := exec.Command(filepath.Join(dir, "tgsend"), "--dry-run", "message with spaces")
	invoke.Env = env
	if out, err := invoke.CombinedOutput(); err != nil {
		t.Fatalf("installed wrapper failed: %v\n%s", err, out)
	}
	args := strings.Fields(string(mustRead(argsPath)))
	assertContains(t, args, "fixture/image:v1")
	assertContains(t, args, "--dry-run")
	assertContains(t, args, "message")
	if !strings.Contains(string(mustRead(argsPath)), "message with spaces") {
		t.Fatalf("wrapper did not preserve argument with spaces: %q", mustRead(argsPath))
	}
}

func TestInstallerOutputDoesNotLeakFixtureToken(t *testing.T) {
	f := newReleaseFixture(t)
	f.checksumText = hashLine("other.tar.gz", []byte("other"))
	token := "fixture-secret-token"
	out, err, _ := runInstaller(t, f, "install.sh", map[string]string{
		"TGSEND_VERSION": fixtureVersion,
		"TGSEND_TOKEN":   token,
	})
	if err == nil {
		t.Fatal("invalid fixture unexpectedly passed")
	}
	if strings.Contains(out, token) || strings.Contains(out, "fixture") {
		t.Fatalf("installer output leaked fixture data: %q", out)
	}
}

func fileMode(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.Mode().Perm()
}

func assertDirectoryEmpty(t *testing.T, path string) {
	t.Helper()
	entries, err := os.ReadDir(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("temporary directory is not empty: %v", entries)
	}
}

func assertContains(t *testing.T, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%q not found in %v", want, values)
}
