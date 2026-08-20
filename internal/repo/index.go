package repo

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
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

func (e *PackageEntry) PackagesStanza() string {
	var b strings.Builder
	fields := map[string]string{
		"Package":       e.Control.Package,
		"Version":       e.Control.Version,
		"Architecture":  e.Control.Architecture,
		"Maintainer":    e.Control.Maintainer,
		"Description":   e.Control.Description,
		"Depends":       e.Control.Depends,
		"Installed-Size": e.Control.InstalledSize,
		"Filename":      e.PoolPath,
		"Size":          fmt.Sprintf("%d", e.Size),
		"SHA256":        e.SHA256,
	}
	order := []string{"Package", "Version", "Architecture", "Maintainer", "Installed-Size", "Depends", "Description", "Filename", "Size", "SHA256"}
	for _, k := range order {
		v := fields[k]
		if v == "" {
			continue
		}
		b.WriteString(k)
		b.WriteString(": ")
		b.WriteString(v)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	return b.String()
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
