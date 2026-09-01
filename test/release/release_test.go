package release_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

var archiveNames = []string{
	"tgsend_darwin_amd64.tar.gz",
	"tgsend_darwin_arm64.tar.gz",
	"tgsend_linux_amd64.tar.gz",
	"tgsend_linux_arm64.tar.gz",
	"tgsend_linux_armv7.tar.gz",
	"tgsend_windows_amd64.zip",
	"tgsend_windows_arm64.zip",
}

func TestReleaseCheckerPositiveAndNegative(t *testing.T) {
	cases := []struct {
		name string
		edit func(t *testing.T, dist string)
		want string
	}{
		{"duplicate checksum", func(t *testing.T, dist string) {
			path := filepath.Join(dist, "checksums.txt")
			data := mustRead(t, path)
			line := strings.SplitN(string(data), "\n", 2)[0] + "\n"
			write(t, path, append(data, []byte(line)...))
		}, "malformed or duplicate"},
		{"missing archive", func(t *testing.T, dist string) {
			if err := os.Remove(filepath.Join(dist, archiveNames[0])); err != nil {
				t.Fatal(err)
			}
		}, "archive set"},
		{"extra archive", func(t *testing.T, dist string) {
			write(t, filepath.Join(dist, "tgsend_extra.tar.gz"), []byte("extra"))
		}, "archive set"},
		{"malformed checksum", func(t *testing.T, dist string) {
			path := filepath.Join(dist, "checksums.txt")
			data := mustRead(t, path)
			data[0] = 'z'
			write(t, path, data)
		}, "malformed or duplicate"},
		{"path traversal member", func(t *testing.T, dist string) {
			name := "tgsend_linux_amd64.tar.gz"
			archive := tarGz(t, map[string][]byte{
				"LICENSE":   []byte("license"),
				"README.md": []byte("readme"),
				"tgsend":    []byte("#!/bin/sh\nprintf '%s\\n' '{\"ok\":true}'\n"),
				"../escape": []byte("bad"),
			})
			write(t, filepath.Join(dist, name), archive)
			rewriteChecksums(t, dist)
		}, "unsafe or duplicate members"},
		{"missing SBOM", func(t *testing.T, dist string) {
			if err := os.Remove(filepath.Join(dist, archiveNames[0]+".sbom.json")); err != nil {
				t.Fatal(err)
			}
		}, "missing archive SBOM"},
	}

	t.Run("positive", func(t *testing.T) {
		dist, project := makeReleaseFixture(t)
		out, err := runChecker(t, dist, project)
		if err != nil {
			t.Fatalf("release checker rejected valid fixture: %v\n%s", err, out)
		}
	})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dist, project := makeReleaseFixture(t)
			tc.edit(t, dist)
			out, err := runChecker(t, dist, project)
			if err == nil || !strings.Contains(out, tc.want) {
				t.Fatalf("checker failure did not identify %s: %v\n%s", tc.name, err, out)
			}
		})
	}
}

func TestImageCheckerRejectsExtraPlatform(t *testing.T) {
	if _, err := exec.LookPath("jq"); err != nil {
		t.Skip("jq is required by the image checker")
	}
	valid := imageIndex(t, false)
	if out, err := runImageChecker(t, valid); err != nil {
		t.Fatalf("image checker rejected valid index: %v\n%s", err, out)
	}
	extra := imageIndex(t, true)
	if out, err := runImageChecker(t, extra); err == nil || !strings.Contains(out, "missing the exact three platforms") {
		t.Fatalf("image checker accepted extra platform: %v\n%s", err, out)
	}
}

func makeReleaseFixture(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dist := filepath.Join(root, "dist")
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(filepath.Join(project, "scripts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dist, 0o755); err != nil {
		t.Fatal(err)
	}
	repo := repoRoot(t)
	write(t, filepath.Join(project, "scripts", "install.sh"), mustRead(t, filepath.Join(repo, "scripts", "install.sh")))
	wrapper := mustRead(t, filepath.Join(repo, "tgsend.sh"))
	write(t, filepath.Join(project, "tgsend.sh"), wrapper)
	write(t, filepath.Join(project, "tgsend.sh.sha256"), []byte(hashLine("tgsend.sh", wrapper)))

	native := []byte("#!/bin/sh\nprintf '%s\\n' '{\"ok\":true,\"schema_version\":1,\"result\":{\"version\":\"1.2.3\"}}'\n")
	for _, name := range archiveNames {
		member := "tgsend"
		if strings.HasSuffix(name, ".zip") {
			member = "tgsend.exe"
		}
		archive := map[string][]byte{"LICENSE": []byte("license"), "README.md": []byte("readme"), member: native}
		if strings.HasSuffix(name, ".tar.gz") {
			write(t, filepath.Join(dist, name), tarGz(t, archive))
		} else {
			write(t, filepath.Join(dist, name), zipArchive(t, archive))
		}
		write(t, filepath.Join(dist, name+".sbom.json"), []byte(`{"spdxVersion":"SPDX-2.3"}`))
	}
	for _, suffix := range []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64", "linux_arm", "windows_amd64", "windows_arm64"} {
		write(t, filepath.Join(dist, "tgsend_1.2.3_"+suffix+".sbom.json"), []byte(`{"spdxVersion":"SPDX-2.3"}`))
	}
	rewriteChecksums(t, dist)
	return dist, project
}

func rewriteChecksums(t *testing.T, dist string) {
	t.Helper()
	entries := make([]string, 0)
	items, err := os.ReadDir(dist)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range items {
		if item.Name() != "checksums.txt" && item.Type().IsRegular() {
			entries = append(entries, item.Name())
		}
	}
	sort.Strings(entries)
	var b strings.Builder
	for _, name := range entries {
		b.WriteString(hashLine(name, mustRead(t, filepath.Join(dist, name))))
	}
	write(t, filepath.Join(dist, "checksums.txt"), []byte(b.String()))
}

func tarGz(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := gzip.NewWriter(&out)
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
	return out.Bytes()
}

func zipArchive(t *testing.T, files map[string][]byte) []byte {
	t.Helper()
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(files[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

func imageIndex(t *testing.T, extra bool) []byte {
	t.Helper()
	manifests := []map[string]any{
		{"platform": map[string]any{"os": "linux", "architecture": "amd64"}},
		{"platform": map[string]any{"os": "linux", "architecture": "arm64"}},
		{"platform": map[string]any{"os": "linux", "architecture": "arm", "variant": "v7"}},
	}
	if extra {
		manifests = append(manifests, map[string]any{"platform": map[string]any{"os": "linux", "architecture": "386"}})
	}
	data := map[string]any{
		"annotations": map[string]string{
			"org.opencontainers.image.licenses": "MIT",
			"org.opencontainers.image.source":   "https://github.com/manprint/tgsend",
			"org.opencontainers.image.version":  "1.2.3",
			"org.opencontainers.image.revision": "fixture",
		},
		"manifests": manifests,
	}
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return encoded
}

func runChecker(t *testing.T, dist, project string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", filepath.Join(repoRoot(t), "scripts", "check-release.sh"), dist, project)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func runImageChecker(t *testing.T, index []byte) (string, error) {
	t.Helper()
	tmp := t.TempDir()
	jsonPath := filepath.Join(tmp, "index.json")
	write(t, jsonPath, index)
	fakeBin := filepath.Join(tmp, "bin")
	if err := os.Mkdir(fakeBin, 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(fakeBin, "docker"), []byte("#!/bin/sh\ncat \"$FAKE_IMAGE_JSON\"\n"))
	if err := os.Chmod(filepath.Join(fakeBin, "docker"), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", filepath.Join(repoRoot(t), "scripts", "check-image.sh"), "ghcr.io/manprint/tgsend:v1.2.3")
	cmd.Env = append(os.Environ(), "PATH="+fakeBin+string(os.PathListSeparator)+os.Getenv("PATH"), "FAKE_IMAGE_JSON="+jsonPath)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func hashLine(name string, content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:]) + "  " + name + "\n"
}

func write(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
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
