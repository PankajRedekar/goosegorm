package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/pankajredekar/goosegorm/internal/runner"
)

// LoadMigrations loads all migration files from the migrations directory
func LoadMigrations(migrationsDir string) (*runner.Registry, error) {
	registry := runner.NewRegistry()

	// Walk the migrations directory
	err := filepath.Walk(migrationsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Parse the Go file
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		// Look for init functions that register migrations
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "init" {
				continue
			}

			// We can't easily execute the init function, so we'll need to parse the structs
			// and register them manually. For now, we'll use a simpler approach:
			// Look for migration structs and their methods
			_ = extractMigrationsFromFile(file)
			// TODO: Actually load and register migrations
			// This would require compiling and importing the package
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	return registry, nil
}

// extractMigrationsFromFile extracts migration structs from an AST file
func extractMigrationsFromFile(file *ast.File) []string {
	var migrations []string

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			// Check if it has Version() and Name() methods
			if hasMigrationMethods(file, ts.Name.Name) {
				migrations = append(migrations, ts.Name.Name)
			}
		}
	}

	return migrations
}

func hasMigrationMethods(file *ast.File, typeName string) bool {
	hasVersion := false
	hasName := false
	hasUp := false
	hasDown := false

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}

		recvType := fn.Recv.List[0].Type
		var recvTypeName string
		switch rt := recvType.(type) {
		case *ast.Ident:
			recvTypeName = rt.Name
		case *ast.StarExpr:
			if ident, ok := rt.X.(*ast.Ident); ok {
				recvTypeName = ident.Name
			}
		}

		if recvTypeName != typeName {
			continue
		}

		switch fn.Name.Name {
		case "Version":
			hasVersion = true
		case "Name":
			hasName = true
		case "Up":
			hasUp = true
		case "Down":
			hasDown = true
		}
	}

	return hasVersion && hasName && hasUp && hasDown
}

// LoadMigrationsFromPackage loads migrations by importing the package
// This is a runtime approach that requires the package to be compiled
func LoadMigrationsFromPackage(packagePath string) (*runner.Registry, error) {
	registry := runner.NewRegistry()

	// This would require dynamic package loading which is complex in Go
	// For now, we'll use a simpler approach where migrations register themselves
	// at import time via init() functions

	return registry, nil
}

// RegisterMigration is a global registry function that migrations can call
var globalRegistry *runner.Registry

func init() {
	globalRegistry = runner.NewRegistry()
}

// SetGlobalRegistry sets the global registry (for testing)
func SetGlobalRegistry(reg *runner.Registry) {
	globalRegistry = reg
}

// GetGlobalRegistry returns the global registry
func GetGlobalRegistry() *runner.Registry {
	return globalRegistry
}

// RegisterMigration registers a migration in the global registry
func RegisterMigration(m runner.Migration) {
	if globalRegistry == nil {
		globalRegistry = runner.NewRegistry()
	}
	globalRegistry.RegisterMigration(m)
}
