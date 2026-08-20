package repo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func PoolPath(component, pkgName, filename string) string {
	letter := "x"
	if pkgName != "" {
		letter = strings.ToLower(pkgName[:1])
	}
	return filepath.Join("pool", component, letter, pkgName, filename)
}

func PoolFilename(pkg, version, arch string) string {
	return fmt.Sprintf("%s_%s_%s.deb", pkg, version, arch)
}

func PlaceInPool(repoDir, component, pkgName, filename string, data []byte) (string, error) {
	rel := PoolPath(component, pkgName, filename)
	full := filepath.Join(repoDir, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, full); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return rel, nil
}

func RemoveFromPool(repoDir, poolPath string) error {
	full := filepath.Join(repoDir, poolPath)
	if err := os.Remove(full); err != nil && !os.IsNotExist(err) {
		return err
	}
	// Clean empty parent dirs up to pool/
	dir := filepath.Dir(full)
	poolRoot := filepath.Join(repoDir, "pool")
	for strings.HasPrefix(dir, poolRoot) && dir != poolRoot {
		if err := os.Remove(dir); err != nil {
			break
		}
		dir = filepath.Dir(dir)
	}
	return nil
}
