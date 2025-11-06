package generator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if len(version) != 14 {
		t.Errorf("Expected version length 14 (YYYYMMDDHHMMSS), got %d", len(version))
	}

	// Version should be numeric
	for _, r := range version {
		if r < '0' || r > '9' {
			t.Errorf("Version should contain only digits, got '%s'", version)
			break
		}
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
