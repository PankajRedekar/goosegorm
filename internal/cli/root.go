package cli

import (
	"fmt"

	"github.com/pankajredekar/goosegorm"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "goosegorm",
	Short:   "Django-style migration framework for GORM",
	Long:    "GooseGORM is a Go-based migration framework for GORM with in-memory schema simulation",
	Version: goosegorm.GetVersion(),
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Long:  "Print the version number of GooseGORM",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("goosegorm version %s\n", goosegorm.GetVersion())
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the CLI
func Execute() error {
	return rootCmd.Execute()
}
