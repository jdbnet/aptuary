package repo

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

// Control holds parsed Debian control file fields.
type Control struct {
	Package      string
	Version      string
	Architecture string
	Maintainer   string
	Description  string
	Depends      string
	InstalledSize string
	Fields       map[string]string
}

func ParseDeb(data []byte) (*Control, error) {
	controlData, err := extractControl(data)
	if err != nil {
		return nil, err
	}
	return parseControl(controlData)
}

func ParseDebReader(r io.Reader) (*Control, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return ParseDeb(data)
}

func extractControl(deb []byte) ([]byte, error) {
	if len(deb) < 8 || string(deb[:8]) != "!<arch>\n" {
		return nil, fmt.Errorf("invalid deb archive header")
	}
	offset := 8
	for offset+60 <= len(deb) {
		name := strings.TrimSpace(string(deb[offset : offset+16]))
		sizeStr := strings.TrimSpace(string(deb[offset+48 : offset+58]))
		var size int
		fmt.Sscanf(sizeStr, "%d", &size)
		offset += 60
		if offset+size > len(deb) {
			break
		}
		content := deb[offset : offset+size]
		offset += size
		if offset%2 == 1 {
			offset++
		}
		base := name
		if idx := strings.Index(base, "/"); idx >= 0 {
			base = base[:idx]
		}
		if strings.HasPrefix(base, "control.tar") {
			return extractControlFromTar(content, base)
		}
	}
	return nil, fmt.Errorf("control.tar not found in deb")
}

func extractControlFromTar(data []byte, name string) ([]byte, error) {
	var tr *tar.Reader
	if strings.HasSuffix(name, ".gz") {
		gr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gr.Close()
		tr = tar.NewReader(gr)
	} else {
		tr = tar.NewReader(bytes.NewReader(data))
	}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == "control" || hdr.Name == "./control" {
			return io.ReadAll(tr)
		}
	}
	return nil, fmt.Errorf("control file not found in control.tar")
}

func parseControl(data []byte) (*Control, error) {
	fields := parseDeb822(string(data))
	c := &Control{Fields: fields}
	c.Package = fields["Package"]
	c.Version = fields["Version"]
	c.Architecture = fields["Architecture"]
	c.Maintainer = fields["Maintainer"]
	c.Description = fields["Description"]
	c.Depends = fields["Depends"]
	c.InstalledSize = fields["Installed-Size"]
	if c.Package == "" || c.Version == "" || c.Architecture == "" {
		return nil, fmt.Errorf("missing required control fields (Package, Version, Architecture)")
	}
	return c, nil
}

func parseDeb822(s string) map[string]string {
	out := map[string]string{}
	var key string
	var val []string
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if line == "" {
			if key != "" {
				out[key] = strings.TrimSpace(strings.Join(val, "\n"))
				key = ""
				val = nil
			}
			continue
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if key != "" {
				val = append(val, strings.TrimSpace(line))
			}
			continue
		}
		if key != "" {
			out[key] = strings.TrimSpace(strings.Join(val, "\n"))
		}
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key = strings.TrimSpace(parts[0])
		val = []string{strings.TrimSpace(parts[1])}
	}
	if key != "" {
		out[key] = strings.TrimSpace(strings.Join(val, "\n"))
	}
	return out
}

func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
