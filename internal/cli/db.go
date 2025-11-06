package cli

import (
	"fmt"
	"strings"

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
// Note: This is a simplified version - in production, you'd need to actually
// compile and import the migration packages to register them
func loadMigrations(migrationsDir string) *runner.Registry {
	registry := runner.NewRegistry()
	// TODO: Actually load and register migrations from the directory
	return registry
}
