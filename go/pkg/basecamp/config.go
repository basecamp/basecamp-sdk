package basecamp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config holds the resolved configuration for API access.
type Config struct {
	// BaseURL is the API base URL (e.g., "https://3.basecampapi.com").
	BaseURL string `json:"base_url"`

	// AccountID is the default account ID.
	AccountID string `json:"account_id"`

	// ProjectID is the default project/bucket ID.
	ProjectID string `json:"project_id"`

	// TodolistID is the default todolist ID.
	TodolistID string `json:"todolist_id"`

	// CacheDir is the directory for HTTP cache storage.
	CacheDir string `json:"cache_dir"`

	// CacheEnabled controls whether HTTP caching is enabled.
	CacheEnabled bool `json:"cache_enabled"`

	// Format is the default output format.
	Format string `json:"format"`

	// Scope is the OAuth scope.
	Scope string `json:"scope"`

	// Host settings (multiple environments)
	Hosts       map[string]*HostConfig `json:"hosts,omitempty"`
	DefaultHost string                 `json:"default_host,omitempty"`

	// Sources tracks where each value came from (for debugging).
	Sources map[string]Source `json:"-"`
}

// HostConfig holds configuration for a specific host/environment.
type HostConfig struct {
	BaseURL  string `json:"base_url"`
	ClientID string `json:"client_id,omitempty"`
}

// Source indicates where a config value came from.
type Source string

const (
	SourceDefault Source = "default"
	SourceSystem  Source = "system"
	SourceGlobal  Source = "global"
	SourceRepo    Source = "repo"
	SourceLocal   Source = "local"
	SourceEnv     Source = "env"
	SourceFlag    Source = "flag"
)

// LoadOptions holds options for loading configuration.
type LoadOptions struct {
	// Account overrides the account_id from any source.
	Account string
	// Project overrides the project_id from any source.
	Project string
	// Todolist overrides the todolist_id from any source.
	Todolist string
	// BaseURL overrides the base_url from any source.
	BaseURL string
	// CacheDir overrides the cache_dir from any source.
	CacheDir string
	// Format overrides the format from any source.
	Format string
}

// DefaultConfig returns a Config with sensible defaults.
// This is maintained for backwards compatibility.
func DefaultConfig() *Config {
	return defaultConfig()
}

// defaultConfig creates a new config with default values.
func defaultConfig() *Config {
	cacheDir := os.Getenv("XDG_CACHE_HOME")
	if cacheDir == "" {
		home, _ := os.UserHomeDir()
		cacheDir = filepath.Join(home, ".cache")
	}

	return &Config{
		BaseURL:      "https://3.basecampapi.com",
		Scope:        "read",
		CacheDir:     filepath.Join(cacheDir, "basecamp"),
		CacheEnabled: false,
		Format:       "auto",
		Sources:      make(map[string]Source),
	}
}

// LoadConfig loads configuration from a JSON file.
// This is maintained for backwards compatibility.
func LoadConfig(path string) (*Config, error) {
	cfg := defaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// LoadConfigFromEnv loads configuration from environment variables.
// Environment variables override any values already set in the config.
// This is maintained for backwards compatibility.
func (c *Config) LoadConfigFromEnv() {
	loadFromEnv(c)
}

// Load loads configuration from all sources with proper precedence.
// Precedence: flags > env > local > repo > global > system > defaults
//
// File locations:
//   - System: /etc/basecamp/config.json
//   - Global: ~/.config/basecamp/config.json (XDG-compliant)
//   - Repo: .basecamp/config.json at git root
//   - Local: .basecamp/config.json in current and parent directories
func Load(opts LoadOptions) (*Config, error) {
	cfg := defaultConfig()

	// Load from file layers (system -> global -> repo -> local)
	loadFromFile(cfg, systemConfigPath(), SourceSystem)
	loadFromFile(cfg, GlobalConfigPath(), SourceGlobal)

	repoPath := repoConfigPath()
	if repoPath != "" {
		loadFromFile(cfg, repoPath, SourceRepo)
	}

	// Load all local configs from root to current (closer overrides)
	// This allows nested directories to override parent directories
	localPaths := localConfigPaths(repoPath)
	for _, path := range localPaths {
		loadFromFile(cfg, path, SourceLocal)
	}

	// Load from environment
	loadFromEnv(cfg)

	// Apply flag overrides
	applyOverrides(cfg, opts)

	return cfg, nil
}

// loadFromFile loads configuration from a JSON file into cfg.
func loadFromFile(cfg *Config, path string, source Source) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: Path is from trusted config locations
	if err != nil {
		return // File doesn't exist, skip
	}

	// Use json.Decoder with UseNumber to preserve precision for large numeric IDs
	var fileCfg map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&fileCfg); err != nil {
		return // Invalid JSON, skip
	}

	if v, ok := fileCfg["base_url"].(string); ok && v != "" {
		cfg.BaseURL = v
		cfg.Sources["base_url"] = source
	}
	if v := getStringOrNumber(fileCfg, "account_id"); v != "" {
		cfg.AccountID = v
		cfg.Sources["account_id"] = source
	}
	if v := getStringOrNumber(fileCfg, "project_id"); v != "" {
		cfg.ProjectID = v
		cfg.Sources["project_id"] = source
	}
	if v := getStringOrNumber(fileCfg, "todolist_id"); v != "" {
		cfg.TodolistID = v
		cfg.Sources["todolist_id"] = source
	}
	if v, ok := fileCfg["scope"].(string); ok && v != "" {
		cfg.Scope = v
		cfg.Sources["scope"] = source
	}
	if v, ok := fileCfg["cache_dir"].(string); ok && v != "" {
		cfg.CacheDir = v
		cfg.Sources["cache_dir"] = source
	}
	if v, ok := fileCfg["cache_enabled"].(bool); ok {
		cfg.CacheEnabled = v
		cfg.Sources["cache_enabled"] = source
	}
	if v, ok := fileCfg["format"].(string); ok && v != "" {
		cfg.Format = v
		cfg.Sources["format"] = source
	}
	if v, ok := fileCfg["default_host"].(string); ok && v != "" {
		cfg.DefaultHost = v
		cfg.Sources["default_host"] = source
	}
	if v, ok := fileCfg["hosts"].(map[string]any); ok {
		if cfg.Hosts == nil {
			cfg.Hosts = make(map[string]*HostConfig)
		}
		for name, hostData := range v {
			if hostMap, ok := hostData.(map[string]any); ok {
				hostConfig := &HostConfig{}
				if baseURL, ok := hostMap["base_url"].(string); ok && baseURL != "" {
					hostConfig.BaseURL = baseURL
				} else {
					// Skip hosts with empty or missing base_url
					continue
				}
				if clientID, ok := hostMap["client_id"].(string); ok {
					hostConfig.ClientID = clientID
				}
				cfg.Hosts[name] = hostConfig
			}
		}
		cfg.Sources["hosts"] = source
	}
}

// loadFromEnv loads configuration from environment variables.
func loadFromEnv(cfg *Config) {
	if cfg.Sources == nil {
		cfg.Sources = make(map[string]Source)
	}

	if v := os.Getenv("BASECAMP_BASE_URL"); v != "" {
		cfg.BaseURL = v
		cfg.Sources["base_url"] = SourceEnv
	}
	if v := os.Getenv("BASECAMP_ACCOUNT_ID"); v != "" {
		cfg.AccountID = v
		cfg.Sources["account_id"] = SourceEnv
	}
	if v := os.Getenv("BASECAMP_PROJECT_ID"); v != "" {
		cfg.ProjectID = v
		cfg.Sources["project_id"] = SourceEnv
	}
	if v := os.Getenv("BASECAMP_TODOLIST_ID"); v != "" {
		cfg.TodolistID = v
		cfg.Sources["todolist_id"] = SourceEnv
	}
	if v := os.Getenv("BASECAMP_CACHE_DIR"); v != "" {
		cfg.CacheDir = v
		cfg.Sources["cache_dir"] = SourceEnv
	}
	if v := os.Getenv("BASECAMP_CACHE_ENABLED"); v != "" {
		cfg.CacheEnabled = strings.ToLower(v) == "true" || v == "1"
		cfg.Sources["cache_enabled"] = SourceEnv
	}
	if v := os.Getenv("BASECAMP_SCOPE"); v != "" {
		cfg.Scope = v
		cfg.Sources["scope"] = SourceEnv
	}
	if v := os.Getenv("BASECAMP_FORMAT"); v != "" {
		cfg.Format = v
		cfg.Sources["format"] = SourceEnv
	}
}

// getStringOrNumber extracts a value that may be either a string or number in JSON.
// Uses json.Number to preserve precision for large numeric IDs.
func getStringOrNumber(m map[string]any, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		// json.Number preserves full precision for large integers
		return val.String()
	case float64:
		// Fallback for standard JSON unmarshaling (loses precision for large IDs)
		// If the value is an integer, format without decimals
		if val == float64(int64(val)) {
			return fmt.Sprintf("%.0f", val)
		}
		return fmt.Sprintf("%g", val)
	case int:
		return fmt.Sprintf("%d", val)
	case int64:
		return fmt.Sprintf("%d", val)
	default:
		return ""
	}
}

// applyOverrides applies command-line flag overrides to the config.
func applyOverrides(cfg *Config, o LoadOptions) {
	if o.Account != "" {
		cfg.AccountID = o.Account
		cfg.Sources["account_id"] = SourceFlag
	}
	if o.Project != "" {
		cfg.ProjectID = o.Project
		cfg.Sources["project_id"] = SourceFlag
	}
	if o.Todolist != "" {
		cfg.TodolistID = o.Todolist
		cfg.Sources["todolist_id"] = SourceFlag
	}
	if o.BaseURL != "" {
		cfg.BaseURL = o.BaseURL
		cfg.Sources["base_url"] = SourceFlag
	}
	if o.CacheDir != "" {
		cfg.CacheDir = o.CacheDir
		cfg.Sources["cache_dir"] = SourceFlag
	}
	if o.Format != "" {
		cfg.Format = o.Format
		cfg.Sources["format"] = SourceFlag
	}
}

// Path helpers

func systemConfigPath() string {
	return "/etc/basecamp/config.json"
}

// GlobalConfigPath returns the path to the global config file.
func GlobalConfigPath() string {
	return filepath.Join(GlobalConfigDir(), "config.json")
}

func repoConfigPath() string {
	// Walk up to find .git directory, then look for .basecamp/config.json
	dir, _ := os.Getwd()
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			cfgPath := filepath.Join(dir, ".basecamp", "config.json")
			if _, err := os.Stat(cfgPath); err == nil {
				return cfgPath
			}
			return ""
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// localConfigPaths returns all .basecamp/config.json paths from root to current directory,
// excluding the repo config path (already loaded as SourceRepo).
// Paths are returned in order from furthest ancestor to closest, so closer configs override.
func localConfigPaths(repoConfigPath string) []string {
	dir, _ := os.Getwd()
	var paths []string

	// Collect all paths walking up
	for {
		cfgPath := filepath.Join(dir, ".basecamp", "config.json")
		if _, err := os.Stat(cfgPath); err == nil {
			// Skip if this is the repo config (already loaded)
			if cfgPath != repoConfigPath {
				paths = append(paths, cfgPath)
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	// Reverse so paths go from root to current (closer overrides)
	for i, j := 0, len(paths)-1; i < j; i, j = i+1, j-1 {
		paths[i], paths[j] = paths[j], paths[i]
	}

	return paths
}

// GlobalConfigDir returns the global config directory path.
func GlobalConfigDir() string {
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		home, _ := os.UserHomeDir()
		configDir = filepath.Join(home, ".config")
	}
	return filepath.Join(configDir, "basecamp")
}

// NormalizeBaseURL ensures consistent URL format (no trailing slashes).
func NormalizeBaseURL(url string) string {
	return strings.TrimRight(url, "/")
}

// GetSource returns the source of a configuration value.
func (c *Config) GetSource(key string) Source {
	if c.Sources == nil {
		return SourceDefault
	}
	if src, ok := c.Sources[key]; ok {
		return src
	}
	return SourceDefault
}

// GetHost returns the host configuration for the given name.
// If name is empty, it returns the default host configuration.
// Returns nil if no matching host is found.
func (c *Config) GetHost(name string) *HostConfig {
	if name == "" {
		name = c.DefaultHost
	}
	if name == "" || c.Hosts == nil {
		return nil
	}
	return c.Hosts[name]
}
