package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Repository struct {
	Name           string   `json:"name" yaml:"name"`
	Components     []string `json:"components" yaml:"components"`
	Architectures  []string `json:"architectures" yaml:"architectures"`
}

type Config struct {
	DataDir       string       `json:"data_dir" yaml:"data_dir"`
	AdminListen   string       `json:"admin_listen" yaml:"admin_listen"`
	PublicListen  string       `json:"public_listen" yaml:"public_listen"`
	PublicURL     string       `json:"public_url" yaml:"public_url"`
	GPGKeyID      string       `json:"gpg_key_id" yaml:"gpg_key_id"`
	GPGHome       string       `json:"gpg_home" yaml:"gpg_home"`
	Repositories  []Repository `json:"repositories" yaml:"repositories"`
}

func Default() *Config {
	return &Config{
		DataDir:      "./data",
		AdminListen:  "127.0.0.1:8080",
		PublicListen: "0.0.0.0:9090",
		PublicURL:    "http://localhost:9090",
		Repositories: []Repository{
			{Name: "stable", Components: []string{"main"}, Architectures: []string{"amd64", "arm64"}},
			{Name: "testing", Components: []string{"main"}, Architectures: []string{"amd64", "arm64"}},
		},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("read config: %w", err)
			}
		} else {
			if err := yaml.Unmarshal(b, cfg); err != nil {
				return nil, fmt.Errorf("parse config: %w", err)
			}
		}
	}
	cfg.applyEnv()
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) applyEnv() {
	if v := os.Getenv("APTUARY_DATA_DIR"); v != "" {
		c.DataDir = v
	}
	if v := os.Getenv("APTUARY_ADMIN_LISTEN"); v != "" {
		c.AdminListen = v
	}
	if v := os.Getenv("APTUARY_PUBLIC_LISTEN"); v != "" {
		c.PublicListen = v
	}
	if v := os.Getenv("APTUARY_PUBLIC_URL"); v != "" {
		c.PublicURL = v
	}
}

func (c *Config) validate() error {
	if c.DataDir == "" {
		return fmt.Errorf("data_dir is required")
	}
	c.DataDir = filepath.Clean(c.DataDir)
	if c.AdminListen == "" {
		return fmt.Errorf("admin_listen is required")
	}
	if c.PublicListen == "" {
		return fmt.Errorf("public_listen is required")
	}
	if len(c.Repositories) == 0 {
		return fmt.Errorf("at least one repository is required")
	}
	for i, r := range c.Repositories {
		if strings.TrimSpace(r.Name) == "" {
			return fmt.Errorf("repository %d: name is required", i)
		}
		if len(r.Components) == 0 {
			return fmt.Errorf("repository %s: components required", r.Name)
		}
		if len(r.Architectures) == 0 {
			return fmt.Errorf("repository %s: architectures required", r.Name)
		}
	}
	if c.GPGHome == "" {
		c.GPGHome = filepath.Join(c.DataDir, "gpg")
	}
	return nil
}

func (c *Config) RepoDir() string {
	return filepath.Join(c.DataDir, "repo")
}

func (c *Config) ConfigPath() string {
	return filepath.Join(c.DataDir, "config.yaml")
}

func (c *Config) Save() error {
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConfigPath(), b, 0o644)
}

func (c *Config) FindRepo(name string) (*Repository, bool) {
	for i := range c.Repositories {
		if c.Repositories[i].Name == name {
			return &c.Repositories[i], true
		}
	}
	return nil, false
}

func (c *Config) ValidComponent(dist, comp string) bool {
	r, ok := c.FindRepo(dist)
	if !ok {
		return false
	}
	for _, c := range r.Components {
		if c == comp {
			return true
		}
	}
	return false
}

func (c *Config) ValidArch(dist, arch string) bool {
	r, ok := c.FindRepo(dist)
	if !ok {
		return false
	}
	for _, a := range r.Architectures {
		if a == arch {
			return true
		}
	}
	return false
}
