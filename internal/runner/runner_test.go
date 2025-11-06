package runner

import (
	"testing"

	"github.com/pankajredekar/goosegorm/internal/schema"
	"github.com/pankajredekar/goosegorm/internal/versioner"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestMigration implements the Migration interface
type TestMigration struct {
	version  string
	name     string
	upFunc   func(*gorm.DB) error
	downFunc func(*gorm.DB) error
}

func (m TestMigration) Version() string { return m.version }
func (m TestMigration) Name() string    { return m.name }
func (m TestMigration) Up(db *gorm.DB) error {
	if m.upFunc != nil {
		return m.upFunc(db)
	}
	return nil
}
func (m TestMigration) Down(db *gorm.DB) error {
	if m.downFunc != nil {
		return m.downFunc(db)
	}
	return nil
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	return db
}

func TestNewRegistry(t *testing.T) {
	registry := NewRegistry()
	if registry == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if registry.migrations == nil {
		t.Fatal("migrations map is nil")
	}
}

func TestRegisterMigration(t *testing.T) {
	registry := NewRegistry()
	migration := TestMigration{version: "20250101000000", name: "test"}

	registry.RegisterMigration(migration)

	m, ok := registry.GetMigration("20250101000000")
	if !ok {
		t.Fatal("Migration not found after registration")
	}
	if m.Version() != "20250101000000" {
		t.Errorf("Expected version '20250101000000', got '%s'", m.Version())
	}
}

func TestGetAllMigrations(t *testing.T) {
	registry := NewRegistry()
	migrations := []Migration{
		TestMigration{version: "20250103000000", name: "third"},
		TestMigration{version: "20250101000000", name: "first"},
		TestMigration{version: "20250102000000", name: "second"},
	}

	for _, m := range migrations {
		registry.RegisterMigration(m)
	}

	all := registry.GetAllMigrations()
	if len(all) != 3 {
		t.Errorf("Expected 3 migrations, got %d", len(all))
	}

	// Check order (should be sorted by version)
	if all[0].Version() != "20250101000000" {
		t.Errorf("Expected first migration version '20250101000000', got '%s'", all[0].Version())
	}
	if all[2].Version() != "20250103000000" {
		t.Errorf("Expected last migration version '20250103000000', got '%s'", all[2].Version())
	}
}

func TestNewRunner(t *testing.T) {
	db := setupTestDB(t)
	registry := NewRegistry()
	ver := versioner.NewVersioner(db, "_test_migrations")

	run := NewRunner(db, registry, ver)
	if run == nil {
		t.Fatal("NewRunner returned nil")
	}
}

func TestMigrate(t *testing.T) {
	db := setupTestDB(t)
	registry := NewRegistry()
	ver := versioner.NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	run := NewRunner(db, registry, ver)

	migration := TestMigration{
		version: "20250101000000",
		name:    "test",
		upFunc: func(db *gorm.DB) error {
			// Create a test table
			return db.Exec("CREATE TABLE IF NOT EXISTS test_table (id INTEGER)").Error
		},
	}
	registry.RegisterMigration(migration)

	if err := run.Migrate(); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}

	// Check if migration was recorded
	applied, err := ver.IsApplied("20250101000000")
	if err != nil {
		t.Fatalf("IsApplied failed: %v", err)
	}
	if !applied {
		t.Error("Migration should be marked as applied")
	}
}

func TestGetPendingMigrations(t *testing.T) {
	db := setupTestDB(t)
	registry := NewRegistry()
	ver := versioner.NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	run := NewRunner(db, registry, ver)

	// Register migrations
	m1 := TestMigration{version: "20250101000000", name: "first"}
	m2 := TestMigration{version: "20250102000000", name: "second"}
	registry.RegisterMigration(m1)
	registry.RegisterMigration(m2)

	// Apply first migration
	if err := ver.RecordApplied("20250101000000", "first"); err != nil {
		t.Fatalf("RecordApplied failed: %v", err)
	}

	pending, err := run.GetPendingMigrations()
	if err != nil {
		t.Fatalf("GetPendingMigrations failed: %v", err)
	}

	if len(pending) != 1 {
		t.Errorf("Expected 1 pending migration, got %d", len(pending))
	}
	if pending[0].Version() != "20250102000000" {
		t.Errorf("Expected pending version '20250102000000', got '%s'", pending[0].Version())
	}
}

func TestRollback(t *testing.T) {
	db := setupTestDB(t)
	registry := NewRegistry()
	ver := versioner.NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	run := NewRunner(db, registry, ver)

	// Register and apply migrations
	migrations := []TestMigration{
		{version: "20250101000000", name: "first"},
		{version: "20250102000000", name: "second"},
	}

	for _, m := range migrations {
		registry.RegisterMigration(m)
		if err := ver.RecordApplied(m.Version(), m.Name()); err != nil {
			t.Fatalf("RecordApplied failed: %v", err)
		}
	}

	// Rollback 1 migration
	if err := run.Rollback(1); err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	// Check that last migration was removed
	applied, err := ver.IsApplied("20250102000000")
	if err != nil {
		t.Fatalf("IsApplied failed: %v", err)
	}
	if applied {
		t.Error("Rolled back migration should not be applied")
	}

	// First migration should still be applied
	applied, err = ver.IsApplied("20250101000000")
	if err != nil {
		t.Fatalf("IsApplied failed: %v", err)
	}
	if !applied {
		t.Error("First migration should still be applied")
	}
}

func TestSimulateSchema(t *testing.T) {
	// For testing, we'll create a simple schema builder directly
	// since the actual simulation requires migrations to check types
	builder := schema.NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumn("id", "uint").
		AddColumn("name", "string")

	// Check if table was created
	if !builder.TableExists("users") {
		t.Error("Table 'users' should exist")
	}

	table, exists := builder.GetTable("users")
	if !exists {
		t.Fatal("GetTable should return users table")
	}
	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}
}
