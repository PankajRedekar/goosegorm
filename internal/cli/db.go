package cli

import (
	"fmt"
	"strings"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/loader"
	"github.com/pankajredekar/goosegorm/internal/runner"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// connectDB connects to the database based on the URL
func connectDB(databaseURL string) (*gorm.DB, error) {
	if strings.Contains(databaseURL, "postgres://") || strings.Contains(databaseURL, "postgresql://") {
		return gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	} else if strings.Contains(databaseURL, "sqlite://") {
		path := strings.TrimPrefix(databaseURL, "sqlite://")
		return gorm.Open(sqlite.Open(path), &gorm.Config{})
	}
	return nil, fmt.Errorf("unsupported database URL: %s", databaseURL)
}

// loadMigrations loads migrations from the directory
// For simulation (makemigrations): uses AST parsing
// For real DB execution (migrate): compiles and executes migrations
func loadMigrations(migrationsDir string, useCompiled bool) (*runner.Registry, error) {
	// Get package name from config
	cfg, err := config.LoadConfig("goosegorm.yml")
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	if useCompiled {
		// For real DB execution: compile and execute migrations
		return loader.LoadMigrationsFromCompiled(migrationsDir, cfg.PackageName)
	}

	// For simulation: use AST parsing
	return loader.LoadMigrationsFromAST(migrationsDir, cfg.PackageName)
}
