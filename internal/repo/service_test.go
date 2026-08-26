package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jdbnet/aptuary/internal/store"
)

func TestUploadKeepsNewerSemanticVersion(t *testing.T) {
	svc := testService(t)

	deb9 := buildTestDeb(t, "myapp", "1.9.0", "amd64", []byte("binary v9"))
	deb10 := buildTestDeb(t, "myapp", "1.10.0", "amd64", []byte("binary v10"))
	if _, err := svc.Upload("stable", "main", deb9); err != nil {
		t.Fatalf("upload 1.9.0: %v", err)
	}
	if _, err := svc.Upload("stable", "main", deb10); err != nil {
		t.Fatalf("upload 1.10.0: %v", err)
	}

	packagesPath := filepath.Join(svc.RepoDir(), "dists", "stable", "main", "binary-amd64", "Packages")
	content, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("read Packages: %v", err)
	}
	text := string(content)
	if !strings.Contains(text, "Version: 1.10.0") {
		t.Fatalf("expected 1.10.0 in index, got:\n%s", text)
	}
	if strings.Contains(text, "Version: 1.9.0") {
		t.Fatalf("old version should be pruned from index:\n%s", text)
	}
}

func TestUploadNfpmDeb(t *testing.T) {
	data, err := os.ReadFile("/tmp/testapp.deb")
	if err != nil {
		t.Skip("nfpm test deb not found; run nfpm package to create /tmp/testapp.deb")
	}
	svc := testService(t)
	pkg, err := svc.Upload("stable", "main", data)
	if err != nil {
		t.Fatalf("upload nfpm deb: %v", err)
	}
	if pkg.Name != "testapp" || pkg.Version != "1.0.0" {
		t.Fatalf("unexpected package: %+v", pkg)
	}
}

func TestUploadReplacesSameVersionDifferentContent(t *testing.T) {
	svc := testService(t)

	deb1 := buildTestDeb(t, "testpkg", "1.0.0", "amd64", []byte("binary v1"))
	if _, err := svc.Upload("stable", "main", deb1); err != nil {
		t.Fatalf("first upload: %v", err)
	}

	deb2 := buildTestDeb(t, "testpkg", "1.0.0", "amd64", []byte("binary v2"))
	pkg, err := svc.Upload("stable", "main", deb2)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if pkg.SHA256 != SHA256Hex(deb2) {
		t.Fatalf("expected package to be replaced with new content")
	}
}

func TestUploadIdempotentSameVersionSameContent(t *testing.T) {
	svc := testService(t)

	deb := buildTestDeb(t, "testpkg", "1.0.0", "amd64", []byte("binary v1"))
	first, err := svc.Upload("stable", "main", deb)
	if err != nil {
		t.Fatalf("first upload: %v", err)
	}

	packagesPath := filepath.Join(svc.RepoDir(), "dists", "stable", "main", "binary-amd64", "Packages")
	before, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("read Packages: %v", err)
	}

	second, err := svc.Upload("stable", "main", deb)
	if err != nil {
		t.Fatalf("second upload: %v", err)
	}
	if second.ID != first.ID {
		t.Fatalf("expected same package id, got %d then %d", first.ID, second.ID)
	}

	after, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("read Packages after re-upload: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("idempotent upload should not republish Packages index")
	}
}

func TestPublishIfStaleSkipsUnchangedRepository(t *testing.T) {
	svc := testService(t)

	deb := buildTestDeb(t, "testpkg", "1.0.0", "amd64", []byte("binary v1"))
	if _, err := svc.Upload("stable", "main", deb); err != nil {
		t.Fatalf("upload: %v", err)
	}

	inRelease := filepath.Join(svc.RepoDir(), "dists", "stable", "InRelease")
	before, err := os.ReadFile(inRelease)
	if err != nil {
		t.Fatalf("read InRelease: %v", err)
	}

	if err := svc.PublishIfStale(); err != nil {
		t.Fatalf("publish if stale: %v", err)
	}

	after, err := os.ReadFile(inRelease)
	if err != nil {
		t.Fatalf("read InRelease after stale check: %v", err)
	}
	if string(after) != string(before) {
		t.Fatal("expected PublishIfStale to skip republish when repository is unchanged")
	}
}

func TestPublishAllIsStableForSingleUpload(t *testing.T) {
	svc := testService(t)

	deb := buildTestDeb(t, "testpkg", "1.0.0", "amd64", []byte("binary v1"))
	if _, err := svc.Upload("stable", "main", deb); err != nil {
		t.Fatalf("upload: %v", err)
	}

	packagesPath := filepath.Join(svc.RepoDir(), "dists", "stable", "main", "binary-amd64", "Packages")
	before, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("read Packages: %v", err)
	}
	if err := assertPackagesMatchesPool(t, svc.RepoDir(), before); err != nil {
		t.Fatalf("after upload: %v", err)
	}

	if err := svc.PublishAll(); err != nil {
		t.Fatalf("republish: %v", err)
	}

	after, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("read Packages after republish: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("startup republish changed Packages index:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if err := assertPackagesMatchesPool(t, svc.RepoDir(), after); err != nil {
		t.Fatalf("after republish: %v", err)
	}
}

func TestUploadMultilineDescriptionKeepsDownloadFields(t *testing.T) {
	svc := testService(t)

	control := "Package: icetray\nVersion: 1.3.8\nArchitecture: amd64\nMaintainer: Test <test@test>\nHomepage: https://example.com/icetray\nDescription: IceTray is a lightweight system tray.\n Longer explanation of the application.\n .\n Second paragraph after a blank line.\n"
	deb := buildTestDebWithControl(t, control, []byte("binary v138"))
	if _, err := svc.Upload("stable", "main", deb); err != nil {
		t.Fatalf("upload: %v", err)
	}

	packagesPath := filepath.Join(svc.RepoDir(), "dists", "stable", "main", "binary-amd64", "Packages")
	content, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("read Packages: %v", err)
	}
	if err := assertPackagesMatchesPool(t, svc.RepoDir(), content); err != nil {
		t.Fatalf("packages vs pool: %v", err)
	}

	text := string(content)
	stanzas := strings.Split(strings.TrimSpace(text), "\n\n")
	if len(stanzas) != 1 {
		t.Fatalf("expected one Packages stanza, got %d:\n%s", len(stanzas), text)
	}
	stanza := stanzas[0] + "\n"
	if !regexp.MustCompile(`(?m)^Package: icetray$`).MatchString(stanza) {
		t.Fatalf("missing Package field:\n%s", text)
	}
	if !regexp.MustCompile(`(?m)^Filename: pool/main/i/icetray/icetray_1.3.8_amd64.deb$`).MatchString(stanza) {
		t.Fatalf("Filename must stay in the package stanza:\n%s", text)
	}
	if !regexp.MustCompile(`(?m)^SHA256: [0-9a-f]+$`).MatchString(stanza) {
		t.Fatalf("SHA256 must stay in the package stanza:\n%s", text)
	}
	if !regexp.MustCompile(`(?m)^Homepage: https://example.com/icetray$`).MatchString(stanza) {
		t.Fatalf("expected extra control fields from the .deb:\n%s", text)
	}
}

func TestAptUpgradeFindsSourceForInstalledPackage(t *testing.T) {
	if _, err := exec.LookPath("apt-get"); err != nil {
		t.Skip("apt-get not available")
	}

	svc := testService(t)
	control := "Package: icetray\nVersion: 1.3.8\nArchitecture: amd64\nMaintainer: Test <test@test>\nDescription: IceTray is a lightweight system tray.\n Longer explanation of the application.\n .\n Second paragraph after a blank line.\n"
	deb := buildTestDebWithControl(t, control, []byte("binary v138"))
	if _, err := svc.Upload("stable", "main", deb); err != nil {
		t.Fatalf("upload: %v", err)
	}

	root := t.TempDir()
	etc := filepath.Join(root, "etc", "apt")
	state := filepath.Join(root, "var", "lib", "apt")
	cache := filepath.Join(root, "var", "cache", "apt")
	dpkg := filepath.Join(root, "var", "lib", "dpkg")
	for _, dir := range []string{
		filepath.Join(etc, "apt.conf.d"),
		filepath.Join(etc, "preferences.d"),
		filepath.Join(state, "lists", "partial"),
		filepath.Join(cache, "archives", "partial"),
		dpkg,
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	repoDir := svc.RepoDir()
	if !strings.HasSuffix(repoDir, string(filepath.Separator)) {
		repoDir += string(filepath.Separator)
	}
	sources := fmt.Sprintf("deb [trusted=yes arch=amd64] file:%s stable main\n", repoDir)
	if err := os.WriteFile(filepath.Join(etc, "sources.list"), []byte(sources), 0o644); err != nil {
		t.Fatal(err)
	}
	status := "Package: icetray\nStatus: install ok installed\nArchitecture: amd64\nVersion: 1.3.7\nMaintainer: Test <test@test>\nDescription: old version\n\n"
	if err := os.WriteFile(filepath.Join(dpkg, "status"), []byte(status), 0o644); err != nil {
		t.Fatal(err)
	}

	aptOpts := []string{
		"-o", "Debug::NoLocking=1",
		"-o", "APT::Get::List-Cleanup=0",
		"-o", "APT::Architectures=amd64",
		"-o", "Dir=" + root,
		"-o", "Dir::State=" + state,
		"-o", "Dir::Cache=" + cache,
		"-o", "Dir::Etc=" + etc,
		"-o", "Dir::State::status=" + filepath.Join(dpkg, "status"),
		"-o", "Dir::Etc::sourcelist=" + filepath.Join(etc, "sources.list"),
		"-o", "Dir::Etc::sourceparts=/dev/null",
	}

	update := exec.Command("apt-get", append(aptOpts, "update")...)
	if out, err := update.CombinedOutput(); err != nil {
		t.Fatalf("apt-get update: %v\n%s", err, out)
	}

	upgrade := exec.Command("apt-get", append(aptOpts, "-y", "--print-uris", "upgrade")...)
	out, err := upgrade.CombinedOutput()
	if err != nil {
		t.Fatalf("apt-get upgrade: %v\n%s", err, out)
	}
	text := string(out)
	if strings.Contains(text, "Can't find a source to download") {
		t.Fatalf("apt could not download the published version:\n%s", text)
	}
	if !strings.Contains(text, "icetray_1.3.8_amd64.deb") {
		t.Fatalf("expected apt to select icetray 1.3.8, got:\n%s", text)
	}
}

func TestPackagesIndexHasSingleStanzaPerPackage(t *testing.T) {
	svc := testService(t)

	deb := buildTestDeb(t, "testpkg", "1.0.0", "amd64", []byte("binary v1"))
	if _, err := svc.Upload("stable", "main", deb); err != nil {
		t.Fatalf("upload: %v", err)
	}

	packagesPath := filepath.Join(svc.RepoDir(), "dists", "stable", "main", "binary-amd64", "Packages")
	content, err := os.ReadFile(packagesPath)
	if err != nil {
		t.Fatalf("read Packages: %v", err)
	}
	if strings.Count(string(content), "Package: testpkg\n") != 1 {
		t.Fatalf("expected one package stanza, got:\n%s", content)
	}
}

func assertPackagesMatchesPool(t *testing.T, repoDir string, packages []byte) error {
	t.Helper()
	shaRe := regexp.MustCompile(`(?m)^SHA256: ([0-9a-f]+)$`)
	filenameRe := regexp.MustCompile(`(?m)^Filename: (.+)$`)
	versionRe := regexp.MustCompile(`(?m)^Version: (.+)$`)

	sha := shaRe.FindStringSubmatch(string(packages))
	filename := filenameRe.FindStringSubmatch(string(packages))
	version := versionRe.FindStringSubmatch(string(packages))
	if len(sha) != 2 || len(filename) != 2 || len(version) != 2 {
		return fmt.Errorf("could not parse Packages stanza:\n%s", packages)
	}

	poolPath := filepath.Join(repoDir, filepath.FromSlash(filename[1]))
	data, err := os.ReadFile(poolPath)
	if err != nil {
		return fmt.Errorf("read pool file %s: %w", poolPath, err)
	}
	if got := SHA256Hex(data); got != sha[1] {
		return fmt.Errorf("Packages SHA256 %s does not match pool file %s", sha[1], got)
	}

	ctrl, err := ParseDeb(data)
	if err != nil {
		return fmt.Errorf("parse pool deb: %w", err)
	}
	if ctrl.Version != version[1] {
		return fmt.Errorf("Packages version %q does not match pool control version %q", version[1], ctrl.Version)
	}
	return nil
}

func testService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	db, err := store.Open(dir)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	repoDir := filepath.Join(dir, "repo")
	signer := &Signer{GPGHome: filepath.Join(dir, "gpg")}
	if _, err := signer.EnsureKey(); err != nil {
		t.Fatalf("ensure gpg key: %v", err)
	}

	repos := func() []RepoConfig {
		return []RepoConfig{
			{Name: "stable", Components: []string{"main"}, Architectures: []string{"amd64"}},
		}
	}
	return NewService(db.SQL, repoDir, signer, repos)
}

func buildTestDeb(t *testing.T, pkg, version, arch string, binary []byte) []byte {
	t.Helper()
	control := fmt.Sprintf(
		"Package: %s\nVersion: %s\nArchitecture: %s\nMaintainer: Test <test@test>\nDescription: test package\n",
		pkg, version, arch,
	)
	return buildTestDebWithControl(t, control, binary)
}

func buildTestDebWithControl(t *testing.T, control string, binary []byte) []byte {
	t.Helper()
	pkgRe := regexp.MustCompile(`(?m)^Package: (.+)$`)
	m := pkgRe.FindStringSubmatch(control)
	if len(m) != 2 {
		t.Fatalf("control is missing Package: %s", control)
	}
	pkg := m[1]

	var ctar bytes.Buffer
	gw := gzip.NewWriter(&ctar)
	tw := tar.NewWriter(gw)
	hdr := &tar.Header{Name: "control", Mode: 0o644, Size: int64(len(control))}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(control)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}

	var dtar bytes.Buffer
	gw2 := gzip.NewWriter(&dtar)
	tw2 := tar.NewWriter(gw2)
	hdr2 := &tar.Header{Name: "./usr/bin/" + pkg, Mode: 0o755, Size: int64(len(binary))}
	if err := tw2.WriteHeader(hdr2); err != nil {
		t.Fatal(err)
	}
	if _, err := tw2.Write(binary); err != nil {
		t.Fatal(err)
	}
	if err := tw2.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw2.Close(); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	out.WriteString("!<arch>\n")
	writeArMember(&out, "debian-binary", []byte("2.0\n"))
	writeArMember(&out, "control.tar.gz", ctar.Bytes())
	writeArMember(&out, "data.tar.gz", dtar.Bytes())
	return out.Bytes()
}

func writeArMember(w *bytes.Buffer, name string, data []byte) {
	fmt.Fprintf(w, "%-16s", name)
	fmt.Fprintf(w, "%-12d", 0)
	fmt.Fprintf(w, "%-6d", 0)
	fmt.Fprintf(w, "%-6d", 0)
	fmt.Fprintf(w, "%-8o", 0o644)
	fmt.Fprintf(w, "%-10d", len(data))
	w.WriteString("`\n")
	w.Write(data)
	if len(data)%2 == 1 {
		w.WriteByte('\n')
	}
}
