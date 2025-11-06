package cli

import (
	"os"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/utils"
	"github.com/pankajredekar/goosegorm/internal/versioner"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply pending migrations",
	Long:  "Applies all migrations that haven't been applied yet",
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

		// Connect to database
		db, err := connectDB(cfg.DatabaseURL)
		if err != nil {
			utils.PrintError("Failed to connect to database: %v", err)
			os.Exit(1)
		}

		// Initialize versioner
		ver := versioner.NewVersioner(db, cfg.MigrationTable)
		if err := ver.Initialize(); err != nil {
			utils.PrintError("Failed to initialize version table: %v", err)
			os.Exit(1)
		}

		// Load migrations
		registry, err := loadMigrations(cfg.MigrationsDir)
		if err != nil {
			utils.PrintError("Failed to load migrations: %v", err)
			os.Exit(1)
		}

		// Create runner
		run := runner.NewRunner(db, registry, ver)

		// Get pending migrations
		pending, err := run.GetPendingMigrations()
		if err != nil {
			utils.PrintError("Failed to get pending migrations: %v", err)
			os.Exit(1)
		}

		if len(pending) == 0 {
			utils.PrintSuccess("No pending migrations")
			return
		}

		utils.PrintInfo("Applying %d migration(s)...", len(pending))

		// Apply migrations
		if err := run.Migrate(); err != nil {
			utils.PrintError("Failed to apply migrations: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Applied %d migration(s)", len(pending))
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
