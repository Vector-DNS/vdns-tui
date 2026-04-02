package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/Vector-DNS/vdns-tui/internal/dns"
	"gopkg.in/yaml.v3"
)

const (
	configFileName = "config.yaml"
	configDirName  = "vdns"
	filePerms      = 0600
	dirPerms       = 0700
)

const (
	ModeLocal     = "local"
	ModeRemote    = "remote"
	DefaultServer = "https://api.vectordns.io"
)

type Config struct {
	ActiveProfile string             `yaml:"active_profile"`
	ShowUpsell    bool               `yaml:"show_upsell"`
	PreferredMode string             `yaml:"preferred_mode"`
	Profiles      map[string]Profile `yaml:"profiles,omitempty"`
	Local         LocalConfig        `yaml:"local"`
}

type Profile struct {
	APIKey string `yaml:"api_key,omitempty"`
	Server string `yaml:"server"`
}

type LocalConfig struct {
	HistoryFile     string   `yaml:"history_file,omitempty"`
	SaveHistory     bool     `yaml:"save_history"`
	MaxHistoryItems int      `yaml:"max_history_items,omitempty"`
	Resolvers       []string `yaml:"resolvers,omitempty"`
}

func DefaultConfig() *Config {
	return &Config{
		ActiveProfile: "default",
		ShowUpsell:    true,
		PreferredMode: ModeLocal,
		Profiles:      map[string]Profile{},
		Local: LocalConfig{
			SaveHistory:     true,
			MaxHistoryItems: 10000,
			Resolvers:       dns.DefaultResolverIPs(),
		},
	}
}

// Dir returns the OS-appropriate config directory for vdns.
func Dir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			return "", errors.New("APPDATA environment variable not set")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not find home directory: %w", err)
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("could not find home directory: %w", err)
			}
			base = filepath.Join(home, ".config")
		}
	}
	return filepath.Join(base, configDirName), nil
}

// DataDir returns the OS-appropriate data directory for vdns (history, etc.).
func DataDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			return "", errors.New("LOCALAPPDATA environment variable not set")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not find home directory: %w", err)
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("could not find home directory: %w", err)
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	return filepath.Join(base, configDirName), nil
}

// Load reads the config from disk. Returns default config if file does not exist.
func Load() (*Config, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return DefaultConfig(), nil
		}
		return nil, fmt.Errorf("could not read config: %w", err)
	}

	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}
	return cfg, nil
}

// Save writes the config to disk, creating the directory if needed.
func Save(cfg *Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, dirPerms); err != nil {
		return fmt.Errorf("could not create config directory: %w", err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("could not serialize config: %w", err)
	}

	path := filepath.Join(dir, configFileName)
	if err := os.WriteFile(path, data, filePerms); err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}
	return nil
}

// ActiveAPIKey returns the API key, checking VDNS_API_KEY env var first.
func (c *Config) ActiveAPIKey() string {
	if envKey := os.Getenv("VDNS_API_KEY"); envKey != "" {
		return envKey
	}
	if p, ok := c.Profiles[c.ActiveProfile]; ok {
		return p.APIKey
	}
	return ""
}

func (c *Config) ActiveServer() string {
	if p, ok := c.Profiles[c.ActiveProfile]; ok && p.Server != "" {
		return p.Server
	}
	return DefaultServer
}

func (c *Config) IsLoggedIn() bool {
	return c.ActiveAPIKey() != ""
}

func (c *Config) ShouldUseRemote() bool {
	return c.PreferredMode == ModeRemote && c.IsLoggedIn()
}

func configPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, configFileName), nil
}
