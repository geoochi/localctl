// Package config loads configuration from a .env file and/or ~/.localctl/config.json.
//
// Precedence: process env > .env file > config file > generated defaults.
// Supported variables:
//
//	LOCALCTL_ADDR      listen address (default 127.0.0.1:7788)
//	LOCALCTL_PASSWORD  login password (plaintext; hashed at startup)
//	LOCALCTL_SECRET    HMAC key for session cookies (any random string)
package config

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Config is the effective runtime configuration.
type Config struct {
	PasswordHash string `json:"password_hash"`
	SecretKey    string `json:"secret_key"` // HMAC key for session cookies
	ListenAddr   string `json:"listen_addr"`
}

// LoadDotEnv reads key=value pairs from path into the process environment.
// Existing process env values are never overridden. Missing file is not an error.
func LoadDotEnv(path string) error {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' && val[len(val)-1] == '"' || val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
	return sc.Err()
}

// Path returns ~/.localctl/config.json.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".localctl", "config.json"), nil
}

// Load reads the config file. Returns (nil, nil) when the file does not exist.
func Load() (*Config, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}

// Save writes the config to disk with 0700/0600 permissions.
func Save(cfg *Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// Ensure resolves the effective configuration and returns it along with the
// generated plaintext password on first run without env config (empty otherwise).
func Ensure(listenAddr string) (*Config, string, error) {
	cfg, err := Load()
	if err != nil {
		return nil, "", err
	}
	if cfg == nil {
		cfg = &Config{}
	}

	// Environment overrides (.env is loaded into process env beforehand).
	if v := os.Getenv("LOCALCTL_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("LOCALCTL_SECRET"); v != "" {
		cfg.SecretKey = v
	}
	envPassword := os.Getenv("LOCALCTL_PASSWORD")

	if cfg.ListenAddr == "" {
		cfg.ListenAddr = listenAddr
	}
	if cfg.SecretKey == "" {
		secret := make([]byte, 32)
		if _, err := rand.Read(secret); err != nil {
			return nil, "", err
		}
		cfg.SecretKey = base64.RawURLEncoding.EncodeToString(secret)
	}

	switch {
	case envPassword != "":
		// Hash the env-provided password in memory; nothing persisted.
		hash, err := bcrypt.GenerateFromPassword([]byte(envPassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, "", err
		}
		cfg.PasswordHash = string(hash)
	case cfg.PasswordHash == "":
		// No env password and no stored hash: generate one and persist.
		pwBytes := make([]byte, 12)
		if _, err := rand.Read(pwBytes); err != nil {
			return nil, "", err
		}
		password := base64.RawURLEncoding.EncodeToString(pwBytes)

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return nil, "", err
		}
		cfg.PasswordHash = string(hash)
		if err := Save(cfg); err != nil {
			return nil, "", err
		}
		return cfg, password, nil
	}

	return cfg, "", nil
}

// CheckPassword verifies a plaintext password against the stored hash.
func (c *Config) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(c.PasswordHash), []byte(password)) == nil
}
