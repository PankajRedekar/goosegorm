package generator

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pankajredekar/goosegorm/internal/diff"
)

func TestNewGenerator(t *testing.T) {
	gen := NewGenerator("/tmp/migrations", "migrations")
	if gen == nil {
		t.Fatal("NewGenerator returned nil")
	}
	if gen.migrationsDir != "/tmp/migrations" {
		t.Errorf("Expected migrationsDir '/tmp/migrations', got '%s'", gen.migrationsDir)
	}
	if gen.packageName != "migrations" {
		t.Errorf("Expected packageName 'migrations', got '%s'", gen.packageName)
	}
}

func TestGenerateMigration(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	diffs := []diff.Diff{
		{
			Type:      "create_table",
			TableName: "users",
			Table: &diff.TableDiff{
				Name: "users",
				Columns: []*diff.ColumnDiff{
					{Name: "id", Type: "uint", PK: true, Null: false},
					{Name: "name", Type: "string", Null: false},
				},
			},
		},
	}

	filePath, err := gen.GenerateMigration("create_users", diffs)
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	if filePath == "" {
		t.Fatal("GenerateMigration returned empty file path")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatalf("Migration file was not created: %s", filePath)
	}

	// Read and verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for required components
	if !strings.Contains(contentStr, "package migrations") {
		t.Error("Migration file should contain package declaration")
	}
	if !strings.Contains(contentStr, "type CreateUsers struct{}") {
		t.Error("Migration file should contain struct definition")
	}
	if !strings.Contains(contentStr, "func (m CreateUsers) Version()") {
		t.Error("Migration file should contain Version method")
	}
	if !strings.Contains(contentStr, "func (m CreateUsers) Name()") {
		t.Error("Migration file should contain Name method")
	}
	if !strings.Contains(contentStr, "func (m CreateUsers) Up(db *gorm.DB)") {
		t.Error("Migration file should contain Up method")
	}
	if !strings.Contains(contentStr, "func (m CreateUsers) Down(db *gorm.DB)") {
		t.Error("Migration file should contain Down method")
	}
	if !strings.Contains(contentStr, "goosegorm.RegisterMigration") {
		t.Error("Migration file should contain RegisterMigration call")
	}
	if !strings.Contains(contentStr, "sim.CreateTable") {
		t.Error("Migration file should contain simulation code")
	}
}

func TestGenerateMigration_AddColumn(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	diffs := []diff.Diff{
		{
			Type:      "add_column",
			TableName: "users",
			Column: &diff.ColumnDiff{
				Name: "email",
				Type: "string",
				Null: false,
			},
		},
	}

	filePath, err := gen.GenerateMigration("add_email_to_users", diffs)
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "AddColumnWithOptions") {
		t.Error("Migration should contain AddColumnWithOptions for simulation")
	}
}

func TestGenerateVersion(t *testing.T) {
	version := generateVersion()
	// Version can be 14 (YYYYMMDDHHMMSS) or 18 (YYYYMMDDHHMMSS + 4-digit sequence)
	if len(version) != 14 && len(version) != 18 {
		t.Errorf("Expected version length 14 (YYYYMMDDHHMMSS) or 18 (with sequence), got %d", len(version))
	}

	// Version should be numeric
	for _, r := range version {
		if r < '0' || r > '9' {
			t.Errorf("Version should contain only digits, got '%s'", version)
			break
		}
	}

	// First 14 characters should be valid timestamp
	if len(version) >= 14 {
		baseVersion := version[:14]
		if len(baseVersion) != 14 {
			t.Errorf("Base version should be 14 characters, got %d", len(baseVersion))
		}
	}

	// If version has sequence, it should be 4 digits
	if len(version) == 18 {
		sequence := version[14:]
		if len(sequence) != 4 {
			t.Errorf("Sequence should be 4 digits, got %d: %s", len(sequence), sequence)
		}
		// Sequence should start from 0001, not 0000
		if sequence == "0000" {
			t.Errorf("Sequence should not be 0000, got %s", sequence)
		}
	}
}

func TestGenerateVersion_Sequence(t *testing.T) {
	// Reset the global counter to test sequence behavior
	// Note: This test may be flaky if run in parallel, but it's useful for verification
	versions := make([]string, 5)
	for i := 0; i < 5; i++ {
		// Small delay to ensure we're in the same second
		time.Sleep(10 * time.Millisecond)
		versions[i] = generateVersion()
	}

	// All versions should have the same base timestamp (first 14 chars)
	baseVersion := versions[0][:14]
	for i, v := range versions {
		if len(v) < 14 {
			t.Errorf("Version %d is too short: %s", i, v)
			continue
		}
		if v[:14] != baseVersion {
			t.Logf("Warning: Version %d has different base timestamp (expected same second): %s", i, v)
		}
	}

	// Check that sequences are properly formatted (0001, 0002, etc.)
	// Note: First version might be 14 chars (no sequence) or 18 chars (with sequence)
	hasSequence := false
	for i, v := range versions {
		if len(v) == 18 {
			hasSequence = true
			sequence := v[14:]
			// Allow some flexibility since counter is global
			seqNum, err := strconv.Atoi(sequence)
			if err != nil {
				t.Errorf("Version %d has invalid sequence: %s", i, sequence)
			} else if seqNum < 1 || seqNum > 9999 {
				t.Errorf("Version %d has sequence out of range: %s", i, sequence)
			}
		}
	}

	if !hasSequence {
		t.Log("No versions with sequence found (all generated in different seconds)")
	}
}

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"create users", "create_users"},
		{"add-email-to-users", "add_email_to_users"},
		{"test", "test"},
		{"UPPERCASE", "uppercase"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeName(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestToCamelCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"create_users", "CreateUsers"},
		{"add_email_to_users", "AddEmailToUsers"},
		{"test", "Test"},
		{"single", "Single"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := toCamelCase(tt.input)
			if result != tt.expected {
				t.Errorf("toCamelCase(%s) = %s, expected %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGenerateMigration_EmptyDiffs(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	_, err := gen.GenerateMigration("test", []diff.Diff{})
	if err == nil {
		t.Error("GenerateMigration should fail with empty diffs")
	}
	if !strings.Contains(err.Error(), "no diffs") {
		t.Errorf("Expected error about no diffs, got: %v", err)
	}
}

func TestGenerateMigration_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "new", "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	diffs := []diff.Diff{
		{
			Type:      "create_table",
			TableName: "users",
			Table: &diff.TableDiff{
				Name:    "users",
				Columns: []*diff.ColumnDiff{},
			},
		},
	}

	_, err := gen.GenerateMigration("test", diffs)
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	// Directory should have been created
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		t.Fatal("Migrations directory should have been created")
	}
}

func TestGenerateMigration_AddIndex(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	diffs := []diff.Diff{
		{
			Type:      "add_index",
			TableName: "users",
			Index: &diff.IndexDiff{
				Name:   "idx_email",
				Unique: false,
				Fields: []string{"email"},
			},
		},
	}

	filePath, err := gen.GenerateMigration("add_email_index", diffs)
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for index creation in simulation
	if !strings.Contains(contentStr, "AddIndex(\"idx_email\")") {
		t.Error("Migration should contain AddIndex for simulation")
	}

	// Check for index creation in real DB
	if !strings.Contains(contentStr, "CREATE INDEX IF NOT EXISTS idx_email") {
		t.Error("Migration should contain CREATE INDEX for real DB")
	}

	// Check for index drop in Down method
	if !strings.Contains(contentStr, "DROP INDEX IF EXISTS idx_email") {
		t.Error("Migration Down method should contain DROP INDEX")
	}
}

func TestGenerateMigration_DropIndex(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	diffs := []diff.Diff{
		{
			Type:      "drop_index",
			TableName: "users",
			Index: &diff.IndexDiff{
				Name:   "idx_email",
				Unique: false,
				Fields: []string{"email"},
			},
		},
	}

	filePath, err := gen.GenerateMigration("drop_email_index", diffs)
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for index drop in simulation
	if !strings.Contains(contentStr, "DropIndex(\"idx_email\")") {
		t.Error("Migration should contain DropIndex for simulation")
	}

	// Check for index drop in real DB
	if !strings.Contains(contentStr, "DROP INDEX IF EXISTS idx_email") {
		t.Error("Migration should contain DROP INDEX for real DB")
	}

	// Check for index recreation in Down method
	if !strings.Contains(contentStr, "CREATE INDEX IF NOT EXISTS idx_email") {
		t.Error("Migration Down method should contain CREATE INDEX")
	}
}

func TestGenerateMigration_UniqueIndex(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	diffs := []diff.Diff{
		{
			Type:      "add_index",
			TableName: "users",
			Index: &diff.IndexDiff{
				Name:   "idx_email_unique",
				Unique: true,
				Fields: []string{"email"},
			},
		},
	}

	filePath, err := gen.GenerateMigration("add_unique_email_index", diffs)
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for UNIQUE keyword in index creation
	if !strings.Contains(contentStr, "CREATE UNIQUE INDEX IF NOT EXISTS idx_email_unique") {
		t.Error("Migration should contain CREATE UNIQUE INDEX for unique indexes")
	}
}

func TestGenerateMigration_CompositeIndex(t *testing.T) {
	tmpDir := t.TempDir()
	migrationsDir := filepath.Join(tmpDir, "migrations")
	gen := NewGenerator(migrationsDir, "migrations")

	diffs := []diff.Diff{
		{
			Type:      "add_index",
			TableName: "users",
			Index: &diff.IndexDiff{
				Name:   "idx_name_email",
				Unique: false,
				Fields: []string{"name", "email"},
			},
		},
	}

	filePath, err := gen.GenerateMigration("add_composite_index", diffs)
	if err != nil {
		t.Fatalf("GenerateMigration failed: %v", err)
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read migration file: %v", err)
	}

	contentStr := string(content)

	// Check for composite index with multiple fields
	if !strings.Contains(contentStr, "name, email") {
		t.Error("Migration should contain composite index fields")
	}

	// Check for index creation
	if !strings.Contains(contentStr, "CREATE INDEX IF NOT EXISTS idx_name_email") {
		t.Error("Migration should contain CREATE INDEX for composite index")
	}
}
