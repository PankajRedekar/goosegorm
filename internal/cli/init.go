package cli

import (
	"os"

	"github.com/pankajredekar/goosegorm/internal/utils"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize GooseGORM project",
	Long:  "Creates a goosegorm.yml configuration file in the current directory",
	Run: func(cmd *cobra.Command, args []string) {
		configPath := "goosegorm.yml"
		if utils.FileExists(configPath) {
			utils.PrintWarning("goosegorm.yml already exists")
			return
		}

		config := map[string]interface{}{
			"database_url":    "postgres://user:pass@localhost:5432/myapp?sslmode=disable",
			"models_dir":      "./models",
			"migrations_dir":  "./migrations",
			"package_name":    "migrations",
			"migration_table": "_goosegorm_migrations",
			"ignore_models":   []string{},
		}

		data, err := yaml.Marshal(config)
		if err != nil {
			utils.PrintError("Failed to generate config: %v", err)
			os.Exit(1)
		}

		if err := os.WriteFile(configPath, data, 0644); err != nil {
			utils.PrintError("Failed to write config file: %v", err)
			os.Exit(1)
		}

		// Create directories
		modelsDir := "./models"
		migrationsDir := "./migrations"

		if err := os.MkdirAll(modelsDir, 0755); err != nil {
			utils.PrintError("Failed to create models directory: %v", err)
			os.Exit(1)
		}

		if err := os.MkdirAll(migrationsDir, 0755); err != nil {
			utils.PrintError("Failed to create migrations directory: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Initialized GooseGORM project")
		utils.PrintInfo("Created goosegorm.yml")
		utils.PrintInfo("Created %s directory", modelsDir)
		utils.PrintInfo("Created %s directory", migrationsDir)
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
