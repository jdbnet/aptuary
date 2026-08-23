package repo

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

// CompareVersions compares Debian package versions using dpkg rules.
// Returns -1 if a < b, 0 if equal, 1 if a > b.
func CompareVersions(a, b string) int {
	va, errA := parseDebianVersion(a)
	vb, errB := parseDebianVersion(b)
	if errA != nil || errB != nil {
		if a == b {
			return 0
		}
		if a < b {
			return -1
		}
		return 1
	}
	return compareDebianVersion(va, vb)
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

type debVersion struct {
	epoch    uint
	version  string
	revision string
}

func parseDebianVersion(input string) (debVersion, error) {
	var result debVersion
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return result, fmt.Errorf("version string is empty")
	}
	if strings.IndexFunc(trimmed, unicode.IsSpace) != -1 {
		return result, fmt.Errorf("version string has embedded spaces")
	}

	colon := strings.Index(trimmed, ":")
	if colon != -1 {
		epoch, err := strconv.ParseUint(trimmed[:colon], 10, 64)
		if err != nil {
			return result, fmt.Errorf("epoch: %w", err)
		}
		result.epoch = uint(epoch)
	}

	result.version = trimmed[colon+1:]
	if len(result.version) == 0 {
		return result, fmt.Errorf("nothing after colon in version number")
	}
	if hyphen := strings.LastIndex(result.version, "-"); hyphen != -1 {
		result.revision = result.version[hyphen+1:]
		result.version = result.version[:hyphen]
	}

	if len(result.version) == 0 || !unicode.IsDigit(rune(result.version[0])) {
		return result, fmt.Errorf("version number does not start with digit")
	}

	return result, nil
}

func compareDebianVersion(a, b debVersion) int {
	if a.epoch > b.epoch {
		return 1
	}
	if a.epoch < b.epoch {
		return -1
	}
	if rc := verrevcmp(a.version, b.version); rc != 0 {
		return rc
	}
	return verrevcmp(a.revision, b.revision)
}

// verrevcmp compares upstream or revision parts per dpkg version rules.
func verrevcmp(a, b string) int {
	i, j := 0, 0
	for i < len(a) || j < len(b) {
		var firstDiff int
		for (i < len(a) && !isDigit(a[i])) || (j < len(b) && !isDigit(b[j])) {
			ac := orderRune(a, i)
			bc := orderRune(b, j)
			if ac != bc {
				return ac - bc
			}
			if i < len(a) {
				i++
			}
			if j < len(b) {
				j++
			}
		}

		for i < len(a) && a[i] == '0' {
			i++
		}
		for j < len(b) && b[j] == '0' {
			j++
		}

		firstDiff = 0
		for i < len(a) && isDigit(a[i]) && j < len(b) && isDigit(b[j]) {
			if firstDiff == 0 {
				firstDiff = int(a[i]) - int(b[j])
			}
			i++
			j++
		}
		if i < len(a) && isDigit(a[i]) {
			return 1
		}
		if j < len(b) && isDigit(b[j]) {
			return -1
		}
		if firstDiff != 0 {
			return firstDiff
		}
	}
	return 0
}

func isDigit(b byte) bool {
	return b >= '0' && b <= '9'
}

func orderRune(s string, i int) int {
	if i >= len(s) {
		return 0
	}
	r := rune(s[i])
	if isDigit(byte(r)) {
		return 0
	}
	if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
		return int(r)
	}
	if r == '~' {
		return -1
	}
	if r != 0 {
		return int(r) + 256
	}
	return 0
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

	type stalePkg struct {
		id       int64
		poolPath string
	}
	var stale []stalePkg
	for rows.Next() {
		var id int64
		var version, poolPath string
		if err := rows.Scan(&id, &version, &poolPath); err != nil {
			rows.Close()
			return 0, err
		}
		if CompareVersions(version, pkg.Version) >= 0 {
			continue
		}
		stale = append(stale, stalePkg{id: id, poolPath: poolPath})
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, err
	}
	rows.Close()

	pruned := 0
	for _, old := range stale {
		if err := RemoveFromPool(s.repoDir, old.poolPath); err != nil {
			return pruned, err
		}
		if _, err := s.db.Exec(`DELETE FROM packages WHERE id = ?`, old.id); err != nil {
			return pruned, err
		}
		pruned++
	}
	return pruned, nil
}

func (s *Service) pruneAllOlderVersions() error {
	rows, err := s.db.Query(
		`SELECT id, name, version, architecture, distribution, component, pool_path
		 FROM packages ORDER BY distribution, component, architecture, name, version`,
	)
	if err != nil {
		return err
	}

	type key struct {
		name, arch, distro, component string
	}
	latest := make(map[key]Package)
	var all []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.ID, &p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.PoolPath); err != nil {
			rows.Close()
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
		rows.Close()
		return err
	}
	rows.Close()

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
