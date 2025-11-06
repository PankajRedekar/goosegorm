package loader

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pankajredekar/goosegorm/internal/runner"
)

func TestLoadMigrationsFromAST(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create a test migration file
	migrationFile := filepath.Join(migrationsDir, "0001_create_user.go")
	migrationContent := `package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type CreateUser struct{}

func (m CreateUser) Version() string { return "20251106133644" }

func (m CreateUser) Name() string { return "create_user" }

func (m CreateUser) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.CreateTable("user").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("email", "string", false, false, true).
			AddColumnWithOptions("username", "string", false, false, true)
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

func init() {
	goosegorm.RegisterMigration(CreateUser{})
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

	// Check that migration was loaded
	allMigrations := registry.GetAllMigrations()
	if len(allMigrations) != 1 {
		t.Fatalf("Expected 1 migration, got %d", len(allMigrations))
	}

	migration := allMigrations[0]
	if migration.Version() != "20251106133644" {
		t.Errorf("Expected version '20251106133644', got '%s'", migration.Version())
	}
	if migration.Name() != "create_user" {
		t.Errorf("Expected name 'create_user', got '%s'", migration.Name())
	}
}

func TestASTInterpreter_CreateTable(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create a test migration file
	migrationFile := filepath.Join(migrationsDir, "0001_create_user.go")
	migrationContent := `package migrations

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
			AddColumnWithOptions("email", "string", false, false, true)
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

	// Verify table was created
	if !simulatedSchema.TableExists("user") {
		t.Error("Table 'user' should exist")
	}

	// Verify columns
	table, exists := simulatedSchema.GetTable("user")
	if !exists {
		t.Fatal("GetTable should return user table")
	}

	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}

	// Check id column
	idCol, exists := table.Columns["id"]
	if !exists {
		t.Error("Column 'id' should exist")
	} else {
		if idCol.Type != "bigint" {
			t.Errorf("Expected id type 'bigint', got '%s'", idCol.Type)
		}
		if !idCol.PK {
			t.Error("Column 'id' should be primary key")
		}
	}

	// Check email column
	emailCol, exists := table.Columns["email"]
	if !exists {
		t.Error("Column 'email' should exist")
	} else {
		if emailCol.Type != "string" {
			t.Errorf("Expected email type 'string', got '%s'", emailCol.Type)
		}
		if !emailCol.Unique {
			t.Error("Column 'email' should be unique")
		}
	}
}

func TestASTInterpreter_AddColumn(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create first migration - create table
	migration1File := filepath.Join(migrationsDir, "0001_create_user.go")
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
			AddColumnWithOptions("email", "string", false, false, true)
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

	if err := os.WriteFile(migration1File, []byte(migration1Content), 0644); err != nil {
		t.Fatalf("Failed to write migration file: %v", err)
	}

	// Create second migration - add column
	migration2File := filepath.Join(migrationsDir, "0002_add_name.go")
	migration2Content := `package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddName struct{}

func (m AddName) Version() string { return "20251106133744" }
func (m AddName) Name() string { return "add_name" }

func (m AddName) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.AlterTable("user").
			AddColumnWithOptions("name", "string", false, false, false)
		return nil
	}
	return nil
}

func (m AddName) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.AlterTable("user").DropColumn("name")
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

	// Verify all columns exist
	table, exists := simulatedSchema.GetTable("user")
	if !exists {
		t.Fatal("GetTable should return user table")
	}

	if len(table.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(table.Columns))
	}

	// Check name column was added
	nameCol, exists := table.Columns["name"]
	if !exists {
		t.Error("Column 'name' should exist")
	} else {
		if nameCol.Type != "string" {
			t.Errorf("Expected name type 'string', got '%s'", nameCol.Type)
		}
	}
}

func TestASTInterpreter_DropTable(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create migration with DropTable
	migrationFile := filepath.Join(migrationsDir, "0001_drop_user.go")
	migrationContent := `package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type DropUser struct{}

func (m DropUser) Version() string { return "20251106133644" }
func (m DropUser) Name() string { return "drop_user" }

func (m DropUser) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.DropTable("user")
		return nil
	}
	return nil
}

func (m DropUser) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.CreateTable("user").
			AddColumnWithOptions("id", "bigint", false, true, false)
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

	// Verify table was dropped (doesn't exist)
	if simulatedSchema.TableExists("user") {
		t.Error("Table 'user' should not exist after DropTable")
	}
}

func TestASTInterpreter_AddIndex(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create migration with AddIndex
	migrationFile := filepath.Join(migrationsDir, "0001_add_index.go")
	migrationContent := `package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddIndex struct{}

func (m AddIndex) Version() string { return "20251106133644" }
func (m AddIndex) Name() string { return "add_index" }

func (m AddIndex) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		sim.CreateTable("user").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("email", "string", false, false, false).
			AddIndex("idx_email")
		return nil
	}
	return nil
}

func (m AddIndex) Down(db *gorm.DB) error {
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

	// Verify table exists
	table, exists := simulatedSchema.GetTable("user")
	if !exists {
		t.Fatal("GetTable should return user table")
	}

	// Verify index was added
	if len(table.Indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(table.Indexes))
	}
	if table.Indexes[0] != "idx_email" {
		t.Errorf("Expected index 'idx_email', got '%s'", table.Indexes[0])
	}
}

func TestASTInterpreter_MultipleChainedCalls(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create migration with multiple chained calls
	migrationFile := filepath.Join(migrationsDir, "0001_create_user.go")
	migrationContent := `package migrations

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
			AddColumnWithOptions("email", "string", false, false, true).
			AddColumnWithOptions("username", "string", false, false, true).
			AddColumnWithOptions("password", "string", false, false, false).
			AddColumnWithOptions("first_name", "string", false, false, false).
			AddColumnWithOptions("last_name", "string", false, false, false).
			AddColumnWithOptions("is_active", "bool", false, false, false).
			AddColumnWithOptions("created_at", "timestamp", false, false, false).
			AddColumnWithOptions("updated_at", "timestamp", false, false, false)
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

	// Verify table exists
	table, exists := simulatedSchema.GetTable("user")
	if !exists {
		t.Fatal("GetTable should return user table")
	}

	// Verify all columns were added
	expectedColumns := []string{"id", "email", "username", "password", "first_name", "last_name", "is_active", "created_at", "updated_at"}
	if len(table.Columns) != len(expectedColumns) {
		t.Errorf("Expected %d columns, got %d", len(expectedColumns), len(table.Columns))
	}

	for _, colName := range expectedColumns {
		if _, exists := table.Columns[colName]; !exists {
			t.Errorf("Column '%s' should exist", colName)
		}
	}
}
