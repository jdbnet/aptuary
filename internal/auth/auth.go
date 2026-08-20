package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const cookieName = "aptuary_session"

const (
	DefaultAdminUser     = "admin"
	DefaultAdminPassword = "changeme"
)

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

var roleScopes = map[Role][]string{
	RoleAdmin:    allScopes(),
	RoleOperator: operatorScopes(),
	RoleViewer:   viewerScopes(),
}

func AllScopes() []string {
	resources := []string{"packages", "repos", "users", "apikeys", "audit", "stats", "settings", "gpg"}
	var out []string
	for _, r := range resources {
		out = append(out, r+":read", r+":write")
	}
	return out
}

func allScopes() []string {
	return AllScopes()
}

func operatorScopes() []string {
	var out []string
	for _, r := range []string{"packages", "repos", "stats", "settings", "gpg"} {
		out = append(out, r+":read", r+":write")
	}
	out = append(out, "audit:read")
	return out
}

func viewerScopes() []string {
	var out []string
	for _, r := range []string{"packages", "repos", "stats", "audit", "gpg"} {
		out = append(out, r+":read")
	}
	return out
}

func ValidScope(s string) bool {
	for _, x := range allScopes() {
		if x == s {
			return true
		}
	}
	return false
}

func NormalizeScopes(in []string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !ValidScope(s) {
			return nil, fmt.Errorf("unknown scope %q", s)
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one scope is required")
	}
	return out, nil
}

type User struct {
	ID           int64  `json:"id"`
	Username     string `json:"username"`
	PasswordHash string `json:"-"`
	Role         Role   `json:"role"`
	CreatedAt    string `json:"created_at"`
}

type APIKey struct {
	ID         int64    `json:"id"`
	Name       string   `json:"name"`
	Prefix     string   `json:"prefix"`
	Scopes     []string `json:"scopes"`
	CreatedBy  *int64   `json:"created_by,omitempty"`
	CreatedAt  string   `json:"created_at"`
	LastUsedAt *string  `json:"last_used_at,omitempty"`
}

type Actor struct {
	Type   string
	ID     string
	User   *User
	Key    *APIKey
	Scopes []string
}

type contextKey int

const actorKey contextKey = 1

func ActorFrom(ctx context.Context) *Actor {
	a, _ := ctx.Value(actorKey).(*Actor)
	return a
}

func WithActor(ctx context.Context, a *Actor) context.Context {
	return context.WithValue(ctx, actorKey, a)
}

func (a *Actor) Has(scope string) bool {
	if a == nil {
		return false
	}
	for _, s := range a.Scopes {
		if s == scope {
			return true
		}
	}
	return false
}

type Service struct {
	db *sql.DB
}

func New(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) BootstrapAdmin(envUser, envPass string) (username string, created bool, err error) {
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n); err != nil {
		return "", false, err
	}
	if n > 0 {
		return "", false, nil
	}
	username, password := DefaultAdminUser, DefaultAdminPassword
	if envUser != "" && envPass != "" {
		username, password = envUser, envPass
	}
	if _, err := s.CreateUser(username, password, RoleAdmin); err != nil {
		return "", false, err
	}
	return username, true, nil
}

func (s *Service) CreateUser(username, password string, role Role) (*User, error) {
	if role != RoleAdmin && role != RoleOperator && role != RoleViewer {
		return nil, fmt.Errorf("invalid role")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.Exec(
		`INSERT INTO users (username, password_hash, role, created_at) VALUES (?, ?, ?, ?)`,
		username, string(hash), string(role), now,
	)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return &User{ID: id, Username: username, Role: role, CreatedAt: now}, nil
}

func (s *Service) ChangePassword(id int64, current, next string) error {
	if current == "" || next == "" {
		return fmt.Errorf("current and new password are required")
	}
	if current == next {
		return fmt.Errorf("new password must be different")
	}
	u, err := s.GetUser(id)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(current)); err != nil {
		return fmt.Errorf("current password is incorrect")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, string(hash), id)
	return err
}

func (s *Service) UpdateUser(id int64, password string, role Role) error {
	u, err := s.GetUser(id)
	if err != nil {
		return err
	}
	if role != "" {
		if role != RoleAdmin && role != RoleOperator && role != RoleViewer {
			return fmt.Errorf("invalid role")
		}
		u.Role = role
	}
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		u.PasswordHash = string(hash)
	}
	_, err = s.db.Exec(`UPDATE users SET password_hash = ?, role = ? WHERE id = ?`, u.PasswordHash, string(u.Role), id)
	return err
}

func (s *Service) DeleteUser(id int64) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	return err
}

func (s *Service) GetUser(id int64) (*User, error) {
	var u User
	var role string
	err := s.db.QueryRow(`SELECT id, username, password_hash, role, created_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.Role = Role(role)
	return &u, nil
}

func (s *Service) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, created_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var role string
		if err := rows.Scan(&u.ID, &u.Username, &role, &u.CreatedAt); err != nil {
			return nil, err
		}
		u.Role = Role(role)
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Service) Authenticate(username, password string) (*User, error) {
	var u User
	var role string
	err := s.db.QueryRow(`SELECT id, username, password_hash, role, created_at FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &role, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	u.Role = Role(role)
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}
	return &u, nil
}

func (s *Service) CreateSession(userID int64) (string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	token := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(token))
	expires := time.Now().UTC().Add(24 * time.Hour).Format(time.RFC3339)
	_, err := s.db.Exec(`INSERT INTO sessions (user_id, token_hash, expires_at) VALUES (?, ?, ?)`, userID, hex.EncodeToString(sum[:]), expires)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *Service) SessionUser(token string) (*User, error) {
	sum := sha256.Sum256([]byte(token))
	var userID int64
	var expires string
	err := s.db.QueryRow(`SELECT user_id, expires_at FROM sessions WHERE token_hash = ?`, hex.EncodeToString(sum[:])).
		Scan(&userID, &expires)
	if err != nil {
		return nil, err
	}
	exp, err := time.Parse(time.RFC3339, expires)
	if err != nil || time.Now().UTC().After(exp) {
		return nil, fmt.Errorf("session expired")
	}
	return s.GetUser(userID)
}

func (s *Service) DeleteSession(token string) error {
	sum := sha256.Sum256([]byte(token))
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token_hash = ?`, hex.EncodeToString(sum[:]))
	return err
}

func (s *Service) CreateAPIKey(name string, scopes []string, createdBy *int64) (plaintext string, key *APIKey, err error) {
	scopes, err = NormalizeScopes(scopes)
	if err != nil {
		return "", nil, err
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	plaintext = "apk_" + hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plaintext))
	now := time.Now().UTC().Format(time.RFC3339)
	scopeJSON, _ := json.Marshal(scopes)
	prefix := plaintext[:12]
	res, err := s.db.Exec(
		`INSERT INTO api_keys (name, key_hash, prefix, scopes, created_by, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		name, hex.EncodeToString(sum[:]), prefix, string(scopeJSON), createdBy, now,
	)
	if err != nil {
		return "", nil, err
	}
	id, _ := res.LastInsertId()
	return plaintext, &APIKey{ID: id, Name: name, Prefix: prefix, Scopes: scopes, CreatedBy: createdBy, CreatedAt: now}, nil
}

func (s *Service) LookupAPIKey(token string) (*APIKey, error) {
	sum := sha256.Sum256([]byte(token))
	var k APIKey
	var scopes string
	var last sql.NullString
	var createdBy sql.NullInt64
	err := s.db.QueryRow(
		`SELECT id, name, prefix, scopes, created_by, created_at, last_used_at FROM api_keys WHERE key_hash = ?`,
		hex.EncodeToString(sum[:]),
	).Scan(&k.ID, &k.Name, &k.Prefix, &scopes, &createdBy, &k.CreatedAt, &last)
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(scopes), &k.Scopes)
	if createdBy.Valid {
		k.CreatedBy = &createdBy.Int64
	}
	if last.Valid {
		k.LastUsedAt = &last.String
	}
	_, _ = s.db.Exec(`UPDATE api_keys SET last_used_at = ? WHERE id = ?`, time.Now().UTC().Format(time.RFC3339), k.ID)
	return &k, nil
}

func (s *Service) ListAPIKeys() ([]APIKey, error) {
	rows, err := s.db.Query(`SELECT id, name, prefix, scopes, created_by, created_at, last_used_at FROM api_keys ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []APIKey
	for rows.Next() {
		var k APIKey
		var scopes string
		var last sql.NullString
		var createdBy sql.NullInt64
		if err := rows.Scan(&k.ID, &k.Name, &k.Prefix, &scopes, &createdBy, &k.CreatedAt, &last); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(scopes), &k.Scopes)
		if createdBy.Valid {
			k.CreatedBy = &createdBy.Int64
		}
		if last.Valid {
			k.LastUsedAt = &last.String
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

func (s *Service) DeleteAPIKey(id int64) error {
	_, err := s.db.Exec(`DELETE FROM api_keys WHERE id = ?`, id)
	return err
}

func (s *Service) scanAPIKey(row interface {
	Scan(dest ...any) error
}) (*APIKey, error) {
	var k APIKey
	var scopes string
	var last sql.NullString
	var createdBy sql.NullInt64
	if err := row.Scan(&k.ID, &k.Name, &k.Prefix, &scopes, &createdBy, &k.CreatedAt, &last); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(scopes), &k.Scopes)
	if createdBy.Valid {
		k.CreatedBy = &createdBy.Int64
	}
	if last.Valid {
		k.LastUsedAt = &last.String
	}
	return &k, nil
}

func (s *Service) UpdateAPIKeyScopes(id int64, scopes []string) (*APIKey, error) {
	scopes, err := NormalizeScopes(scopes)
	if err != nil {
		return nil, err
	}
	scopeJSON, _ := json.Marshal(scopes)
	res, err := s.db.Exec(`UPDATE api_keys SET scopes = ? WHERE id = ?`, string(scopeJSON), id)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return nil, fmt.Errorf("api key not found")
	}
	return s.scanAPIKey(s.db.QueryRow(
		`SELECT id, name, prefix, scopes, created_by, created_at, last_used_at FROM api_keys WHERE id = ?`,
		id,
	))
}

func SetSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400,
	})
}

func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
}

func (s *Service) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if authz := r.Header.Get("Authorization"); strings.HasPrefix(authz, "Bearer ") {
			token := strings.TrimPrefix(authz, "Bearer ")
			key, err := s.LookupAPIKey(token)
			if err != nil {
				http.Error(w, `{"error":"invalid api key"}`, http.StatusUnauthorized)
				return
			}
			actorID := key.Name
			if actorID == "" {
				actorID = key.Prefix
			}
			actor := &Actor{Type: "apikey", ID: actorID, Key: key, Scopes: key.Scopes}
			next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), actor)))
			return
		}
		c, err := r.Cookie(cookieName)
		if err != nil || c.Value == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		u, err := s.SessionUser(c.Value)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		actor := &Actor{Type: "user", ID: u.Username, User: u, Scopes: roleScopes[u.Role]}
		next.ServeHTTP(w, r.WithContext(WithActor(r.Context(), actor)))
	})
}

func Require(scope string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a := ActorFrom(r.Context())
		if !a.Has(scope) {
			http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func HasUsers(db *sql.DB) (bool, error) {
	var n int
	err := db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n > 0, err
}
