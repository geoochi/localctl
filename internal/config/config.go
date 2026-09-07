// Package config resolves runtime configuration from environment variables
// and a .env file (process env > .env > generated defaults).
//
// Supported variables:
//
//	LOCALCTL_ADDR      listen address (default 127.0.0.1:8003)
//	LOCALCTL_PASSWORD  login password (plaintext; hashed at startup)
//	LOCALCTL_SECRET    HMAC key for session tokens (set it in .env to keep
//	                   tokens valid across restarts; otherwise ephemeral)
package config

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Config is the effective runtime configuration.
type Config struct {
	PasswordHash string
	SecretKey    string
	ListenAddr   string
}

// LoadDotEnv reads key=value pairs into the process environment.
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

// LoadDotEnvAnywhere loads .env from the working directory, falling back to
// the directory of the running executable.
func LoadDotEnvAnywhere() error {
	if err := LoadDotEnv(".env"); err != nil {
		return err
	}
	if os.Getenv("LOCALCTL_ADDR") == "" && os.Getenv("LOCALCTL_PASSWORD") == "" {
		if exe, err := os.Executable(); err == nil {
			return LoadDotEnv(filepath.Join(filepath.Dir(exe), ".env"))
		}
	}
	return nil
}

// randomString returns a URL-safe random string of n bytes entropy.
func randomString(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// Ensure resolves the effective configuration. Returns the config and, when
// a random password had to be generated (no LOCALCTL_PASSWORD configured),
// that plaintext password for display.
func Ensure(defaultAddr string) (*Config, string, error) {
	cfg := &Config{
		ListenAddr: os.Getenv("LOCALCTL_ADDR"),
		SecretKey:  os.Getenv("LOCALCTL_SECRET"),
	}
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultAddr
	}

	password := os.Getenv("LOCALCTL_PASSWORD")
	if password == "" {
		// No configured password: generate one for this run and print it.
		pw, err := randomString(12)
		if err != nil {
			return nil, "", err
		}
		password = pw
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, "", err
	}
	cfg.PasswordHash = string(hash)

	if cfg.SecretKey == "" {
		secret, err := randomString(32)
		if err != nil {
			return nil, "", err
		}
		cfg.SecretKey = secret
	}

	if os.Getenv("LOCALCTL_PASSWORD") == "" {
		return cfg, password, nil
	}
	return cfg, "", nil
}

// CheckPassword verifies a plaintext password against the stored hash.
func (c *Config) CheckPassword(password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(c.PasswordHash), []byte(password)) == nil
}
