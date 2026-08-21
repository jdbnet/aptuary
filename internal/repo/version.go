package repo

import (
	"os/exec"
)

// CompareVersions compares Debian package versions using dpkg rules.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func CompareVersions(a, b string) int {
	if a == b {
		return 0
	}
	if compareWithDpkg(a, b, "gt") {
		return 1
	}
	if compareWithDpkg(a, b, "eq") {
		return 0
	}
	return -1
}

func compareWithDpkg(a, b, op string) bool {
	if _, err := exec.LookPath("dpkg"); err != nil {
		return fallbackCompare(a, b, op)
	}
	cmd := exec.Command("dpkg", "--compare-versions", a, op, b)
	return cmd.Run() == nil
}

func fallbackCompare(a, b, op string) bool {
	switch op {
	case "gt":
		return a > b
	case "eq":
		return a == b
	default:
		return false
	}
}

func latestPackages(packages []Package) ([]Package, error) {
	best := make(map[string]Package, len(packages))
	for _, p := range packages {
		cur, ok := best[p.Name]
		if !ok || CompareVersions(p.Version, cur.Version) > 0 {
			best[p.Name] = p
		}
	}
	out := make([]Package, 0, len(best))
	for _, p := range best {
		out = append(out, p)
	}
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Name < out[i].Name || (out[j].Name == out[i].Name && CompareVersions(out[j].Version, out[i].Version) < 0) {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out, nil
}

func (s *Service) pruneOlderVersions(pkg *Package) (int, error) {
	rows, err := s.db.Query(
		`SELECT id, version, pool_path FROM packages
		 WHERE name = ? AND architecture = ? AND distribution = ? AND component = ? AND id != ?`,
		pkg.Name, pkg.Architecture, pkg.Distribution, pkg.Component, pkg.ID,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	pruned := 0
	for rows.Next() {
		var id int64
		var version, poolPath string
		if err := rows.Scan(&id, &version, &poolPath); err != nil {
			return pruned, err
		}
		if CompareVersions(version, pkg.Version) >= 0 {
			continue
		}
		if err := RemoveFromPool(s.repoDir, poolPath); err != nil {
			return pruned, err
		}
		if _, err := s.db.Exec(`DELETE FROM packages WHERE id = ?`, id); err != nil {
			return pruned, err
		}
		pruned++
	}
	return pruned, rows.Err()
}

func (s *Service) pruneAllOlderVersions() error {
	rows, err := s.db.Query(
		`SELECT id, name, version, architecture, distribution, component, pool_path
		 FROM packages ORDER BY distribution, component, architecture, name, version`,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	type key struct {
		name, arch, distro, component string
	}
	latest := make(map[key]Package)
	var all []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.ID, &p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.PoolPath); err != nil {
			return err
		}
		all = append(all, p)
		k := key{p.Name, p.Architecture, p.Distribution, p.Component}
		cur, ok := latest[k]
		if !ok || CompareVersions(p.Version, cur.Version) > 0 {
			latest[k] = p
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range all {
		k := key{p.Name, p.Architecture, p.Distribution, p.Component}
		if p.ID == latest[k].ID {
			continue
		}
		if err := RemoveFromPool(s.repoDir, p.PoolPath); err != nil {
			return err
		}
		if _, err := s.db.Exec(`DELETE FROM packages WHERE id = ?`, p.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) finalizeUpload(pkg *Package, distro string) error {
	if _, err := s.pruneOlderVersions(pkg); err != nil {
		return err
	}
	if err := s.PublishDistribution(distro); err != nil {
		return err
	}
	return s.writePublishFingerprintFromDB()
}
