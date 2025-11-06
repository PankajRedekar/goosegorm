package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/utils"
	"github.com/pankajredekar/goosegorm/internal/versioner"
	"github.com/spf13/cobra"
)

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show migration status",
	Long:  "Shows all applied and pending migrations",
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

		// Get applied migrations
		applied, err := run.GetAppliedMigrations()
		if err != nil {
			utils.PrintError("Failed to get applied migrations: %v", err)
			os.Exit(1)
		}

		// Get pending migrations
		pending, err := run.GetPendingMigrations()
		if err != nil {
			utils.PrintError("Failed to get pending migrations: %v", err)
			os.Exit(1)
		}

		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("Migration Status")
		fmt.Println(strings.Repeat("=", 60))

		if len(applied) > 0 {
			fmt.Println("\n✓ Applied Migrations:")
			for _, m := range applied {
				fmt.Printf("  %s - %s\n", m.Version(), m.Name())
			}
		} else {
			fmt.Println("\n✓ Applied Migrations: (none)")
		}

		if len(pending) > 0 {
			fmt.Println("\n○ Pending Migrations:")
			for _, m := range pending {
				fmt.Printf("  %s - %s\n", m.Version(), m.Name())
			}
		} else {
			fmt.Println("\n○ Pending Migrations: (none)")
		}

		fmt.Println()
	},
}

func init() {
	rootCmd.AddCommand(showCmd)
}
