package admin

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/jdbnet/aptuary/internal/audit"
	"github.com/jdbnet/aptuary/internal/auth"
	"github.com/jdbnet/aptuary/internal/config"
	"github.com/jdbnet/aptuary/internal/httpx"
	"github.com/jdbnet/aptuary/internal/repo"
	"github.com/jdbnet/aptuary/ui"
)

type Server struct {
	cfg      *config.Config
	auth     *auth.Service
	audit    *audit.Log
	repo     *repo.Service
	signer   *repo.Signer
	hasUsers bool
	version  string
}

func New(cfg *config.Config, authSvc *auth.Service, al *audit.Log, repoSvc *repo.Service, signer *repo.Signer, hasUsers bool, version string) *Server {
	return &Server{
		cfg:      cfg,
		auth:     authSvc,
		audit:    al,
		repo:     repoSvc,
		signer:   signer,
		hasUsers: hasUsers,
		version:  version,
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health", s.health)
	mux.HandleFunc("GET /api/v1/auth/status", s.authStatus)
	mux.HandleFunc("POST /api/v1/auth/login", s.login)

	prot := http.NewServeMux()
	prot.HandleFunc("POST /api/v1/auth/logout", s.logout)
	prot.HandleFunc("GET /api/v1/auth/me", s.me)
	prot.HandleFunc("POST /api/v1/auth/password", s.changePassword)
	prot.HandleFunc("GET /api/v1/stats", auth.Require("stats:read", s.stats))
	prot.HandleFunc("GET /api/v1/packages", auth.Require("packages:read", s.listPackages))
	prot.HandleFunc("DELETE /api/v1/packages/{id}", auth.Require("packages:write", s.deletePackage))
	prot.HandleFunc("GET /api/v1/repositories", auth.Require("repos:read", s.getRepositories))
	prot.HandleFunc("PUT /api/v1/repositories", auth.Require("repos:write", s.putRepositories))
	prot.HandleFunc("GET /api/v1/users", auth.Require("users:read", s.listUsers))
	prot.HandleFunc("POST /api/v1/users", auth.Require("users:write", s.createUser))
	prot.HandleFunc("PUT /api/v1/users/{id}", auth.Require("users:write", s.updateUser))
	prot.HandleFunc("DELETE /api/v1/users/{id}", auth.Require("users:write", s.deleteUser))
	prot.HandleFunc("GET /api/v1/apikeys", auth.Require("apikeys:read", s.listKeys))
	prot.HandleFunc("POST /api/v1/apikeys", auth.Require("apikeys:write", s.createKey))
	prot.HandleFunc("PUT /api/v1/apikeys/{id}", auth.Require("apikeys:write", s.updateKey))
	prot.HandleFunc("DELETE /api/v1/apikeys/{id}", auth.Require("apikeys:write", s.deleteKey))
	prot.HandleFunc("GET /api/v1/audit", auth.Require("audit:read", s.listAudit))
	prot.HandleFunc("GET /api/v1/settings", auth.Require("settings:read", s.getSettings))
	prot.HandleFunc("GET /api/v1/gpg/status", auth.Require("gpg:read", s.gpgStatus))
	prot.HandleFunc("GET /api/v1/gpg/public-key", auth.Require("gpg:read", s.gpgPublicKey))

	mux.Handle("/api/v1/", s.protect(prot))
	mux.Handle("/", spaHandler())
	return mux
}

func (s *Server) protect(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hasUsers {
			actor := &auth.Actor{Type: "system", ID: "bootstrap", Scopes: auth.AllScopes()}
			h.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
			return
		}
		s.auth.Middleware(h).ServeHTTP(w, r)
	})
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok", "version": s.version})
}

func (s *Server) authStatus(w http.ResponseWriter, r *http.Request) {
	authenticated := false
	if c, err := r.Cookie("aptuary_session"); err == nil && c.Value != "" {
		if _, err := s.auth.SessionUser(c.Value); err == nil {
			authenticated = true
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"auth_required": s.hasUsers,
		"authenticated": authenticated,
		"version":       s.version,
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	u, err := s.auth.Authenticate(req.Username, req.Password)
	if err != nil {
		httpx.WriteErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := s.auth.CreateSession(u.ID)
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	auth.SetSessionCookie(w, token)
	_ = s.audit.Record("user", u.Username, "auth.login", "session", nil, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ok": true, "user": u})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("aptuary_session"); err == nil {
		_ = s.auth.DeleteSession(c.Value)
	}
	auth.ClearSessionCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	a := auth.ActorFrom(r.Context())
	httpx.WriteJSON(w, http.StatusOK, a)
}

func (s *Server) changePassword(w http.ResponseWriter, r *http.Request) {
	a := auth.ActorFrom(r.Context())
	if a == nil || a.User == nil {
		httpx.WriteErr(w, http.StatusBadRequest, "password can only be changed from a user session")
		return
	}
	var req struct {
		Current string `json:"current_password"`
		Next    string `json:"new_password"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := s.auth.ChangePassword(a.User.ID, req.Current, req.Next); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.audit.Record(a.Type, a.ID, "auth.password", "users/"+a.User.Username, nil, nil)
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) stats(w http.ResponseWriter, r *http.Request) {
	st, err := s.repo.Stats()
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	disk, _ := s.repo.DiskUsage()
	st["repo_disk_bytes"] = disk
	st["gpg_key_id"] = s.cfg.GPGKeyID
	st["public_url"] = s.cfg.PublicURL
	st["repositories"] = s.cfg.Repositories
	httpx.WriteJSON(w, http.StatusOK, st)
}

func (s *Server) listPackages(w http.ResponseWriter, r *http.Request) {
	pkgs, err := s.repo.List()
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, pkgs)
}

func (s *Server) deletePackage(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := parseID(idStr, &id); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	before, _ := s.repo.GetByID(id)
	pkg, err := s.repo.Delete(id)
	if err != nil {
		httpx.WriteErr(w, http.StatusNotFound, err.Error())
		return
	}
	a := auth.ActorFrom(r.Context())
	_ = s.audit.Record(a.Type, a.ID, "packages.delete", "packages/"+idStr, before, pkg)
	httpx.WriteJSON(w, http.StatusOK, pkg)
}

func (s *Server) getRepositories(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, s.cfg.Repositories)
}

func (s *Server) putRepositories(w http.ResponseWriter, r *http.Request) {
	var repos []config.Repository
	if !httpx.DecodeJSON(w, r, &repos) {
		return
	}
	if len(repos) == 0 {
		httpx.WriteErr(w, http.StatusBadRequest, "at least one repository is required")
		return
	}
	for i, repo := range repos {
		if strings.TrimSpace(repo.Name) == "" {
			httpx.WriteErr(w, http.StatusBadRequest, fmt.Sprintf("repository %d: name is required", i))
			return
		}
		if len(repo.Components) == 0 || len(repo.Architectures) == 0 {
			httpx.WriteErr(w, http.StatusBadRequest, fmt.Sprintf("repository %s: components and architectures required", repo.Name))
			return
		}
	}
	before := s.cfg.Repositories
	s.cfg.Repositories = repos
	if err := s.cfg.Save(); err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.repo.PublishAll(); err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	a := auth.ActorFrom(r.Context())
	_ = s.audit.Record(a.Type, a.ID, "repos.update", "repositories", before, repos)
	httpx.WriteJSON(w, http.StatusOK, repos)
}

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.auth.ListUsers()
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, users)
}

func (s *Server) createUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	u, err := s.auth.CreateUser(req.Username, req.Password, auth.Role(req.Role))
	if err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a := auth.ActorFrom(r.Context())
	_ = s.audit.Record(a.Type, a.ID, "users.create", "users/"+u.Username, nil, u)
	httpx.WriteJSON(w, http.StatusCreated, u)
}

func (s *Server) updateUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := parseID(idStr, &id); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Password string `json:"password"`
		Role     string `json:"role"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if err := s.auth.UpdateUser(id, req.Password, auth.Role(req.Role)); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, err.Error())
		return
	}
	u, _ := s.auth.GetUser(id)
	a := auth.ActorFrom(r.Context())
	_ = s.audit.Record(a.Type, a.ID, "users.update", "users/"+idStr, nil, u)
	httpx.WriteJSON(w, http.StatusOK, u)
}

func (s *Server) deleteUser(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := parseID(idStr, &id); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.auth.DeleteUser(id); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a := auth.ActorFrom(r.Context())
	_ = s.audit.Record(a.Type, a.ID, "users.delete", "users/"+idStr, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := s.auth.ListAPIKeys()
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, keys)
}

func (s *Server) createKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name   string   `json:"name"`
		Scopes []string `json:"scopes"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	var createdBy *int64
	a := auth.ActorFrom(r.Context())
	if a.User != nil {
		createdBy = &a.User.ID
	}
	plaintext, key, err := s.auth.CreateAPIKey(req.Name, req.Scopes, createdBy)
	if err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = s.audit.Record(a.Type, a.ID, "apikeys.create", "apikeys/"+key.Prefix, nil, key)
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"key": key, "token": plaintext})
}

func (s *Server) updateKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := parseID(idStr, &id); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req struct {
		Scopes []string `json:"scopes"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	key, err := s.auth.UpdateAPIKeyScopes(id, req.Scopes)
	if err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a := auth.ActorFrom(r.Context())
	_ = s.audit.Record(a.Type, a.ID, "apikeys.update", "apikeys/"+idStr, nil, key)
	httpx.WriteJSON(w, http.StatusOK, key)
}

func (s *Server) deleteKey(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	var id int64
	if _, err := parseID(idStr, &id); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := s.auth.DeleteAPIKey(id); err != nil {
		httpx.WriteErr(w, http.StatusBadRequest, err.Error())
		return
	}
	a := auth.ActorFrom(r.Context())
	_ = s.audit.Record(a.Type, a.ID, "apikeys.delete", "apikeys/"+idStr, nil, nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	entries, err := s.audit.List(100, 0)
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusOK, entries)
}

func (s *Server) getSettings(w http.ResponseWriter, r *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"data_dir":       s.cfg.DataDir,
		"admin_listen":   s.cfg.AdminListen,
		"public_listen":  s.cfg.PublicListen,
		"public_url":     s.cfg.PublicURL,
		"gpg_key_id":     s.cfg.GPGKeyID,
		"gpg_home":       s.cfg.GPGHome,
	})
}

func (s *Server) gpgStatus(w http.ResponseWriter, r *http.Request) {
	fp, _ := s.signer.Fingerprint()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"key_id":      s.cfg.GPGKeyID,
		"fingerprint": fp,
	})
}

func (s *Server) gpgPublicKey(w http.ResponseWriter, r *http.Request) {
	key, err := s.signer.ExportPublicKey()
	if err != nil {
		httpx.WriteErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=aptuary.gpg")
	_, _ = w.Write(key)
}

func spaHandler() http.Handler {
	sub, err := fs.Sub(ui.FS, "dist")
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "ui not built", http.StatusNotFound)
		})
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			f, err := sub.Open(strings.TrimPrefix(r.URL.Path, "/"))
			if err == nil {
				_ = f.Close()
				fileServer.ServeHTTP(w, r)
				return
			}
		}
		index, err := sub.Open("index.html")
		if err != nil {
			http.Error(w, "ui not built", http.StatusNotFound)
			return
		}
		defer index.Close()
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.Copy(w, index)
	})
}

func parseID(s string, v *int64) (int64, error) {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	*v = n
	return n, nil
}
