package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Bump when repository metadata format changes and existing installs need a republish.
const publishFormatVersion = 4

func (s *Service) PublishIfStale() error {
	fingerprint, err := s.packagesFingerprint()
	if err != nil {
		return err
	}
	stored, err := s.readPublishFingerprint()
	if err != nil {
		return err
	}
	if stored == fingerprint {
		return nil
	}
	if err := s.PublishAll(); err != nil {
		return err
	}
	return s.writePublishFingerprint(fingerprint)
}

func (s *Service) packagesFingerprint() (string, error) {
	rows, err := s.db.Query(
		`SELECT name, version, architecture, distribution, component, sha256
		 FROM packages ORDER BY distribution, component, architecture, name, version`,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	byIndex := make(map[string][]Package)
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.SHA256); err != nil {
			return "", err
		}
		key := p.Distribution + "/" + p.Component + "/" + p.Architecture
		byIndex[key] = append(byIndex[key], p)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	var parts []string
	parts = append(parts, fmt.Sprintf("format=%d", publishFormatVersion))
	parts = append(parts, "publish=latest-only")
	for _, r := range s.repos() {
		parts = append(parts, fmt.Sprintf("repo=%s:%s:%s", r.Name, strings.Join(r.Components, ","), strings.Join(r.Architectures, ",")))
	}
	for key, pkgs := range byIndex {
		latest, err := latestPackages(pkgs)
		if err != nil {
			return "", err
		}
		for _, p := range latest {
			parts = append(parts, fmt.Sprintf("%s/%s=%s", key, p.Name+"/"+p.Version, p.SHA256))
		}
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:]), nil
}

func (s *Service) readPublishFingerprint() (string, error) {
	path := filepath.Join(s.repoDir, ".publish-fingerprint")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func (s *Service) writePublishFingerprint(fingerprint string) error {
	path := filepath.Join(s.repoDir, ".publish-fingerprint")
	return atomicWrite(path, []byte(fingerprint+"\n"))
}

func (s *Service) packageEntryFromPool(p Package, distro, component string) (PackageEntry, error) {
	full := filepath.Join(s.repoDir, p.PoolPath)
	data, err := os.ReadFile(full)
	if err != nil {
		return PackageEntry{}, fmt.Errorf("read pool file %s: %w", p.PoolPath, err)
	}
	ctrl, err := ParseDeb(data)
	if err != nil {
		return PackageEntry{}, fmt.Errorf("parse pool file %s: %w", p.PoolPath, err)
	}
	return PackageEntry{
		Control:   ctrl,
		PoolPath:  p.PoolPath,
		Filename:  p.Filename,
		Size:      int64(len(data)),
		SHA256:    SHA256Hex(data),
		Distro:    distro,
		Component: component,
	}, nil
}
