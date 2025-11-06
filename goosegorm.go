package goosegorm

import (
	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/schema"
	"gorm.io/gorm"
)

// Migration interface that all migrations must implement
type Migration = runner.Migration

// SchemaBuilder is exported for use in migrations
type SchemaBuilder = schema.SchemaBuilder

// NewSchemaBuilder creates a new schema builder
func NewSchemaBuilder() *SchemaBuilder {
	return schema.NewSchemaBuilder()
}

// RegisterMigration registers a migration globally
var globalRegistry *runner.Registry

func init() {
	globalRegistry = runner.NewRegistry()
}

// RegisterMigration registers a migration in the global registry
func RegisterMigration(m Migration) {
	if globalRegistry == nil {
		globalRegistry = runner.NewRegistry()
	}
	globalRegistry.RegisterMigration(m)
}

// GetGlobalRegistry returns the global registry
func GetGlobalRegistry() *runner.Registry {
	return globalRegistry
}

// SetGlobalRegistry sets the global registry (for testing)
func SetGlobalRegistry(reg *runner.Registry) {
	globalRegistry = reg
}

// Helper function to check if db is a SchemaBuilder (for migrations)
func IsSchemaBuilder(db *gorm.DB) (*schema.SchemaBuilder, bool) {
	// This is a type assertion helper
	// Migrations will need to use interface{} and check manually
	return nil, false
}

// Helper to convert interface{} to SchemaBuilder
func AsSchemaBuilder(db interface{}) (*schema.SchemaBuilder, bool) {
	if sb, ok := db.(*schema.SchemaBuilder); ok {
		return sb, true
	}
	return nil, false
}
