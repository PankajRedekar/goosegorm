package utils

import (
	"fmt"
	"os"
)

// Color output helpers
const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
	ColorCyan   = "\033[36m"
)

// PrintSuccess prints a success message
func PrintSuccess(msg string, args ...interface{}) {
	fmt.Printf(ColorGreen+"✓ "+msg+ColorReset+"\n", args...)
}

// PrintError prints an error message
func PrintError(msg string, args ...interface{}) {
	fmt.Printf(ColorRed+"✗ "+msg+ColorReset+"\n", args...)
}

// PrintInfo prints an info message
func PrintInfo(msg string, args ...interface{}) {
	fmt.Printf(ColorCyan+"ℹ "+msg+ColorReset+"\n", args...)
}

// PrintWarning prints a warning message
func PrintWarning(msg string, args ...interface{}) {
	fmt.Printf(ColorYellow+"⚠ "+msg+ColorReset+"\n", args...)
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// DirExists checks if a directory exists
func DirExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
