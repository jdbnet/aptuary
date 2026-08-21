package repo

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Package struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Version       string `json:"version"`
	Architecture  string `json:"architecture"`
	Distribution  string `json:"distribution"`
	Component     string `json:"component"`
	Filename      string `json:"filename"`
	PoolPath      string `json:"pool_path"`
	SHA256        string `json:"sha256"`
	Size          int64  `json:"size"`
	UploadedAt    string `json:"uploaded_at"`
}

type Service struct {
	db       *sql.DB
	repoDir  string
	signer   *Signer
	repos    func() []RepoConfig
	mu       sync.Mutex
	distMu   map[string]*sync.Mutex
}

type RepoConfig struct {
	Name           string
	Components     []string
	Architectures  []string
}

func NewService(db *sql.DB, repoDir string, signer *Signer, repos func() []RepoConfig) *Service {
	return &Service{
		db:      db,
		repoDir: repoDir,
		signer:  signer,
		repos:   repos,
		distMu:  make(map[string]*sync.Mutex),
	}
}

func (s *Service) distLock(distro string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.distMu[distro] == nil {
		s.distMu[distro] = &sync.Mutex{}
	}
	return s.distMu[distro]
}

func (s *Service) Upload(distro, component string, data []byte) (*Package, error) {
	cfg, ok := s.findRepo(distro)
	if !ok {
		return nil, fmt.Errorf("unknown distribution %q", distro)
	}
	validComp := false
	for _, c := range cfg.Components {
		if c == component {
			validComp = true
			break
		}
	}
	if !validComp {
		return nil, fmt.Errorf("invalid component %q for distribution %q", component, distro)
	}

	ctrl, err := ParseDeb(data)
	if err != nil {
		return nil, err
	}
	validArch := false
	for _, a := range cfg.Architectures {
		if a == ctrl.Architecture {
			validArch = true
			break
		}
	}
	if !validArch {
		return nil, fmt.Errorf("architecture %q not allowed for distribution %q", ctrl.Architecture, distro)
	}

	sha := SHA256Hex(data)
	now := time.Now().UTC().Format(time.RFC3339)
	size := int64(len(data))

	existing, _ := s.GetByIdentity(ctrl.Package, ctrl.Version, ctrl.Architecture, distro, component)
	if existing != nil {
		if existing.SHA256 == sha {
			pruned, err := s.pruneOlderVersions(existing)
			if err != nil {
				return nil, err
			}
			if pruned > 0 {
				if err := s.finalizeUpload(existing, distro); err != nil {
					return nil, err
				}
			}
			return existing, nil
		}
		if err := RemoveFromPool(s.repoDir, existing.PoolPath); err != nil {
			return nil, err
		}
		if _, err := s.db.Exec(`DELETE FROM packages WHERE id = ?`, existing.ID); err != nil {
			return nil, err
		}
	}

	filename := PoolFilename(ctrl.Package, ctrl.Version, ctrl.Architecture)
	poolPath, err := PlaceInPool(s.repoDir, component, ctrl.Package, filename, data)
	if err != nil {
		return nil, err
	}

	res, err := s.db.Exec(
		`INSERT INTO packages (name, version, architecture, distribution, component, filename, pool_path, sha256, size, uploaded_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ctrl.Package, ctrl.Version, ctrl.Architecture, distro, component, filename, poolPath, sha, size, now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	pkg := &Package{
		ID: id, Name: ctrl.Package, Version: ctrl.Version, Architecture: ctrl.Architecture,
		Distribution: distro, Component: component, Filename: filename, PoolPath: poolPath,
		SHA256: sha, Size: size, UploadedAt: now,
	}

	if err := s.finalizeUpload(pkg, distro); err != nil {
		return nil, err
	}
	return pkg, nil
}

func (s *Service) Delete(id int64) (*Package, error) {
	pkg, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}
	if err := RemoveFromPool(s.repoDir, pkg.PoolPath); err != nil {
		return nil, err
	}
	_, err = s.db.Exec(`DELETE FROM packages WHERE id = ?`, id)
	if err != nil {
		return nil, err
	}
	if err := s.PublishDistribution(pkg.Distribution); err != nil {
		return nil, err
	}
	if err := s.writePublishFingerprintFromDB(); err != nil {
		return nil, err
	}
	return pkg, nil
}

func (s *Service) DeleteByName(name, version, arch, distro, component string) (*Package, error) {
	pkg, err := s.GetByIdentity(name, version, arch, distro, component)
	if err != nil {
		return nil, err
	}
	return s.Delete(pkg.ID)
}

func (s *Service) PublishDistribution(distro string) error {
	cfg, ok := s.findRepo(distro)
	if !ok {
		return fmt.Errorf("unknown distribution %q", distro)
	}
	lock := s.distLock(distro)
	lock.Lock()
	defer lock.Unlock()

	if err := s.pruneAllOlderVersions(); err != nil {
		return err
	}

	for _, comp := range cfg.Components {
		for _, arch := range cfg.Architectures {
			entries, err := s.packageEntries(distro, comp, arch)
			if err != nil {
				return err
			}
			if err := WritePackagesIndex(s.repoDir, distro, comp, arch, entries); err != nil {
				return err
			}
		}
	}
	if err := WriteRelease(s.repoDir, distro, cfg.Components, cfg.Architectures); err != nil {
		return err
	}
	return s.signer.SignRelease(s.repoDir, distro)
}

func (s *Service) packageEntries(distro, component, arch string) ([]PackageEntry, error) {
	rows, err := s.db.Query(
		`SELECT name, version, architecture, distribution, component, filename, pool_path, sha256, size
		 FROM packages WHERE distribution = ? AND component = ? AND architecture = ?`,
		distro, component, arch,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var packages []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.Filename, &p.PoolPath, &p.SHA256, &p.Size); err != nil {
			return nil, err
		}
		packages = append(packages, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	latest, err := latestPackages(packages)
	if err != nil {
		return nil, err
	}

	var out []PackageEntry
	for _, p := range latest {
		entry, err := s.packageEntryFromPool(p, distro, component)
		if err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *Service) List() ([]Package, error) {
	rows, err := s.db.Query(
		`SELECT id, name, version, architecture, distribution, component, filename, pool_path, sha256, size, uploaded_at
		 FROM packages ORDER BY uploaded_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.ID, &p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.Filename, &p.PoolPath, &p.SHA256, &p.Size, &p.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) GetByID(id int64) (*Package, error) {
	var p Package
	err := s.db.QueryRow(
		`SELECT id, name, version, architecture, distribution, component, filename, pool_path, sha256, size, uploaded_at FROM packages WHERE id = ?`,
		id,
	).Scan(&p.ID, &p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.Filename, &p.PoolPath, &p.SHA256, &p.Size, &p.UploadedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) GetByIdentity(name, version, arch, distro, component string) (*Package, error) {
	var p Package
	err := s.db.QueryRow(
		`SELECT id, name, version, architecture, distribution, component, filename, pool_path, sha256, size, uploaded_at
		 FROM packages WHERE name = ? AND version = ? AND architecture = ? AND distribution = ? AND component = ?`,
		name, version, arch, distro, component,
	).Scan(&p.ID, &p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.Filename, &p.PoolPath, &p.SHA256, &p.Size, &p.UploadedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (s *Service) Stats() (map[string]any, error) {
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM packages`).Scan(&total); err != nil {
		return nil, err
	}
	var diskBytes int64
	rows, err := s.db.Query(`SELECT size FROM packages`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sz int64
		if err := rows.Scan(&sz); err != nil {
			return nil, err
		}
		diskBytes += sz
	}

	byDistro := map[string]int{}
	rows2, err := s.db.Query(`SELECT distribution, COUNT(*) FROM packages GROUP BY distribution`)
	if err != nil {
		return nil, err
	}
	defer rows2.Close()
	for rows2.Next() {
		var d string
		var n int
		if err := rows2.Scan(&d, &n); err != nil {
			return nil, err
		}
		byDistro[d] = n
	}

	recent, err := s.recentUploads(10)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"total_packages": total,
		"disk_bytes":     diskBytes,
		"by_distribution": byDistro,
		"recent_uploads": recent,
	}, nil
}

func (s *Service) recentUploads(limit int) ([]Package, error) {
	rows, err := s.db.Query(
		`SELECT id, name, version, architecture, distribution, component, filename, pool_path, sha256, size, uploaded_at
		 FROM packages ORDER BY uploaded_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Package
	for rows.Next() {
		var p Package
		if err := rows.Scan(&p.ID, &p.Name, &p.Version, &p.Architecture, &p.Distribution, &p.Component, &p.Filename, &p.PoolPath, &p.SHA256, &p.Size, &p.UploadedAt); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Service) RepoDir() string {
	return s.repoDir
}

func (s *Service) DiskUsage() (int64, error) {
	var total int64
	err := filepath.Walk(s.repoDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}

func (s *Service) findRepo(name string) (RepoConfig, bool) {
	for _, r := range s.repos() {
		if r.Name == name {
			return r, true
		}
	}
	return RepoConfig{}, false
}

func (s *Service) PublishAll() error {
	for _, r := range s.repos() {
		if err := s.PublishDistribution(r.Name); err != nil {
			return err
		}
	}
	return s.writePublishFingerprintFromDB()
}

func (s *Service) writePublishFingerprintFromDB() error {
	fingerprint, err := s.packagesFingerprint()
	if err != nil {
		return err
	}
	return s.writePublishFingerprint(fingerprint)
}
