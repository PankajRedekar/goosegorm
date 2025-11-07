package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pankajredekar/goosegorm/internal/config"
	"github.com/pankajredekar/goosegorm/internal/utils"
	"github.com/spf13/cobra"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "Apply pending migrations",
	Long:  "Applies all migrations that haven't been applied yet using compiled migrator",
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

		// Get the directory where goosegorm.yml is located
		configDir, err := os.Getwd()
		if err != nil {
			utils.PrintError("Failed to get current directory: %v", err)
			os.Exit(1)
		}

		// Check if migrations directory exists and has migration files
		migrationsAbsPath, err := filepath.Abs(cfg.MigrationsDir)
		if err != nil {
			utils.PrintError("Failed to get absolute path for migrations: %v", err)
			os.Exit(1)
		}

		// Check if migrations directory exists
		if !utils.FileExists(migrationsAbsPath) {
			utils.PrintError("Migrations directory does not exist: %s", migrationsAbsPath)
			utils.PrintInfo("Please run 'goosegorm makemigrations' first to create initial migrations")
			os.Exit(1)
		}

		// Check if there are any .go files in the migrations directory
		hasMigrations, err := utils.HasMigrationFiles(migrationsAbsPath)
		if err != nil {
			utils.PrintError("Failed to check migrations directory: %v", err)
			os.Exit(1)
		}

		if !hasMigrations {
			utils.PrintError("No migration files found in: %s", migrationsAbsPath)
			utils.PrintInfo("Please run 'goosegorm makemigrations' first to create initial migrations")
			os.Exit(1)
		}

		// Find module path
		modulePath, err := findModulePath(configDir)
		if err != nil {
			utils.PrintError("Failed to find module path: %v", err)
			os.Exit(1)
		}

		// Calculate the relative path from project root to migrations directory
		// migrationsAbsPath was already calculated above
		relPath, err := filepath.Rel(configDir, migrationsAbsPath)
		if err != nil {
			utils.PrintError("Failed to calculate relative path: %v", err)
			os.Exit(1)
		}
		// Convert to forward slashes for import path (Go uses forward slashes)
		relPath = filepath.ToSlash(relPath)
		// Build the import path: modulePath/relativePath
		migrationsImportPath := fmt.Sprintf("%s/%s", modulePath, relPath)

		// Create temporary migrator package in the same directory as goosegorm.yml
		tempMigratorDir := filepath.Join(configDir, ".goosegorm_migrator")
		defer os.RemoveAll(tempMigratorDir) // Clean up after migration

		if err := os.MkdirAll(tempMigratorDir, 0755); err != nil {
			utils.PrintError("Failed to create temporary migrator directory: %v", err)
			os.Exit(1)
		}

		// Create main.go for temporary migrator
		mainFile := filepath.Join(tempMigratorDir, "main.go")
		mainContent := fmt.Sprintf(`package main

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/pankajredekar/goosegorm"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	
	// Import migrations to trigger their init() functions
	_ "%s"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: goosegorm <command>")
		fmt.Println("Commands: migrate, rollback, show")
		os.Exit(1)
	}

	command := os.Args[1]
	configPath := "goosegorm.yml"
	
	// Simple config loading (inline to avoid internal package dependency)
	type Config struct {
		DatabaseURL    string
		MigrationTable string
	}
	
	configData, err := os.ReadFile(configPath)
	if err != nil {
		log.Fatalf("goosegorm.yml not found. Run 'goosegorm init' first")
	}
	
	// Simple YAML parsing for database_url and migration_table
	cfg := Config{
		DatabaseURL:    "sqlite://:memory:",
		MigrationTable: "_goosegorm_migrations",
	}
	lines := strings.Split(string(configData), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "database_url:") {
			cfg.DatabaseURL = strings.TrimSpace(strings.TrimPrefix(line, "database_url:"))
		} else if strings.HasPrefix(line, "migration_table:") {
			cfg.MigrationTable = strings.TrimSpace(strings.TrimPrefix(line, "migration_table:"))
		}
	}

	// Connect to database
	db, err := connectDB(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %%v", err)
	}

	// Initialize versioner using public API
	ver := goosegorm.NewVersioner(db, cfg.MigrationTable)
	if err := ver.Initialize(); err != nil {
		log.Fatalf("Failed to initialize version table: %%v", err)
	}

	// Get the global registry (migrations register themselves via init())
	registry := goosegorm.GetGlobalRegistry()

	// Create runner using public API
	run := goosegorm.NewRunner(db, registry, ver)

	switch command {
	case "migrate":
		// Get pending migrations
		pending, err := run.GetPendingMigrations()
		if err != nil {
			log.Fatalf("Failed to get pending migrations: %%v", err)
		}

		if len(pending) == 0 {
			fmt.Println("No pending migrations")
			return
		}

		fmt.Printf("Applying %%d migration(s)...\n", len(pending))

		// Apply migrations
		if err := run.Migrate(); err != nil {
			log.Fatalf("Failed to apply migrations: %%v", err)
		}

		fmt.Printf("Applied %%d migration(s)\n", len(pending))

	case "rollback":
		n := 1
		if len(os.Args) > 2 {
			if _, err := fmt.Sscanf(os.Args[2], "%%d", &n); err != nil {
				log.Fatalf("Invalid number: %%v", err)
			}
		}

		// Get applied count
		appliedCount, err := ver.GetAppliedCount()
		if err != nil {
			log.Fatalf("Failed to get applied count: %%v", err)
		}

		if appliedCount == 0 {
			fmt.Println("No migrations to rollback")
			return
		}

		if int64(n) > appliedCount {
			n = int(appliedCount)
		}

		fmt.Printf("Rolling back %%d migration(s)...\n", n)

		// Rollback
		if err := run.Rollback(n); err != nil {
			log.Fatalf("Failed to rollback: %%v", err)
		}

		fmt.Printf("Rolled back %%d migration(s)\n", n)

	case "show":
		// Get applied migrations
		applied, err := run.GetAppliedMigrations()
		if err != nil {
			log.Fatalf("Failed to get applied migrations: %%v", err)
		}

		// Get pending migrations
		pending, err := run.GetPendingMigrations()
		if err != nil {
			log.Fatalf("Failed to get pending migrations: %%v", err)
		}

		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("Migration Status")
		fmt.Println(strings.Repeat("=", 60))

		if len(applied) > 0 {
			fmt.Println("\n✓ Applied Migrations:")
			for _, m := range applied {
				fmt.Printf("  %%s - %%s\n", m.Version(), m.Name())
			}
		} else {
			fmt.Println("\n✓ Applied Migrations: (none)")
		}

		if len(pending) > 0 {
			fmt.Println("\n○ Pending Migrations:")
			for _, m := range pending {
				fmt.Printf("  %%s - %%s\n", m.Version(), m.Name())
			}
		} else {
			fmt.Println("\n○ Pending Migrations: (none)")
		}

		fmt.Println()

	default:
		fmt.Printf("Unknown command: %%s\n", command)
		fmt.Println("Commands: migrate, rollback, show")
		os.Exit(1)
	}
}

func connectDB(databaseURL string) (*gorm.DB, error) {
	if strings.Contains(databaseURL, "postgres://") || strings.Contains(databaseURL, "postgresql://") {
		return gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	} else if strings.Contains(databaseURL, "sqlite://") {
		path := strings.TrimPrefix(databaseURL, "sqlite://")
		return gorm.Open(sqlite.Open(path), &gorm.Config{})
	}
	return nil, fmt.Errorf("unsupported database URL: %%s", databaseURL)
}
`, migrationsImportPath)

		if err := os.WriteFile(mainFile, []byte(mainContent), 0644); err != nil {
			utils.PrintError("Failed to create temporary migrator: %v", err)
			os.Exit(1)
		}

		// Build the migrator using the project's go.mod
		// Since the migrator is in a subdirectory of the project, we can use the project's go.mod
		utils.PrintInfo("Building migrator...")
		binaryPath := filepath.Join(tempMigratorDir, "goosegorm")
		// Build from project root to use project's go.mod
		buildCmd := exec.Command("go", "build", "-o", binaryPath, mainFile)
		buildCmd.Dir = configDir // Build from project root to use project's go.mod
		buildCmd.Env = os.Environ()
		if output, err := buildCmd.CombinedOutput(); err != nil {
			utils.PrintError("Failed to build migrator: %v\nOutput: %s", err, string(output))
			os.Exit(1)
		}

		// Run the migrator
		utils.PrintInfo("Running migrator...")
		runCmd := exec.Command(binaryPath, "migrate")
		runCmd.Dir = configDir // Run from configDir so it can find goosegorm.yml
		runCmd.Stdout = os.Stdout
		runCmd.Stderr = os.Stderr
		if err := runCmd.Run(); err != nil {
			utils.PrintError("Migration failed: %v", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)
}
