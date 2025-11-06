package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "goosegorm.yml")

	content := `database_url: postgres://user:pass@localhost:5432/myapp?sslmode=disable
models_dir: ./models
migrations_dir: ./migrations
package_name: migrations
migration_table: _goosegorm_migrations
ignore_models: []
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg.DatabaseURL != "postgres://user:pass@localhost:5432/myapp?sslmode=disable" {
		t.Errorf("Expected database_url, got '%s'", cfg.DatabaseURL)
	}
	if cfg.ModelsDir == "" {
		t.Error("ModelsDir should be set")
	}
	if cfg.MigrationsDir == "" {
		t.Error("MigrationsDir should be set")
	}
	if cfg.PackageName != "migrations" {
		t.Errorf("Expected package_name 'migrations', got '%s'", cfg.PackageName)
	}
	if cfg.MigrationTable != "_goosegorm_migrations" {
		t.Errorf("Expected migration_table '_goosegorm_migrations', got '%s'", cfg.MigrationTable)
	}
}

func TestLoadConfigWithDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "goosegorm.yml")

	content := `database_url: postgres://localhost/test
models_dir: ./models
migrations_dir: ./migrations
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Check defaults
	if cfg.PackageName == "" {
		t.Error("PackageName should have default value")
	}
	if cfg.MigrationTable == "" {
		t.Error("MigrationTable should have default value")
	}
}

func TestValidateConfig(t *testing.T) {
	cfg := &Config{
		DatabaseURL:   "postgres://localhost/test",
		ModelsDir:     "./models",
		MigrationsDir: "./migrations",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Valid config should not error: %v", err)
	}

	// Test missing database_url
	cfg.DatabaseURL = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Config with missing database_url should error")
	}

	// Test missing models_dir
	cfg.DatabaseURL = "postgres://localhost/test"
	cfg.ModelsDir = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Config with missing models_dir should error")
	}

	// Test missing migrations_dir
	cfg.ModelsDir = "./models"
	cfg.MigrationsDir = ""
	if err := cfg.Validate(); err == nil {
		t.Error("Config with missing migrations_dir should error")
	}
}

func TestLoadConfigWithRelativePaths(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "goosegorm.yml")

	content := `database_url: postgres://localhost/test
models_dir: ./models
migrations_dir: ./migrations
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	// Paths should be resolved relative to config file location
	if !filepath.IsAbs(cfg.ModelsDir) {
		// In test, relative paths are resolved to absolute paths
		expected := filepath.Join(tmpDir, "models")
		if cfg.ModelsDir != expected {
			t.Errorf("Expected ModelsDir '%s', got '%s'", expected, cfg.ModelsDir)
		}
	}
}
