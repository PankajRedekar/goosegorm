package cli

import (
	"os"
	"strconv"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/utils"
	"github.com/pankajredekar/goosegorm/internal/versioner"
	"github.com/spf13/cobra"
)

var rollbackCmd = &cobra.Command{
	Use:   "rollback [n]",
	Short: "Rollback migrations",
	Long:  "Rolls back the last N migrations (default: 1)",
	Args:  cobra.MaximumNArgs(1),
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

		n := 1
		if len(args) > 0 {
			n, err = strconv.Atoi(args[0])
			if err != nil {
				utils.PrintError("Invalid number: %v", err)
				os.Exit(1)
			}
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

		// Load migrations - use compiled execution for real DB operations
		registry, err := loadMigrations(cfg.MigrationsDir, true)
		if err != nil {
			utils.PrintError("Failed to load migrations: %v", err)
			os.Exit(1)
		}

		// Create runner
		run := runner.NewRunner(db, registry, ver)

		// Get applied count
		appliedCount, err := ver.GetAppliedCount()
		if err != nil {
			utils.PrintError("Failed to get applied count: %v", err)
			os.Exit(1)
		}

		if appliedCount == 0 {
			utils.PrintWarning("No migrations to rollback")
			return
		}

		if int64(n) > appliedCount {
			n = int(appliedCount)
		}

		utils.PrintInfo("Rolling back %d migration(s)...", n)

		// Rollback
		if err := run.Rollback(n); err != nil {
			utils.PrintError("Failed to rollback: %v", err)
			os.Exit(1)
		}

		utils.PrintSuccess("Rolled back %d migration(s)", n)
	},
}

func init() {
	rootCmd.AddCommand(rollbackCmd)
}
