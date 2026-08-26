package repo

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type PackageEntry struct {
	Control   *Control
	PoolPath  string
	Filename  string
	Size      int64
	SHA256    string
	Distro    string
	Component string
}

var packagesControlFieldOrder = []string{
	"Package", "Source", "Version", "Architecture", "Maintainer",
	"Installed-Size", "Depends", "Pre-Depends", "Recommends", "Suggests",
	"Conflicts", "Breaks", "Replaces", "Provides", "Enhances",
	"Section", "Priority", "Homepage", "Description",
}

var packagesScannerFields = map[string]bool{
	"Filename": true,
	"Size":     true,
	"MD5sum":   true,
	"SHA1":     true,
	"SHA256":   true,
	"SHA512":   true,
}

func (e *PackageEntry) PackagesStanza() string {
	var b strings.Builder
	body := ""
	if e.Control != nil {
		body = strings.TrimRight(e.Control.Raw, "\n")
		if body == "" {
			body = strings.TrimRight(e.controlFieldsStanza(), "\n")
		}
	}
	if body != "" {
		b.WriteString(body)
		b.WriteByte('\n')
	}
	writeDeb822Field(&b, "Filename", filepath.ToSlash(e.PoolPath))
	writeDeb822Field(&b, "Size", fmt.Sprintf("%d", e.Size))
	writeDeb822Field(&b, "SHA256", e.SHA256)
	b.WriteByte('\n')
	return b.String()
}

func (e *PackageEntry) controlFieldsStanza() string {
	if e.Control == nil {
		return ""
	}
	values := map[string]string{}
	if e.Control.Fields != nil {
		for k, v := range e.Control.Fields {
			if packagesScannerFields[k] || v == "" {
				continue
			}
			values[k] = v
		}
	}
	setIfMissing := func(key, val string) {
		if val != "" && values[key] == "" {
			values[key] = val
		}
	}
	setIfMissing("Package", e.Control.Package)
	setIfMissing("Version", e.Control.Version)
	setIfMissing("Architecture", e.Control.Architecture)
	setIfMissing("Maintainer", e.Control.Maintainer)
	setIfMissing("Installed-Size", e.Control.InstalledSize)
	setIfMissing("Depends", e.Control.Depends)
	setIfMissing("Description", e.Control.Description)

	var b strings.Builder
	seen := map[string]bool{}
	for _, key := range packagesControlFieldOrder {
		v := values[key]
		if v == "" {
			continue
		}
		writeDeb822Field(&b, key, v)
		seen[key] = true
	}
	var extra []string
	for key := range values {
		if !seen[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		writeDeb822Field(&b, key, values[key])
	}
	return b.String()
}

// writeDeb822Field writes a field using Debian control folding: continuation
// lines start with a space, and a blank line in the value is " .".
func writeDeb822Field(b *strings.Builder, key, value string) {
	if value == "" {
		return
	}
	b.WriteString(key)
	b.WriteString(": ")
	lines := strings.Split(value, "\n")
	b.WriteString(lines[0])
	b.WriteByte('\n')
	for _, line := range lines[1:] {
		if line == "" {
			b.WriteString(" .\n")
			continue
		}
		b.WriteString(" ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

func WritePackagesIndex(repoDir, distro, component, arch string, entries []PackageEntry) error {
	dir := filepath.Join(repoDir, "dists", distro, component, "binary-"+arch)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var content strings.Builder
	for _, e := range entries {
		content.WriteString(e.PackagesStanza())
	}
	packagesPath := filepath.Join(dir, "Packages")
	if err := atomicWrite(packagesPath, []byte(content.String())); err != nil {
		return err
	}
	gzPath := filepath.Join(dir, "Packages.gz")
	var gzBuf bytes.Buffer
	gw := gzip.NewWriter(&gzBuf)
	if _, err := gw.Write([]byte(content.String())); err != nil {
		return err
	}
	if err := gw.Close(); err != nil {
		return err
	}
	return atomicWrite(gzPath, gzBuf.Bytes())
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
