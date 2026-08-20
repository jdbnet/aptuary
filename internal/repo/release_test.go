package repo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteReleasePaths(t *testing.T) {
	dir := t.TempDir()
	distro := "stable"
	distsDir := filepath.Join(dir, "dists", distro)
	pkgDir := filepath.Join(distsDir, "main", "binary-amd64")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := []byte("Package: test\n\n")
	if err := os.WriteFile(filepath.Join(pkgDir, "Packages"), content, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pkgDir, "Packages.gz"), content, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := WriteRelease(dir, distro, []string{"main"}, []string{"amd64"}); err != nil {
		t.Fatal(err)
	}

	release, err := os.ReadFile(filepath.Join(distsDir, "Release"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(release)
	if strings.Contains(text, "dists/stable/") {
		t.Fatalf("Release should use distro-relative paths, got:\n%s", text)
	}
	if !strings.Contains(text, "main/binary-amd64/Packages") {
		t.Fatalf("expected main/binary-amd64/Packages in Release, got:\n%s", text)
	}
	if strings.Count(text, "SHA256:") != 1 {
		t.Fatalf("Release should have one SHA256 section header, got:\n%s", text)
	}
	if !strings.Contains(text, "SHA256:\n ") {
		t.Fatalf("Release checksums should be listed under a single SHA256 section, got:\n%s", text)
	}
}
