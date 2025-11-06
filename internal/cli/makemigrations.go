package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/diff"
	"github.com/pankajredekar/goosegorm/internal/generator"
	"github.com/pankajredekar/goosegorm/internal/loader"
	"github.com/pankajredekar/goosegorm/internal/modelreflect"
	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/utils"
	"github.com/spf13/cobra"
)

var makemigrationsCmd = &cobra.Command{
	Use:   "makemigrations",
	Short: "Generate new migration files",
	Long:  "Compares the current models with the simulated schema and generates migration files",
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

		// Load existing migrations
		registry, err := loadMigrationsFromDir(cfg.MigrationsDir, cfg.PackageName)
		if err != nil {
			utils.PrintError("Failed to load migrations: %v", err)
			os.Exit(1)
		}

		// Simulate schema from existing migrations
		simRunner := runner.NewRunner(nil, registry, nil)
		simulatedSchema, err := simRunner.SimulateSchema()
		if err != nil {
			utils.PrintError("Failed to simulate schema: %v", err)
			os.Exit(1)
		}

		// Parse models
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

		// Compare schema
		diffs, err := diff.CompareSchema(simulatedSchema.Schema, managedModels)
		if err != nil {
			utils.PrintError("Failed to compare schema: %v", err)
			os.Exit(1)
		}

		if len(diffs) == 0 {
			utils.PrintSuccess("No changes detected")
			return
		}

		utils.PrintInfo("Found %d changes", len(diffs))

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

func init() {
	rootCmd.AddCommand(makemigrationsCmd)
}
