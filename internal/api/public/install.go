package public

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/jdbnet/aptuary/internal/httpx"
)

const keyringPath = "/usr/share/keyrings/aptuary.gpg"

func (s *Server) gpgPublicKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.signer.ExportPublicKey()
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/pgp-keys")
	w.Header().Set("Content-Disposition", "attachment; filename=aptuary.gpg")
	_, _ = w.Write(key)
}

func (s *Server) installScript(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.PathValue("name"))
	distro := strings.TrimSuffix(name, ".sh")
	if distro == "" {
		httpx.WriteErr(w, http.StatusBadRequest, "distribution is required")
		return
	}
	repo, ok := s.cfg.FindRepo(distro)
	if !ok {
		httpx.WriteErr(w, http.StatusNotFound, "unknown distribution")
		return
	}
	component := strings.TrimSpace(r.URL.Query().Get("component"))
	components := repo.Components
	if component != "" {
		if !s.cfg.ValidComponent(distro, component) {
			httpx.WriteErr(w, http.StatusBadRequest, "invalid component for distribution")
			return
		}
		components = []string{component}
	}

	base := strings.TrimRight(s.cfg.PublicURL, "/")
	script := buildInstallScript(base, distro, components)
	w.Header().Set("Content-Type", "text/x-shellscript; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=aptuary-%s.sh", distro))
	_, _ = w.Write([]byte(script))
}

func buildInstallScript(base, distro string, components []string) string {
	componentsLine := strings.Join(components, " ")

	return fmt.Sprintf(`#!/bin/bash
# Aptuary client setup for distribution: %s
# Usage: curl -fsSL %s/install/%s.sh | sudo bash
set -euo pipefail

DISTRO="%s"
BASE="%s"
KEYRING="%s"
SOURCES="/etc/apt/sources.list.d/aptuary-${DISTRO}.sources"

if [ "$(id -u)" -ne 0 ]; then
  echo "Re-run as root: curl -fsSL ${BASE}/install/${DISTRO}.sh | sudo bash"
  exit 1
fi

command -v curl >/dev/null || { echo "curl is required"; exit 1; }
command -v gpg >/dev/null || { echo "gpg is required"; exit 1; }

install -d -m 0755 /usr/share/keyrings /etc/apt/sources.list.d
curl -fsSL "${BASE}/aptuary.gpg" | gpg --dearmor -o "${KEYRING}"
chmod 644 "${KEYRING}"
cat > "${SOURCES}" <<EOF
# Aptuary repository (${DISTRO})
Types: deb
URIs: %s
Suites: ${DISTRO}
Components: %s
Signed-By: ${KEYRING}
EOF
chmod 644 "${SOURCES}"
apt-get update
echo "Aptuary repository ${DISTRO} configured."
echo "Install packages with: apt install <package>"
`, distro, base, distro, distro, base, keyringPath, base, componentsLine)
}
