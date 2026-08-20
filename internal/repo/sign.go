package repo

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Signer struct {
	KeyID   string
	GPGHome string
}

func (s *Signer) EnsureKey() (string, error) {
	if err := os.MkdirAll(s.GPGHome, 0o700); err != nil {
		return "", err
	}
	if s.KeyID != "" {
		if err := s.verifyKey(s.KeyID); err == nil {
			return s.KeyID, nil
		}
	}
	keyID, err := s.generateKey()
	if err != nil {
		return "", err
	}
	s.KeyID = keyID
	return keyID, nil
}

func (s *Signer) verifyKey(keyID string) error {
	cmd := exec.Command("gpg", "--homedir", s.GPGHome, "--list-secret-keys", keyID)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+s.GPGHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg list keys: %w: %s", err, string(out))
	}
	return nil
}

func (s *Signer) generateKey() (string, error) {
	batch := fmt.Sprintf(`
%%no-protection
Key-Type: RSA
Key-Length: 4096
Subkey-Type: RSA
Subkey-Length: 4096
Name-Real: Aptuary
Name-Email: aptuary@localhost
Expire-Date: 0
%%commit
`)
	cmd := exec.Command("gpg", "--homedir", s.GPGHome, "--batch", "--gen-key")
	cmd.Env = append(os.Environ(), "GNUPGHOME="+s.GPGHome)
	cmd.Stdin = strings.NewReader(batch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gpg gen-key: %w: %s", err, string(out))
	}
	cmd = exec.Command("gpg", "--homedir", s.GPGHome, "--list-secret-keys", "--keyid-format", "long")
	cmd.Env = append(os.Environ(), "GNUPGHOME="+s.GPGHome)
	out, err = cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gpg list keys: %w: %s", err, string(out))
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "sec") {
			parts := strings.Fields(line)
			for _, p := range parts {
				if strings.HasPrefix(p, "rsa") && strings.Contains(p, "/") {
					idx := strings.Index(p, "/")
					return p[idx+1:], nil
				}
			}
		}
	}
	return "", fmt.Errorf("could not parse generated key id")
}

func (s *Signer) SignRelease(repoDir, distro string) error {
	releasePath := filepath.Join(repoDir, "dists", distro, "Release")
	releaseData, err := os.ReadFile(releasePath)
	if err != nil {
		return err
	}

	// Detached signature Release.gpg
	gpgPath := filepath.Join(repoDir, "dists", distro, "Release.gpg")
	if err := s.detachSign(releasePath, gpgPath); err != nil {
		return err
	}

	// Cleartext signed InRelease
	inReleasePath := filepath.Join(repoDir, "dists", distro, "InRelease")
	if err := s.clearsign(releaseData, inReleasePath); err != nil {
		return err
	}
	return nil
}

func (s *Signer) detachSign(inputPath, outputPath string) error {
	cmd := exec.Command("gpg", "--homedir", s.GPGHome, "--batch", "--yes", "--local-user", s.KeyID,
		"--armor", "--detach-sign", "--output", outputPath, inputPath)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+s.GPGHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg detach-sign: %w: %s", err, string(out))
	}
	return nil
}

func (s *Signer) clearsign(data []byte, outputPath string) error {
	cmd := exec.Command("gpg", "--homedir", s.GPGHome, "--batch", "--yes", "--local-user", s.KeyID,
		"--armor", "--clearsign", "--output", outputPath)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+s.GPGHome)
	cmd.Stdin = bytes.NewReader(data)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gpg clearsign: %w: %s", err, string(out))
	}
	return nil
}

func (s *Signer) ExportPublicKey() ([]byte, error) {
	cmd := exec.Command("gpg", "--homedir", s.GPGHome, "--batch", "--armor", "--export", s.KeyID)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+s.GPGHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("gpg export: %w: %s", err, string(out))
	}
	return out, nil
}

func (s *Signer) Fingerprint() (string, error) {
	cmd := exec.Command("gpg", "--homedir", s.GPGHome, "--fingerprint", "--keyid-format", "long", s.KeyID)
	cmd.Env = append(os.Environ(), "GNUPGHOME="+s.GPGHome)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gpg fingerprint: %w: %s", err, string(out))
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "Key fingerprint") {
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1]), nil
			}
		}
	}
	return "", nil
}
