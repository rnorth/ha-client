package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/zalando/go-keyring"
	"gopkg.in/yaml.v3"
)

const (
	keychainService = "ha-client"
	keychainServer  = "server"
	keychainToken   = "token"
)

type Config struct {
	Server string `yaml:"server"`
	Token  string `yaml:"token"`
}

// Resolve returns config using the full resolution chain:
// CLI flags > env vars > OS keychain > config file
func Resolve(serverFlag, tokenFlag string) (*Config, error) {
	return ResolveWithFile(serverFlag, tokenFlag, DefaultConfigPath())
}

func ResolveWithFile(serverFlag, tokenFlag, configFile string) (*Config, error) {
	cfg := &Config{}

	// Layer 4: config file (lowest priority)
	if fileCfg, err := loadFile(configFile); err == nil {
		cfg.Server = fileCfg.Server
		cfg.Token = fileCfg.Token
	}

	// Layer 3: OS keychain
	if server, err := keyring.Get(keychainService, keychainServer); err == nil && server != "" {
		cfg.Server = server
	}
	if token, err := keyring.Get(keychainService, keychainToken); err == nil && token != "" {
		cfg.Token = token
	}

	// Layer 2: environment variables
	if v := os.Getenv("HASS_SERVER"); v != "" {
		cfg.Server = v
	}
	if v := os.Getenv("HASS_TOKEN"); v != "" {
		cfg.Token = v
	}

	// Layer 1: CLI flags (highest priority)
	if serverFlag != "" {
		cfg.Server = serverFlag
	}
	if tokenFlag != "" {
		cfg.Token = tokenFlag
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if c.Server == "" {
		return fmt.Errorf("no server configured: use 'ha-client login', set HASS_SERVER, or use --server")
	}
	if c.Token == "" {
		return fmt.Errorf("no token configured: use 'ha-client login', set HASS_TOKEN, or use --token")
	}
	return nil
}

// SaveToKeychain saves credentials to OS keychain, falling back to config file.
func SaveToKeychain(server, token string) error {
	if err := keyring.Set(keychainService, keychainServer, server); err != nil {
		// Keychain unavailable (headless/container) — fall back to file
		return SaveToFile(server, token, DefaultConfigPath())
	}
	return keyring.Set(keychainService, keychainToken, token)
}

// DeleteFromKeychain removes stored credentials. Safe to call when already logged out.
func DeleteFromKeychain() error {
	for _, key := range []string{keychainServer, keychainToken} {
		if err := keyring.Delete(keychainService, key); err != nil && err != keyring.ErrNotFound {
			return err
		}
	}
	// Also clear config file (ignore missing file)
	_ = os.Remove(DefaultConfigPath())
	return nil
}

func SaveToFile(server, token, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	cfg := Config{Server: server, Token: token}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func loadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func DefaultConfigPath() string {
	if runtime.GOOS == "windows" {
		return filepath.Join(os.Getenv("APPDATA"), "ha-client", "config.yaml")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "ha-client", "config.yaml")
}
