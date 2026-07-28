package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// Config holds CLI connection settings.
type Config struct {
	APIURL   string `json:"apiUrl"`
	Token    string `json:"token"`
	TenantID string `json:"tenantId"`
}

// FileConfig is the on-disk JSON shape (same fields, optional).
type FileConfig struct {
	APIURL   string `json:"apiUrl"`
	Token    string `json:"token"`
	TenantID string `json:"tenantId"`
}

// FlagOverrides are values from CLI flags (empty means unset).
type FlagOverrides struct {
	APIURL   string
	Token    string
	TenantID string
}

// Load merges config file, environment, and flags.
// Precedence: flags > env > config file > empty.
func Load(flags FlagOverrides) (Config, error) {
	cfg := Config{}

	if fileCfg, err := loadFile(); err != nil {
		return Config{}, err
	} else if fileCfg != nil {
		cfg.APIURL = fileCfg.APIURL
		cfg.Token = fileCfg.Token
		cfg.TenantID = fileCfg.TenantID
	}

	if v := os.Getenv("KVANTUMCI_API_URL"); v != "" {
		cfg.APIURL = v
	}
	if v := os.Getenv("KVANTUMCI_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("KVANTUMCI_TENANT_ID"); v != "" {
		cfg.TenantID = v
	}

	if flags.APIURL != "" {
		cfg.APIURL = flags.APIURL
	}
	if flags.Token != "" {
		cfg.Token = flags.Token
	}
	if flags.TenantID != "" {
		cfg.TenantID = flags.TenantID
	}

	return cfg, nil
}

// RequireAuth ensures token and tenant are set for ClientApi calls.
func (c Config) RequireAuth() error {
	if c.Token == "" {
		return errors.New("token is required (--token or KVANTUMCI_TOKEN)")
	}
	if c.TenantID == "" {
		return errors.New("tenant-id is required (--tenant-id or KVANTUMCI_TENANT_ID)")
	}
	return nil
}

// RequireAPIURL ensures a base URL is configured.
func (c Config) RequireAPIURL() error {
	if c.APIURL == "" {
		return errors.New("api-url is required (--api-url or KVANTUMCI_API_URL)")
	}
	return nil
}

// LoadFile reads the on-disk config only (no env/flag merge).
// Returns an empty Config when the file does not exist.
func LoadFile() (Config, error) {
	fc, err := loadFile()
	if err != nil {
		return Config{}, err
	}
	if fc == nil {
		return Config{}, nil
	}
	return Config{APIURL: fc.APIURL, Token: fc.Token, TenantID: fc.TenantID}, nil
}

// Save writes cfg to the config file, creating the directory if needed.
// Existing unknown JSON keys are not preserved (file only stores known fields).
// File mode is 0600; directory mode is 0700.
func Save(cfg Config) (string, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}

	// Re-login: merge onto existing file so omitted fields stay put.
	existing, err := LoadFile()
	if err != nil {
		return "", err
	}
	merged := existing
	if cfg.APIURL != "" {
		merged.APIURL = cfg.APIURL
	}
	if cfg.Token != "" {
		merged.Token = cfg.Token
	}
	if cfg.TenantID != "" {
		merged.TenantID = cfg.TenantID
	}

	data, err := json.MarshalIndent(FileConfig{
		APIURL:   merged.APIURL,
		Token:    merged.Token,
		TenantID: merged.TenantID,
	}, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}
	return path, nil
}

func loadFile() (*FileConfig, error) {
	path, err := ConfigFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	var fc FileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}
	return &fc, nil
}

// ConfigFilePath returns the config file path.
// KVANTUMCI_CONFIG overrides the default platform location when set.
func ConfigFilePath() (string, error) {
	if p := os.Getenv("KVANTUMCI_CONFIG"); p != "" {
		return p, nil
	}
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func configDir() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "Library", "Application Support", "kvantumci"), nil
	case "windows":
		if appData := os.Getenv("AppData"); appData != "" {
			return filepath.Join(appData, "kvantumci"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, "AppData", "Roaming", "kvantumci"), nil
	default:
		if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
			return filepath.Join(xdg, "kvantumci"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".config", "kvantumci"), nil
	}
}
