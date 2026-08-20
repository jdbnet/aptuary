package public

import (
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/jdbnet/aptuary/internal/audit"
	"github.com/jdbnet/aptuary/internal/auth"
	"github.com/jdbnet/aptuary/internal/config"
	"github.com/jdbnet/aptuary/internal/httpx"
	"github.com/jdbnet/aptuary/internal/repo"
)

type Server struct {
	cfg    *config.Config
	repo   *repo.Service
	auth   *auth.Service
	audit  *audit.Log
	signer *repo.Signer
}

func New(cfg *config.Config, repoSvc *repo.Service, authSvc *auth.Service, al *audit.Log, signer *repo.Signer) *Server {
	return &Server{cfg: cfg, repo: repoSvc, auth: authSvc, audit: al, signer: signer}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", s.health)
	mux.HandleFunc("GET /aptuary.gpg", s.gpgPublicKey)
	mux.HandleFunc("GET /install/{name}", s.installScript)
	mux.HandleFunc("POST /api/v1/upload", s.upload)
	mux.HandleFunc("DELETE /api/v1/packages/{name}", s.deletePackage)
	mux.Handle("/pool/", http.StripPrefix("/pool/", s.fileServer("pool")))
	mux.Handle("/dists/", http.StripPrefix("/dists/", s.fileServer("dists")))
	return mux
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) fileServer(prefix string) http.Handler {
	root := filepath.Join(s.repo.RepoDir(), prefix)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clean := filepath.Clean(r.URL.Path)
		if strings.Contains(clean, "..") {
			http.NotFound(w, r)
			return
		}
		full := filepath.Join(root, clean)
		// Repo metadata and pool objects must not be cached by intermediaries across
		// publishes; stale Packages.gz with fresh InRelease causes hash sum mismatches.
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		http.ServeFile(w, r, full)
	})
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	s.auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := auth.ActorFrom(r.Context())
		if !a.Has("packages:write") {
			httpx.WriteErr(w, http.StatusForbidden, "forbidden")
			return
		}
		distro := r.URL.Query().Get("distribution")
		component := r.URL.Query().Get("component")
		if distro == "" || component == "" {
			httpx.WriteErr(w, http.StatusBadRequest, "distribution and component are required")
			return
		}
		if err := r.ParseMultipartForm(512 << 20); err != nil {
			httpx.WriteErr(w, http.StatusBadRequest, "invalid multipart form")
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			httpx.WriteErr(w, http.StatusBadRequest, "file field required")
			return
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		pkg, err := s.repo.Upload(distro, component, data)
		if err != nil {
			httpx.WriteErr(w, http.StatusBadRequest, err.Error())
			return
		}
		_ = s.audit.Record(a.Type, a.ID, "packages.upload", "packages/"+pkg.Name, nil, pkg)
		httpx.WriteJSON(w, http.StatusCreated, pkg)
	})).ServeHTTP(w, r)
}

func (s *Server) deletePackage(w http.ResponseWriter, r *http.Request) {
	s.auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a := auth.ActorFrom(r.Context())
		if !a.Has("packages:write") {
			httpx.WriteErr(w, http.StatusForbidden, "forbidden")
			return
		}
		name := r.PathValue("name")
		version := r.URL.Query().Get("version")
		arch := r.URL.Query().Get("architecture")
		distro := r.URL.Query().Get("distribution")
		component := r.URL.Query().Get("component")
		if name == "" || version == "" || arch == "" || distro == "" || component == "" {
			httpx.WriteErr(w, http.StatusBadRequest, "name, version, architecture, distribution, and component are required")
			return
		}
		pkg, err := s.repo.DeleteByName(name, version, arch, distro, component)
		if err != nil {
			httpx.WriteErr(w, http.StatusNotFound, err.Error())
			return
		}
		_ = s.audit.Record(a.Type, a.ID, "packages.delete", "packages/"+name, pkg, nil)
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true})
	})).ServeHTTP(w, r)
}

// SourcesListLine builds a deb822 sources.list line for a distribution.
func SourcesListLine(publicURL, distro, component, keyPath string) string {
	u := strings.TrimRight(publicURL, "/")
	if keyPath != "" {
		return "deb [signed-by=" + keyPath + "] " + u + " " + distro + " " + component
	}
	return "deb " + u + " " + distro + " " + component
}

func PublicBaseURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	return strings.TrimRight(u.String(), "/")
}
