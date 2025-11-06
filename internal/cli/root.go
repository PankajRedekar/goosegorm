package cli

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "goosegorm",
	Short: "Django-style migration framework for GORM",
	Long:  "GooseGORM is a Go-based migration framework for GORM with in-memory schema simulation",
}

// Execute runs the CLI
func Execute() error {
	return rootCmd.Execute()
}
