package basecamp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.BaseURL != "https://3.basecampapi.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://3.basecampapi.com")
	}
	if cfg.Scope != "read" {
		t.Errorf("Scope = %q, want %q", cfg.Scope, "read")
	}
	if cfg.Format != "auto" {
		t.Errorf("Format = %q, want %q", cfg.Format, "auto")
	}
	if cfg.CacheEnabled != false {
		t.Errorf("CacheEnabled = %v, want false", cfg.CacheEnabled)
	}
	if cfg.Sources == nil {
		t.Error("Sources should not be nil")
	}
}

func TestLoadFromEnv(t *testing.T) {
	// Save and restore environment
	envVars := []string{
		"BASECAMP_BASE_URL",
		"BASECAMP_ACCOUNT_ID",
		"BASECAMP_PROJECT_ID",
		"BASECAMP_TODOLIST_ID",
		"BASECAMP_CACHE_DIR",
		"BASECAMP_CACHE_ENABLED",
		"BASECAMP_SCOPE",
		"BASECAMP_FORMAT",
	}
	saved := make(map[string]string)
	for _, k := range envVars {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	defer func() {
		for k, v := range saved {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	t.Run("BASECAMP_* variables", func(t *testing.T) {
		os.Setenv("BASECAMP_BASE_URL", "https://custom.api.com")
		os.Setenv("BASECAMP_ACCOUNT_ID", "12345")
		os.Setenv("BASECAMP_PROJECT_ID", "67890")
		os.Setenv("BASECAMP_TODOLIST_ID", "11111")
		os.Setenv("BASECAMP_CACHE_DIR", "/tmp/cache")
		os.Setenv("BASECAMP_CACHE_ENABLED", "true")
		defer func() {
			for _, k := range envVars {
				os.Unsetenv(k)
			}
		}()

		cfg := DefaultConfig()
		cfg.LoadConfigFromEnv()

		if cfg.BaseURL != "https://custom.api.com" {
			t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://custom.api.com")
		}
		if cfg.AccountID != "12345" {
			t.Errorf("AccountID = %q, want %q", cfg.AccountID, "12345")
		}
		if cfg.ProjectID != "67890" {
			t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "67890")
		}
		if cfg.TodolistID != "11111" {
			t.Errorf("TodolistID = %q, want %q", cfg.TodolistID, "11111")
		}
		if cfg.CacheDir != "/tmp/cache" {
			t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, "/tmp/cache")
		}
		if cfg.CacheEnabled != true {
			t.Errorf("CacheEnabled = %v, want true", cfg.CacheEnabled)
		}
		if cfg.Sources["base_url"] != SourceEnv {
			t.Errorf("Sources[base_url] = %q, want %q", cfg.Sources["base_url"], SourceEnv)
		}
	})

	t.Run("cache enabled values", func(t *testing.T) {
		tests := []struct {
			value string
			want  bool
		}{
			{"true", true},
			{"True", true},
			{"TRUE", true},
			{"1", true},
			{"false", false},
			{"0", false},
			{"", false},
		}

		for _, tt := range tests {
			os.Unsetenv("BASECAMP_CACHE_ENABLED")

			if tt.value != "" {
				os.Setenv("BASECAMP_CACHE_ENABLED", tt.value)
			}

			cfg := DefaultConfig()
			cfg.LoadConfigFromEnv()

			if cfg.CacheEnabled != tt.want {
				t.Errorf("BASECAMP_CACHE_ENABLED=%q: CacheEnabled = %v, want %v", tt.value, cfg.CacheEnabled, tt.want)
			}
		}
	})
}

func TestLoadConfigFromFile(t *testing.T) {
	t.Run("loads valid JSON file", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.json")
		// Note: LoadConfig uses json.Unmarshal directly which requires string fields.
		// For numeric IDs, use the layered Load() function which handles number->string.
		content := `{
			"base_url": "https://test.api.com",
			"account_id": "12345",
			"project_id": "67890",
			"cache_enabled": true
		}`
		if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		cfg, err := LoadConfig(cfgPath)
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if cfg.BaseURL != "https://test.api.com" {
			t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://test.api.com")
		}
		if cfg.AccountID != "12345" {
			t.Errorf("AccountID = %q, want %q", cfg.AccountID, "12345")
		}
		if cfg.ProjectID != "67890" {
			t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "67890")
		}
		if cfg.CacheEnabled != true {
			t.Errorf("CacheEnabled = %v, want true", cfg.CacheEnabled)
		}
	})

	t.Run("returns defaults for non-existent file", func(t *testing.T) {
		cfg, err := LoadConfig("/nonexistent/path/config.json")
		if err != nil {
			t.Fatalf("LoadConfig failed: %v", err)
		}

		if cfg.BaseURL != "https://3.basecampapi.com" {
			t.Errorf("BaseURL = %q, want default", cfg.BaseURL)
		}
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfgPath := filepath.Join(tmpDir, "config.json")
		if err := os.WriteFile(cfgPath, []byte("not valid json"), 0644); err != nil {
			t.Fatalf("failed to write test config: %v", err)
		}

		_, err := LoadConfig(cfgPath)
		if err == nil {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestLoadWithOverrides(t *testing.T) {
	// Clear environment variables that might interfere
	envVars := []string{
		"BASECAMP_BASE_URL",
		"BASECAMP_ACCOUNT_ID",
		"BASECAMP_PROJECT_ID",
		"BASECAMP_TODOLIST_ID",
		"BASECAMP_CACHE_DIR",
	}
	for _, k := range envVars {
		os.Unsetenv(k)
	}

	cfg, err := Load(LoadOptions{
		Account:  "99999",
		Project:  "88888",
		Todolist: "77777",
		BaseURL:  "https://override.api.com",
		CacheDir: "/custom/cache",
		Format:   "json",
	})
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.AccountID != "99999" {
		t.Errorf("AccountID = %q, want %q", cfg.AccountID, "99999")
	}
	if cfg.ProjectID != "88888" {
		t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "88888")
	}
	if cfg.TodolistID != "77777" {
		t.Errorf("TodolistID = %q, want %q", cfg.TodolistID, "77777")
	}
	if cfg.BaseURL != "https://override.api.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://override.api.com")
	}
	if cfg.CacheDir != "/custom/cache" {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, "/custom/cache")
	}
	if cfg.Format != "json" {
		t.Errorf("Format = %q, want %q", cfg.Format, "json")
	}

	// Check sources
	if cfg.Sources["account_id"] != SourceFlag {
		t.Errorf("Sources[account_id] = %q, want %q", cfg.Sources["account_id"], SourceFlag)
	}
	if cfg.Sources["base_url"] != SourceFlag {
		t.Errorf("Sources[base_url] = %q, want %q", cfg.Sources["base_url"], SourceFlag)
	}
}

func TestLoadLayeredPrecedence(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create global config directory
	globalDir := filepath.Join(tmpDir, ".config", "basecamp")
	if err := os.MkdirAll(globalDir, 0755); err != nil {
		t.Fatalf("failed to create global config dir: %v", err)
	}

	// Create global config
	globalCfg := `{"account_id": "global-account", "project_id": "global-project"}`
	if err := os.WriteFile(filepath.Join(globalDir, "config.json"), []byte(globalCfg), 0644); err != nil {
		t.Fatalf("failed to write global config: %v", err)
	}

	// Save and set XDG_CONFIG_HOME
	origXDG := os.Getenv("XDG_CONFIG_HOME")
	os.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, ".config"))
	defer os.Setenv("XDG_CONFIG_HOME", origXDG)

	// Clear environment variables
	os.Unsetenv("BASECAMP_ACCOUNT_ID")
	os.Unsetenv("BASECAMP_PROJECT_ID")

	t.Run("global config loaded", func(t *testing.T) {
		cfg, err := Load(LoadOptions{})
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.AccountID != "global-account" {
			t.Errorf("AccountID = %q, want %q", cfg.AccountID, "global-account")
		}
		if cfg.Sources["account_id"] != SourceGlobal {
			t.Errorf("Sources[account_id] = %q, want %q", cfg.Sources["account_id"], SourceGlobal)
		}
	})

	t.Run("env overrides global", func(t *testing.T) {
		os.Setenv("BASECAMP_ACCOUNT_ID", "env-account")
		defer os.Unsetenv("BASECAMP_ACCOUNT_ID")

		cfg, err := Load(LoadOptions{})
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.AccountID != "env-account" {
			t.Errorf("AccountID = %q, want %q", cfg.AccountID, "env-account")
		}
		if cfg.Sources["account_id"] != SourceEnv {
			t.Errorf("Sources[account_id] = %q, want %q", cfg.Sources["account_id"], SourceEnv)
		}
		// Project should still come from global
		if cfg.ProjectID != "global-project" {
			t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "global-project")
		}
	})

	t.Run("flag overrides env", func(t *testing.T) {
		os.Setenv("BASECAMP_ACCOUNT_ID", "env-account")
		defer os.Unsetenv("BASECAMP_ACCOUNT_ID")

		cfg, err := Load(LoadOptions{Account: "flag-account"})
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if cfg.AccountID != "flag-account" {
			t.Errorf("AccountID = %q, want %q", cfg.AccountID, "flag-account")
		}
		if cfg.Sources["account_id"] != SourceFlag {
			t.Errorf("Sources[account_id] = %q, want %q", cfg.Sources["account_id"], SourceFlag)
		}
	})
}

func TestGetStringOrNumber(t *testing.T) {
	tests := []struct {
		name  string
		input map[string]any
		key   string
		want  string
	}{
		{
			name:  "string value",
			input: map[string]any{"id": "12345"},
			key:   "id",
			want:  "12345",
		},
		{
			name:  "float64 value (JSON number)",
			input: map[string]any{"id": float64(12345)},
			key:   "id",
			want:  "12345",
		},
		{
			name:  "int value",
			input: map[string]any{"id": 12345},
			key:   "id",
			want:  "12345",
		},
		{
			name:  "missing key",
			input: map[string]any{"other": "value"},
			key:   "id",
			want:  "",
		},
		{
			name:  "nil value",
			input: map[string]any{"id": nil},
			key:   "id",
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getStringOrNumber(tt.input, tt.key)
			if got != tt.want {
				t.Errorf("getStringOrNumber() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHostConfig(t *testing.T) {
	t.Run("GetHost returns correct host", func(t *testing.T) {
		cfg := &Config{
			Hosts: map[string]*HostConfig{
				"production": {BaseURL: "https://prod.api.com", ClientID: "prod-client"},
				"staging":    {BaseURL: "https://staging.api.com", ClientID: "staging-client"},
			},
			DefaultHost: "production",
		}

		host := cfg.GetHost("staging")
		if host == nil {
			t.Fatal("GetHost returned nil")
		}
		if host.BaseURL != "https://staging.api.com" {
			t.Errorf("BaseURL = %q, want %q", host.BaseURL, "https://staging.api.com")
		}
	})

	t.Run("GetHost returns default when name is empty", func(t *testing.T) {
		cfg := &Config{
			Hosts: map[string]*HostConfig{
				"production": {BaseURL: "https://prod.api.com"},
				"staging":    {BaseURL: "https://staging.api.com"},
			},
			DefaultHost: "production",
		}

		host := cfg.GetHost("")
		if host == nil {
			t.Fatal("GetHost returned nil")
		}
		if host.BaseURL != "https://prod.api.com" {
			t.Errorf("BaseURL = %q, want %q", host.BaseURL, "https://prod.api.com")
		}
	})

	t.Run("GetHost returns nil for unknown host", func(t *testing.T) {
		cfg := &Config{
			Hosts: map[string]*HostConfig{
				"production": {BaseURL: "https://prod.api.com"},
			},
		}

		host := cfg.GetHost("unknown")
		if host != nil {
			t.Errorf("GetHost returned %v, want nil", host)
		}
	})

	t.Run("GetHost returns nil when no hosts", func(t *testing.T) {
		cfg := &Config{}

		host := cfg.GetHost("any")
		if host != nil {
			t.Errorf("GetHost returned %v, want nil", host)
		}
	})
}

func TestGetSource(t *testing.T) {
	t.Run("returns source for tracked key", func(t *testing.T) {
		cfg := &Config{
			Sources: map[string]Source{
				"base_url":   SourceEnv,
				"account_id": SourceFlag,
			},
		}

		if src := cfg.GetSource("base_url"); src != SourceEnv {
			t.Errorf("GetSource(base_url) = %q, want %q", src, SourceEnv)
		}
		if src := cfg.GetSource("account_id"); src != SourceFlag {
			t.Errorf("GetSource(account_id) = %q, want %q", src, SourceFlag)
		}
	})

	t.Run("returns default for untracked key", func(t *testing.T) {
		cfg := &Config{
			Sources: map[string]Source{
				"base_url": SourceEnv,
			},
		}

		if src := cfg.GetSource("project_id"); src != SourceDefault {
			t.Errorf("GetSource(project_id) = %q, want %q", src, SourceDefault)
		}
	})

	t.Run("returns default when Sources is nil", func(t *testing.T) {
		cfg := &Config{}

		if src := cfg.GetSource("base_url"); src != SourceDefault {
			t.Errorf("GetSource(base_url) = %q, want %q", src, SourceDefault)
		}
	})
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://api.com/", "https://api.com"},
		{"https://api.com", "https://api.com"},
		{"https://api.com///", "https://api.com"},
	}

	for _, tt := range tests {
		got := NormalizeBaseURL(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGlobalConfigDir(t *testing.T) {
	t.Run("uses XDG_CONFIG_HOME when set", func(t *testing.T) {
		orig := os.Getenv("XDG_CONFIG_HOME")
		os.Setenv("XDG_CONFIG_HOME", "/custom/config")
		defer os.Setenv("XDG_CONFIG_HOME", orig)

		got := GlobalConfigDir()
		want := "/custom/config/basecamp"
		if got != want {
			t.Errorf("GlobalConfigDir() = %q, want %q", got, want)
		}
	})

	t.Run("uses ~/.config when XDG_CONFIG_HOME not set", func(t *testing.T) {
		orig := os.Getenv("XDG_CONFIG_HOME")
		os.Unsetenv("XDG_CONFIG_HOME")
		defer os.Setenv("XDG_CONFIG_HOME", orig)

		home, _ := os.UserHomeDir()
		want := filepath.Join(home, ".config", "basecamp")

		got := GlobalConfigDir()
		if got != want {
			t.Errorf("GlobalConfigDir() = %q, want %q", got, want)
		}
	})
}

func TestLoadFromFileWithHosts(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	content := `{
		"base_url": "https://default.api.com",
		"default_host": "production",
		"hosts": {
			"production": {
				"base_url": "https://prod.api.com",
				"client_id": "prod-client"
			},
			"staging": {
				"base_url": "https://staging.api.com",
				"client_id": "staging-client"
			},
			"invalid": {
				"client_id": "no-base-url"
			}
		}
	}`
	if err := os.WriteFile(cfgPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg := defaultConfig()
	loadFromFile(cfg, cfgPath, SourceGlobal)

	if cfg.DefaultHost != "production" {
		t.Errorf("DefaultHost = %q, want %q", cfg.DefaultHost, "production")
	}

	if len(cfg.Hosts) != 2 {
		t.Errorf("len(Hosts) = %d, want 2 (invalid should be skipped)", len(cfg.Hosts))
	}

	prod := cfg.Hosts["production"]
	if prod == nil {
		t.Fatal("Hosts[production] is nil")
	}
	if prod.BaseURL != "https://prod.api.com" {
		t.Errorf("production.BaseURL = %q, want %q", prod.BaseURL, "https://prod.api.com")
	}
	if prod.ClientID != "prod-client" {
		t.Errorf("production.ClientID = %q, want %q", prod.ClientID, "prod-client")
	}

	staging := cfg.Hosts["staging"]
	if staging == nil {
		t.Fatal("Hosts[staging] is nil")
	}
	if staging.BaseURL != "https://staging.api.com" {
		t.Errorf("staging.BaseURL = %q, want %q", staging.BaseURL, "https://staging.api.com")
	}

	// Invalid host (no base_url) should be skipped
	if cfg.Hosts["invalid"] != nil {
		t.Error("Hosts[invalid] should be nil (skipped due to missing base_url)")
	}

	if cfg.Sources["hosts"] != SourceGlobal {
		t.Errorf("Sources[hosts] = %q, want %q", cfg.Sources["hosts"], SourceGlobal)
	}
}
