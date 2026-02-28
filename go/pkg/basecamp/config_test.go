package basecamp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.BaseURL != "https://3.basecampapi.com" {
		t.Errorf("BaseURL = %q, want default", cfg.BaseURL)
	}
	if cfg.CacheEnabled {
		t.Error("CacheEnabled should default to false")
	}
}

func TestLoadConfig_FileNotExist(t *testing.T) {
	cfg, err := LoadConfig("/nonexistent/config.json")
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.BaseURL != "https://3.basecampapi.com" {
		t.Errorf("BaseURL = %q, want default", cfg.BaseURL)
	}
}

func TestLoadConfig_ValidFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data := `{"base_url":"https://custom.example.com","project_id":"123","cache_enabled":true}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.BaseURL != "https://custom.example.com" {
		t.Errorf("BaseURL = %q, want custom", cfg.BaseURL)
	}
	if cfg.ProjectID != "123" {
		t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "123")
	}
	if !cfg.CacheEnabled {
		t.Error("CacheEnabled should be true")
	}
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("NOT JSON"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadConfig(path)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestConfig_LoadConfigFromEnv(t *testing.T) {
	t.Setenv("BASECAMP_BASE_URL", "https://env.example.com")
	t.Setenv("BASECAMP_PROJECT_ID", "proj-99")
	t.Setenv("BASECAMP_TODOLIST_ID", "todo-42")
	t.Setenv("BASECAMP_CACHE_DIR", "/tmp/test-cache")
	t.Setenv("BASECAMP_CACHE_ENABLED", "true")

	cfg := DefaultConfig()
	cfg.LoadConfigFromEnv()

	if cfg.BaseURL != "https://env.example.com" {
		t.Errorf("BaseURL = %q, want env value", cfg.BaseURL)
	}
	if cfg.ProjectID != "proj-99" {
		t.Errorf("ProjectID = %q, want %q", cfg.ProjectID, "proj-99")
	}
	if cfg.TodolistID != "todo-42" {
		t.Errorf("TodolistID = %q, want %q", cfg.TodolistID, "todo-42")
	}
	if cfg.CacheDir != "/tmp/test-cache" {
		t.Errorf("CacheDir = %q, want env value", cfg.CacheDir)
	}
	if !cfg.CacheEnabled {
		t.Error("CacheEnabled should be true from env")
	}
}

func TestConfig_LoadConfigFromEnv_CacheEnabled_Values(t *testing.T) {
	tests := []struct {
		env  string
		want bool
	}{
		{"true", true},
		{"TRUE", true},
		{"1", true},
		{"false", false},
		{"0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.env, func(t *testing.T) {
			cfg := DefaultConfig()
			if tt.env != "" {
				t.Setenv("BASECAMP_CACHE_ENABLED", tt.env)
			}
			cfg.LoadConfigFromEnv()
			if cfg.CacheEnabled != tt.want {
				t.Errorf("BASECAMP_CACHE_ENABLED=%q: CacheEnabled = %v, want %v", tt.env, cfg.CacheEnabled, tt.want)
			}
		})
	}
}

func TestNormalizeBaseURL(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"https://3.basecampapi.com/", "https://3.basecampapi.com"},
		{"https://3.basecampapi.com", "https://3.basecampapi.com"},
	}

	for _, tt := range tests {
		got := NormalizeBaseURL(tt.input)
		if got != tt.want {
			t.Errorf("NormalizeBaseURL(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
