package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/schema"
)

// TestASTInterpreter_AddIndex_AlterTable tests the specific pattern used in generated migrations:
// sim.AlterTable("table").AddIndex("index_name")
func TestASTInterpreter_AddIndex_AlterTable(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create migration that first creates a table, then adds index via AlterTable
	migrationFile := filepath.Join(migrationsDir, "0001_create_user.go")
	migration1Content := `package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type CreateUser struct{}

func (m CreateUser) Version() string { return "20251106133644" }
func (m CreateUser) Name() string { return "create_user" }

func (m CreateUser) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.CreateTable("user").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("email", "string", false, false, false)
		return nil
	}
	return nil
}

func (m CreateUser) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.DropTable("user")
		return nil
	}
	return nil
}
`

	if err := os.WriteFile(migrationFile, []byte(migration1Content), 0644); err != nil {
		t.Fatalf("Failed to write migration file: %v", err)
	}

	// Create second migration that adds index using AlterTable pattern (as generated)
	migration2File := filepath.Join(migrationsDir, "0002_add_index.go")
	migration2Content := `package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddIndex struct{}

func (m AddIndex) Version() string { return "20251106133744" }
func (m AddIndex) Name() string { return "add_index" }

func (m AddIndex) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.AlterTable("user").AddIndex("idx_email")
		return nil
	}
	return nil
}

func (m AddIndex) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.AlterTable("user").DropIndex("idx_email")
		return nil
	}
	return nil
}
`

	if err := os.WriteFile(migration2File, []byte(migration2Content), 0644); err != nil {
		t.Fatalf("Failed to write migration file: %v", err)
	}

	// Load migrations
	registry, err := LoadMigrationsFromAST(migrationsDir, "migrations")
	if err != nil {
		t.Fatalf("LoadMigrationsFromAST failed: %v", err)
	}

	// Verify both migrations loaded
	allMigrations := registry.GetAllMigrations()
	if len(allMigrations) != 2 {
		t.Fatalf("Expected 2 migrations, got %d", len(allMigrations))
	}

	// Simulate schema
	simRunner := runner.NewRunner(nil, registry, nil)
	simulatedSchema, err := simRunner.SimulateSchema()
	if err != nil {
		t.Fatalf("SimulateSchema failed: %v", err)
	}

	// Verify table exists
	if !simulatedSchema.TableExists("user") {
		t.Error("Table 'user' should exist")
	}

	// Verify index was added
	table, exists := simulatedSchema.GetTable("user")
	if !exists {
		t.Fatal("GetTable should return user table")
	}

	// Check that index exists
	foundIndex := false
	for _, idxName := range table.Indexes {
		if idxName == "idx_email" {
			foundIndex = true
			break
		}
	}

	if !foundIndex {
		t.Errorf("Index 'idx_email' should exist. Found indexes: %v", table.Indexes)
	}

	if len(table.Indexes) != 1 {
		t.Errorf("Expected 1 index, got %d: %v", len(table.Indexes), table.Indexes)
	}
}

// TestASTInterpreter_AddIndex_Multiple tests adding multiple indexes via AlterTable
func TestASTInterpreter_AddIndex_Multiple(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create migration that adds multiple indexes using separate AlterTable calls
	migrationFile := filepath.Join(migrationsDir, "0001_add_indexes.go")
	migrationContent := `package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddIndexes struct{}

func (m AddIndexes) Version() string { return "20251106133644" }
func (m AddIndexes) Name() string { return "add_indexes" }

func (m AddIndexes) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.CreateTable("user").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("email", "string", false, false, false).
			AddColumnWithOptions("username", "string", false, false, false)
		sim.AlterTable("user").AddIndex("idx_email")
		sim.AlterTable("user").AddIndex("idx_username")
		return nil
	}
	return nil
}

func (m AddIndexes) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.DropTable("user")
		return nil
	}
	return nil
}
`

	if err := os.WriteFile(migrationFile, []byte(migrationContent), 0644); err != nil {
		t.Fatalf("Failed to write migration file: %v", err)
	}

	// Load migrations
	registry, err := LoadMigrationsFromAST(migrationsDir, "migrations")
	if err != nil {
		t.Fatalf("LoadMigrationsFromAST failed: %v", err)
	}

	// Simulate schema
	simRunner := runner.NewRunner(nil, registry, nil)
	simulatedSchema, err := simRunner.SimulateSchema()
	if err != nil {
		t.Fatalf("SimulateSchema failed: %v", err)
	}

	// Verify indexes were added
	table, exists := simulatedSchema.GetTable("user")
	if !exists {
		t.Fatal("GetTable should return user table")
	}

	// Check that both indexes exist
	expectedIndexes := map[string]bool{
		"idx_email":    true,
		"idx_username": true,
	}

	for _, idxName := range table.Indexes {
		if !expectedIndexes[idxName] {
			t.Errorf("Unexpected index: %s", idxName)
		}
		delete(expectedIndexes, idxName)
	}

	if len(expectedIndexes) > 0 {
		t.Errorf("Missing indexes: %v. Found indexes: %v", expectedIndexes, table.Indexes)
	}

	if len(table.Indexes) != 2 {
		t.Errorf("Expected 2 indexes, got %d: %v", len(table.Indexes), table.Indexes)
	}
}

// TestASTInterpreter_AddIndex_Direct tests direct schema builder manipulation
func TestASTInterpreter_AddIndex_Direct(t *testing.T) {
	// Test that AddIndex works correctly when called directly
	builder := schema.NewSchemaBuilder()
	builder.CreateTable("user").
		AddColumnWithOptions("id", "bigint", false, true, false).
		AddColumnWithOptions("email", "string", false, false, false)

	// Add index using AlterTable pattern (as generated migrations do)
	builder.AlterTable("user").AddIndex("idx_email")

	table, exists := builder.GetTable("user")
	if !exists {
		t.Fatal("GetTable should return user table")
	}

	foundIndex := false
	for _, idxName := range table.Indexes {
		if idxName == "idx_email" {
			foundIndex = true
			break
		}
	}

	if !foundIndex {
		t.Errorf("Index 'idx_email' should exist. Found indexes: %v", table.Indexes)
	}
}
