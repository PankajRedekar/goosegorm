package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/diff"
	"github.com/pankajredekar/goosegorm/internal/generator"
	"github.com/pankajredekar/goosegorm/internal/loader"
	"github.com/pankajredekar/goosegorm/internal/modelreflect"
	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/schema"
	"github.com/pankajredekar/goosegorm/internal/utils"
	"github.com/spf13/cobra"
)

var makemigrationsCmd = &cobra.Command{
	Use:   "makemigrations [migration_name]",
	Short: "Generate new migration files",
	Long:  "Compares the current models with the simulated schema and generates migration files. Use --empty to create an empty migration file.",
	Run: func(cmd *cobra.Command, args []string) {
		configPath := "goosegorm.yml"
		if !utils.FileExists(configPath) {
			utils.PrintError("goosegorm.yml not found. Run 'goosegorm init' first")
			os.Exit(1)
		}

		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			utils.PrintError("Failed to load config: %v", err)
			os.Exit(1)
		}

		if err := cfg.Validate(); err != nil {
			utils.PrintError("Invalid config: %v", err)
			os.Exit(1)
		}

		// Check for --empty flag
		emptyFlag, _ := cmd.Flags().GetBool("empty")
		if emptyFlag {
			// Handle empty migration generation
			var migrationName string
			if len(args) > 0 {
				migrationName = args[0]
			}
			// If no name provided, will use Migration{version} format

			gen := generator.NewGenerator(cfg.MigrationsDir, cfg.PackageName)
			filePath, err := gen.GenerateEmptyMigration(migrationName)
			if err != nil {
				utils.PrintError("Failed to generate empty migration: %v", err)
				os.Exit(1)
			}

			utils.PrintSuccess("Generated empty migration: %s", filepath.Base(filePath))
			return
		}

		// Parse models (only need to do this once)
		models, err := modelreflect.ParseModelsFromDir(cfg.ModelsDir, cfg.IgnoreModels)
		if err != nil {
			utils.PrintError("Failed to parse models: %v", err)
			os.Exit(1)
		}

		// Filter managed models
		var managedModels []modelreflect.ParsedModel
		for _, m := range models {
			if m.Managed && !m.ShouldIgnore(cfg.IgnoreModels) {
				managedModels = append(managedModels, m)
			}
		}

		utils.PrintInfo("Found %d managed models", len(managedModels))

		// Initialize variables for loop
		var registry *runner.Registry
		var simulatedSchema *schema.SchemaBuilder

		// Loop until no changes are found
		iteration := 0
		maxIterations := 100 // Safety limit to prevent infinite loops

		for {
			iteration++
			if iteration > maxIterations {
				utils.PrintError("Maximum iterations (%d) reached. Stopping to prevent infinite loop.", maxIterations)
				os.Exit(1)
			}

			// Reload migrations to include newly generated ones
			registry, err = loadMigrationsFromDir(cfg.MigrationsDir, cfg.PackageName)
			if err != nil {
				utils.PrintError("Failed to load migrations: %v", err)
				os.Exit(1)
			}

			// Simulate schema from existing migrations
			simRunner := runner.NewRunner(nil, registry, nil)
			simulatedSchema, err = simRunner.SimulateSchema()
			if err != nil {
				utils.PrintError("Failed to simulate schema: %v", err)
				os.Exit(1)
			}

			// Compare schema
			diffs, err := diff.CompareSchema(simulatedSchema.Schema, managedModels)
			if err != nil {
				utils.PrintError("Failed to compare schema: %v", err)
				os.Exit(1)
			}

			if len(diffs) == 0 {
				if iteration == 1 {
					utils.PrintSuccess("No changes detected")
				} else {
					utils.PrintSuccess("No more changes detected after %d iteration(s)", iteration-1)
				}
				return
			}

			utils.PrintInfo("Found %d changes (iteration %d)", len(diffs), iteration)

			// Generate migration name from diffs
			migrationName := generateMigrationName(diffs)

			// Generate migration file
			gen := generator.NewGenerator(cfg.MigrationsDir, cfg.PackageName)
			filePath, err := gen.GenerateMigration(migrationName, diffs)
			if err != nil {
				utils.PrintError("Failed to generate migration: %v", err)
				os.Exit(1)
			}

			utils.PrintSuccess("Generated migration: %s", filepath.Base(filePath))

			// Continue loop to check for more changes
		}
	},
}

func generateMigrationName(diffs []diff.Diff) string {
	var parts []string
	for _, d := range diffs {
		switch d.Type {
		case "create_table":
			parts = append(parts, "create_"+d.TableName)
		case "drop_table":
			parts = append(parts, "drop_"+d.TableName)
		case "add_column":
			parts = append(parts, "add_"+d.Column.Name+"_to_"+d.TableName)
		case "drop_column":
			parts = append(parts, "drop_"+d.Column.Name+"_from_"+d.TableName)
		case "modify_column":
			parts = append(parts, "modify_"+d.Column.Name+"_in_"+d.TableName)
		case "add_index":
			if d.Index != nil {
				parts = append(parts, "add_index_"+d.Index.Name+"_to_"+d.TableName)
			}
		case "drop_index":
			if d.Index != nil {
				parts = append(parts, "drop_index_"+d.Index.Name+"_from_"+d.TableName)
			}
		}
	}
	if len(parts) == 0 {
		return "migration"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return strings.Join(parts, "_and_")
}

func loadMigrationsFromDir(dir string, packageName string) (*runner.Registry, error) {
	// Use the loader package to load migrations
	return loader.LoadMigrationsFromAST(dir, packageName)
}

// findModulePath finds the module path from go.mod
func findModulePath(dir string) (string, error) {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		// Try parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		return findModulePath(parent)
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}

	// Simple parsing: find "module " line
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module path not found in go.mod")
}

func init() {
	makemigrationsCmd.Flags().Bool("empty", false, "Create an empty migration file")
	rootCmd.AddCommand(makemigrationsCmd)
}
