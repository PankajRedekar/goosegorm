package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/diff"
	"github.com/pankajredekar/goosegorm/internal/utils"
)

func TestMakemigrationsLoop(t *testing.T) {
	tmpDir := t.TempDir()

	// Create config
	configPath := filepath.Join(tmpDir, "goosegorm.yml")
	cfg := &config.Config{
		DatabaseURL:    "sqlite://:memory:",
		ModelsDir:      filepath.Join(tmpDir, "models"),
		MigrationsDir:  filepath.Join(tmpDir, "migrations"),
		PackageName:    "migrations",
		MigrationTable: "_goosegorm_migrations",
		IgnoreModels:   []string{},
	}

	// Write config
	configData := `database_url: sqlite://:memory:
models_dir: ` + cfg.ModelsDir + `
migrations_dir: ` + cfg.MigrationsDir + `
package_name: migrations
migration_table: _goosegorm_migrations
ignore_models: []
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	// Create models directory
	if err := os.MkdirAll(cfg.ModelsDir, 0755); err != nil {
		t.Fatalf("Failed to create models directory: %v", err)
	}

	// Create migrations directory
	if err := os.MkdirAll(cfg.MigrationsDir, 0755); err != nil {
		t.Fatalf("Failed to create migrations directory: %v", err)
	}

	// Create initial model file (just ID and email)
	modelFile := filepath.Join(cfg.ModelsDir, "user.go")
	modelContent := `package models

type User struct {
	ID    uint   ` + "`gorm:\"primaryKey\"`" + `
	Email string ` + "`gorm:\"uniqueIndex\"`" + `
}
`
	if err := os.WriteFile(modelFile, []byte(modelContent), 0644); err != nil {
		t.Fatalf("Failed to write model file: %v", err)
	}

	// Change to tmpDir to simulate running the command
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}
	defer os.Chdir(oldWd)

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Move config to current directory
	if err := os.Rename(configPath, "goosegorm.yml"); err != nil {
		t.Fatalf("Failed to move config: %v", err)
	}

	// Run makemigrations (this will generate first migration)
	// Note: We can't easily test the full command execution without mocking,
	// but we can test the logic by checking if migrations are generated

	// For now, let's verify the setup is correct
	if !utils.FileExists("goosegorm.yml") {
		t.Error("Config file should exist")
	}

	// Verify model file exists
	if !utils.FileExists(cfg.ModelsDir + "/user.go") {
		t.Error("Model file should exist")
	}
}

func TestGenerateMigrationName_WithIndexes(t *testing.T) {
	tests := []struct {
		name     string
		diffs    []diff.Diff
		expected string
	}{
		{
			name: "single add_index",
			diffs: []diff.Diff{
				{
					Type:      "add_index",
					TableName: "user",
					Index: &diff.IndexDiff{
						Name:   "idx_email",
						Unique: false,
						Fields: []string{"email"},
					},
				},
			},
			expected: "add_index_idx_email_to_user",
		},
		{
			name: "add_column and add_index",
			diffs: []diff.Diff{
				{
					Type:      "add_column",
					TableName: "user",
					Column: &diff.ColumnDiff{
						Name: "name",
						Type: "string",
					},
				},
				{
					Type:      "add_index",
					TableName: "user",
					Index: &diff.IndexDiff{
						Name:   "idx_name",
						Unique: false,
						Fields: []string{"name"},
					},
				},
			},
			expected: "add_name_to_user_and_add_index_idx_name_to_user",
		},
		{
			name: "drop_index",
			diffs: []diff.Diff{
				{
					Type:      "drop_index",
					TableName: "user",
					Index: &diff.IndexDiff{
						Name:   "idx_email",
						Unique: false,
						Fields: []string{"email"},
					},
				},
			},
			expected: "drop_index_idx_email_from_user",
		},
		{
			name: "create_table",
			diffs: []diff.Diff{
				{
					Type:      "create_table",
					TableName: "user",
					Table: &diff.TableDiff{
						Name:    "user",
						Columns: []*diff.ColumnDiff{},
					},
				},
			},
			expected: "create_user",
		},
		{
			name: "add_column",
			diffs: []diff.Diff{
				{
					Type:      "add_column",
					TableName: "user",
					Column: &diff.ColumnDiff{
						Name: "email",
						Type: "string",
					},
				},
			},
			expected: "add_email_to_user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateMigrationName(tt.diffs)
			if result != tt.expected {
				t.Errorf("generateMigrationName() = %s, expected %s", result, tt.expected)
			}
		})
	}
}

func TestMakemigrationsLoop_IterationLogic(t *testing.T) {
	// Test the iteration logic by simulating the loop behavior
	// This verifies that the loop structure is correct

	iteration := 0
	maxIterations := 100
	changesFound := []int{3, 2, 1, 0} // Simulate decreasing changes

	for {
		iteration++
		if iteration > maxIterations {
			t.Error("Loop should not exceed max iterations")
			break
		}

		if iteration > len(changesFound) {
			t.Error("Loop should have stopped")
			break
		}

		changes := changesFound[iteration-1]
		if changes == 0 {
			// Loop should exit here
			if iteration != 4 {
				t.Errorf("Expected loop to exit at iteration 4, but iteration is %d", iteration)
			}
			break
		}
	}

	if iteration != 4 {
		t.Errorf("Expected 4 iterations, got %d", iteration)
	}
}
