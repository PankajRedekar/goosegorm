package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DatabaseURL    string   `yaml:"database_url"`
	ModelsDir      string   `yaml:"models_dir"`
	MigrationsDir  string   `yaml:"migrations_dir"`
	PackageName    string   `yaml:"package_name"`
	MigrationTable string   `yaml:"migration_table"`
	IgnoreModels   []string `yaml:"ignore_models"`
	BuildPath      string   `yaml:"build_path"` // Optional: Path to save migrator binary for production use
}

func LoadConfig(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Set defaults
	if cfg.MigrationTable == "" {
		cfg.MigrationTable = "_goosegorm_migrations"
	}
	if cfg.PackageName == "" {
		cfg.PackageName = "migrations"
	}
	// MainPkgPath is deprecated and no longer used - kept for backward compatibility

	// Resolve relative paths
	if !filepath.IsAbs(cfg.ModelsDir) {
		cfg.ModelsDir = filepath.Join(filepath.Dir(configPath), cfg.ModelsDir)
	}
	if !filepath.IsAbs(cfg.MigrationsDir) {
		cfg.MigrationsDir = filepath.Join(filepath.Dir(configPath), cfg.MigrationsDir)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.DatabaseURL == "" {
		return fmt.Errorf("database_url is required")
	}
	if c.ModelsDir == "" {
		return fmt.Errorf("models_dir is required")
	}
	if c.MigrationsDir == "" {
		return fmt.Errorf("migrations_dir is required")
	}
	return nil
}
