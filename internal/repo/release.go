package repo

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type IndexFile struct {
	Path   string
	Size   int64
	SHA256 string
}

func WriteRelease(repoDir, distro string, components, architectures []string) error {
	var indexes []IndexFile
	distsDir := filepath.Join(repoDir, "dists", distro)

	for _, comp := range components {
		for _, arch := range architectures {
			for _, name := range []string{"Packages", "Packages.gz"} {
				rel := filepath.Join("dists", distro, comp, "binary-"+arch, name)
				full := filepath.Join(repoDir, rel)
				info, err := os.Stat(full)
				if err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return err
				}
				data, err := os.ReadFile(full)
				if err != nil {
					return err
				}
				sum := sha256.Sum256(data)
				indexes = append(indexes, IndexFile{
					Path:   rel,
					Size:   info.Size(),
					SHA256: hex.EncodeToString(sum[:]),
				})
			}
		}
	}

	var b strings.Builder
	b.WriteString("Origin: Aptuary\n")
	b.WriteString("Label: Aptuary\n")
	b.WriteString("Suite: ")
	b.WriteString(distro)
	b.WriteString("\n")
	b.WriteString("Version: 1.0\n")
	b.WriteString("Codename: ")
	b.WriteString(distro)
	b.WriteString("\n")
	b.WriteString("Date: ")
	b.WriteString(time.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05 MST"))
	b.WriteString("\n")
	b.WriteString("Architectures: ")
	b.WriteString(strings.Join(architectures, " "))
	b.WriteString("\n")
	b.WriteString("Components: ")
	b.WriteString(strings.Join(components, " "))
	b.WriteString("\n")
	b.WriteString("Description: Aptuary APT repository\n")
	for _, idx := range indexes {
		b.WriteString(fmt.Sprintf("SHA256: %s %d %s\n", idx.SHA256, idx.Size, idx.Path))
	}

	releasePath := filepath.Join(distsDir, "Release")
	return atomicWrite(releasePath, []byte(b.String()))
}
