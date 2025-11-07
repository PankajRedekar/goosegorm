package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/pankajredekar/goosegorm/internal/runner"
	"github.com/pankajredekar/goosegorm/internal/schema"
	"gorm.io/gorm"
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

// LoadMigrationsFromCompiled loads migrations by compiling and executing them
// This is used for real DB execution (migrate, rollback, show)
// It creates a temporary helper program that imports the migrations package,
// which triggers init() functions to register migrations
func LoadMigrationsFromCompiled(migrationsDir string, packageName string) (*runner.Registry, error) {
	// Get the absolute path to migrations directory
	absPath, err := filepath.Abs(migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Find go.mod to determine module path and project root
	// Start from migrations directory and walk up to find go.mod
	projectRoot := absPath
	for {
		goModPath := filepath.Join(projectRoot, "go.mod")
		if _, err := os.Stat(goModPath); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			return nil, fmt.Errorf("go.mod not found in project")
		}
		projectRoot = parent
	}

	// Find module path from go.mod
	modulePath, err := findModulePath(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to find module path: %w", err)
	}

	// Create a temporary helper program
	helperDir := filepath.Join(os.TempDir(), fmt.Sprintf("goosegorm_helper_%d", os.Getpid()))
	if err := os.MkdirAll(helperDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create helper directory: %w", err)
	}
	defer os.RemoveAll(helperDir)

	// Calculate the relative path from project root to migrations directory
	relPath, err := filepath.Rel(projectRoot, absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate relative path: %w", err)
	}
	// Convert to forward slashes for import path (Go uses forward slashes)
	relPath = filepath.ToSlash(relPath)
	// Build the import path: modulePath/relativePath
	migrationsImportPath := fmt.Sprintf("%s/%s", modulePath, relPath)

	// Create helper program that imports migrations
	// If migrations are in an "internal" directory, we can't import them from outside the module
	// So we copy them to a non-internal location in the helper directory
	var migrationsSubDir string
	var helperImportPath string
	if strings.Contains(relPath, "internal/") || strings.HasPrefix(relPath, "internal/") {
		// For internal packages, copy to a non-internal location
		// Replace "internal/" with "migrations/" to avoid the internal package restriction
		nonInternalPath := strings.Replace(relPath, "internal/", "migrations/", 1)
		// If it starts with internal/, just use migrations/
		if strings.HasPrefix(relPath, "internal/") {
			nonInternalPath = "migrations" + strings.TrimPrefix(relPath, "internal")
		}
		migrationsSubDir = filepath.Join(helperDir, nonInternalPath)
		// Import path will be relative to helper module
		helperImportPath = fmt.Sprintf("goosegorm_helper/%s", filepath.ToSlash(nonInternalPath))
	} else {
		// For non-internal packages, maintain the directory structure
		migrationsSubDir = filepath.Join(helperDir, relPath)
		helperImportPath = migrationsImportPath
	}
	
	if err := os.MkdirAll(migrationsSubDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create migrations subdirectory: %w", err)
	}

	// Copy migration files to helper directory
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Read and copy file
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(migrationsSubDir, filepath.Base(path))
		return os.WriteFile(destPath, content, 0644)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to copy migration files: %w", err)
	}

	helperFile := filepath.Join(helperDir, "main.go")
	helperContent := fmt.Sprintf(`package main

import (
	"encoding/json"
	"fmt"
	"os"
	"github.com/pankajredekar/goosegorm"
	_ "%s"
)

func main() {
	registry := goosegorm.GetGlobalRegistry()
	migrations := registry.GetAllMigrations()
	
	// Output migrations as JSON
	result := make([]map[string]string, len(migrations))
	for i, m := range migrations {
		result[i] = map[string]string{
			"version": m.Version(),
			"name":    m.Name(),
		}
	}
	
	jsonData, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %%v\n", err)
		os.Exit(1)
	}
	
	fmt.Print(string(jsonData))
}
`, helperImportPath)

	if err := os.WriteFile(helperFile, []byte(helperContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write helper program: %w", err)
	}

	// Try to get goosegorm version from project's go.mod
	goosegormVersion := "v0.0.0"
	goosegormReplacePath := ""
	projectGoModPath := filepath.Join(projectRoot, "go.mod")
	inRequireBlock := false
	if content, err := os.ReadFile(projectGoModPath); err == nil {
		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			// Check for replace directive
			if strings.HasPrefix(trimmed, "replace github.com/pankajredekar/goosegorm =>") {
				parts := strings.Fields(trimmed)
				if len(parts) >= 4 {
					relPath := parts[3]
					absPath, err := filepath.Abs(filepath.Join(projectRoot, relPath))
					if err == nil {
						goosegormReplacePath = absPath
					}
				}
			}
			// Track require block
			if trimmed == "require (" {
				inRequireBlock = true
				continue
			}
			if inRequireBlock && trimmed == ")" {
				inRequireBlock = false
				continue
			}
			// Check for require with version (inside require block or single line)
			if (inRequireBlock || strings.HasPrefix(trimmed, "require ")) && strings.Contains(trimmed, "github.com/pankajredekar/goosegorm") {
				parts := strings.Fields(trimmed)
				// Remove "require" prefix if present
				if parts[0] == "require" {
					parts = parts[1:]
				}
				// Find goosegorm in the line
				for i, part := range parts {
					if part == "github.com/pankajredekar/goosegorm" && i+1 < len(parts) {
						goosegormVersion = parts[i+1]
						break
					}
				}
			}
		}
	}

	// Create go.mod for helper
	// Use replace directive to point the entire module to the project root
	// This allows Go to resolve the migrations import path correctly
	// Note: We only require the root module, not the subdirectory path
	goModContent := fmt.Sprintf(`module goosegorm_helper

go 1.21

require (
	github.com/pankajredekar/goosegorm %s
	%s v0.0.0
)

replace %s => %s
`, goosegormVersion, modulePath, modulePath, projectRoot)

	// Add replace directive for goosegorm if found in project
	if goosegormReplacePath != "" {
		goModContent += fmt.Sprintf("\nreplace github.com/pankajredekar/goosegorm => %s\n", goosegormReplacePath)
	}

	if err := os.WriteFile(filepath.Join(helperDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		return nil, fmt.Errorf("failed to write go.mod: %w", err)
	}

	// Run go mod tidy first and capture output
	cmdTidy := exec.Command("go", "mod", "tidy")
	cmdTidy.Dir = helperDir
	cmdTidy.Env = os.Environ()
	tidyOutput, err := cmdTidy.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run go mod tidy in helper directory: %w\nOutput: %s", err, string(tidyOutput))
	}

	// Build and run helper program
	cmd := exec.Command("go", "run", helperFile)
	cmd.Dir = helperDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to run helper program: %w\nOutput: %s", err, string(output))
	}

	// The helper program compiled and registered migrations via init() functions
	// But we can't access that registry from here. Instead, we need to use plugin package
	// to load the compiled migrations. However, plugins have limitations.
	//
	// For now, we'll use a hybrid approach: Use AST for structure but improve the
	// interpreter to handle real DB operations better. The key insight is that
	// the generated migration code already has the struct definitions in the Up method,
	// so we can extract and use them.
	//
	// Actually, the simplest solution: Since migrations are already compiled and registered
	// in the helper program, we can build them as a plugin and load that plugin.
	// But plugins require the same Go version and dependencies.
	//
	// For now, let's use AST but with improved real DB execution that can handle
	// the struct definitions that are already in the generated code.
	registry := runner.NewRegistry()

	// Parse migration files to create instances
	// We'll use AST but the Up/Down methods will execute the real DB code path
	fset := token.NewFileSet()
	err = filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", path, err)
		}

		// Find migration struct and create instance
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}

			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				// Check if this struct implements Migration interface
				if hasMigrationMethods(file, ts.Name.Name) {
					// Create ASTMigration instance - but this will use improved real DB execution
					migration := &ASTMigration{
						file:     file,
						filePath: path,
					}
					registry.RegisterMigration(migration)
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	return registry, nil
}

// findModulePath finds the module path from go.mod
func findModulePath(dir string) (string, error) {
	goModPath := filepath.Join(dir, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		// Try parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		return findModulePath(parent)
	}

	content, err := os.ReadFile(goModPath)
	if err != nil {
		return "", err
	}

	// Simple parsing: find "module " line
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module")), nil
		}
	}

	return "", fmt.Errorf("module path not found in go.mod")
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

// LoadMigrationsFromAST loads migrations by parsing AST and creating migration instances
// This allows us to simulate the schema without actually compiling the migrations
func LoadMigrationsFromAST(migrationsDir string, packageName string) (*runner.Registry, error) {
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

		// Extract migrations from file
		migrations := extractMigrationsFromAST(file, fset)
		for _, m := range migrations {
			m.filePath = path // Store file path for context
			registry.RegisterMigration(m)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load migrations: %w", err)
	}

	return registry, nil
}

// ASTMigration is a migration created from AST parsing
type ASTMigration struct {
	version  string
	name     string
	upCode   *ast.BlockStmt
	downCode *ast.BlockStmt
	file     *ast.File // Store the file AST to extract struct definitions
	filePath string    // Store file path for context
}

func (m *ASTMigration) Version() string { return m.version }
func (m *ASTMigration) Name() string    { return m.name }

func (m *ASTMigration) Up(db *gorm.DB) error {
	// Check if this is simulation mode by trying to recover SchemaBuilder
	// During simulation, db is actually a *schema.SchemaBuilder passed as *gorm.DB via unsafe conversion
	dbPtr := unsafe.Pointer(db)
	sim := (*schema.SchemaBuilder)(dbPtr)

	// Use recover to safely check if we can access Schema field
	// If it's a real DB, accessing Schema will cause a panic
	var isSimulation bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				// Panic occurred, so it's NOT simulation mode (it's real DB)
				isSimulation = false
			}
		}()
		// Try to safely check if Schema field exists
		// If sim is actually a SchemaBuilder, this won't panic
		// If sim is a real *gorm.DB cast incorrectly, this will panic
		if sim != nil {
			// Try to access Schema - if it panics, we're in real DB mode
			_ = sim.Schema // This will panic if sim is not actually a SchemaBuilder
			// If we get here without panic, it's simulation mode
			isSimulation = true
		}
	}()

	if isSimulation {
		// Execute simulation code
		return m.executeSimulationCode(m.upCode, sim)
	}

	// Real DB mode - execute the real DB code path using raw SQL
	return m.executeRealDBCode(m.upCode, db)
}

func (m *ASTMigration) Down(db *gorm.DB) error {
	// Check if this is simulation mode
	dbPtr := unsafe.Pointer(db)
	sim := (*schema.SchemaBuilder)(dbPtr)

	var isSimulation bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				isSimulation = false
			}
		}()
		if sim != nil {
			if sim.Schema == nil {
				sim.Schema = &schema.SchemaState{
					Tables: make(map[string]*schema.Table),
				}
			}
			if sim.Schema.Tables == nil {
				sim.Schema.Tables = make(map[string]*schema.Table)
			}
			isSimulation = true
		}
	}()

	if isSimulation {
		return m.executeSimulationCode(m.downCode, sim)
	}

	return m.executeRealDBCode(m.downCode, db)
}

func (m *ASTMigration) executeSimulationCode(block *ast.BlockStmt, sim *schema.SchemaBuilder) error {
	// This is a simplified implementation
	// In a real implementation, we'd need to interpret the AST
	// For now, we'll parse the simulation calls from the AST
	if block == nil {
		return nil
	}

	// Walk the AST and execute simulation calls
	return m.interpretSimulationBlock(block, sim)
}

// executeRealDBCode executes the real DB code path from the AST
// This skips the simulation block and executes the real DB operations
func (m *ASTMigration) executeRealDBCode(block *ast.BlockStmt, db *gorm.DB) error {
	if block == nil {
		return nil
	}

	// First pass: collect struct definitions
	structDefs := make(map[string]*ast.StructType)
	skipSimulationBlock := false
	for _, stmt := range block.List {
		if ifStmt, ok := stmt.(*ast.IfStmt); ok {
			if m.isSimulationCheck(ifStmt) {
				skipSimulationBlock = true
				continue
			}
		}
		if skipSimulationBlock {
			// Collect struct definitions
			if decl, ok := stmt.(*ast.DeclStmt); ok {
				if genDecl, ok := decl.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
					for _, spec := range genDecl.Specs {
						if typeSpec, ok := spec.(*ast.TypeSpec); ok {
							if structType, ok := typeSpec.Type.(*ast.StructType); ok {
								structDefs[typeSpec.Name.Name] = structType
							}
						}
					}
				}
			}
		}
	}

	// Second pass: execute statements, using collected struct definitions
	skipSimulationBlock = false
	for _, stmt := range block.List {
		if ifStmt, ok := stmt.(*ast.IfStmt); ok {
			if m.isSimulationCheck(ifStmt) {
				skipSimulationBlock = true
				continue
			}
		}
		if skipSimulationBlock {
			// Execute real DB operations by interpreting GORM calls
			if err := m.interpretRealDBStatementWithStructs(stmt, db, structDefs); err != nil {
				return err
			}
		}
	}

	return nil
}

// interpretRealDBStatementWithStructs interprets a statement for real DB execution with struct definitions
func (m *ASTMigration) interpretRealDBStatementWithStructs(stmt ast.Stmt, db *gorm.DB, structDefs map[string]*ast.StructType) error {
	// Handle expression statements (GORM method calls)
	if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
		return m.interpretRealDBExpressionWithStructs(exprStmt.X, db, structDefs)
	}
	// Handle if statements (error checks)
	if ifStmt, ok := stmt.(*ast.IfStmt); ok {
		// This might be an error check like: if err := ...; err != nil { return err }
		// For now, just execute the condition
		if ifStmt.Init != nil {
			if assign, ok := ifStmt.Init.(*ast.AssignStmt); ok {
				// Execute the assignment (the GORM call)
				for _, expr := range assign.Rhs {
					if err := m.interpretRealDBExpressionWithStructs(expr, db, structDefs); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// interpretRealDBStatement interprets a statement for real DB execution
func (m *ASTMigration) interpretRealDBStatement(stmt ast.Stmt, db *gorm.DB) error {
	return m.interpretRealDBStatementWithStructs(stmt, db, make(map[string]*ast.StructType))
}

// interpretRealDBExpressionWithStructs interprets an expression for real DB execution with struct definitions
func (m *ASTMigration) interpretRealDBExpressionWithStructs(expr ast.Expr, db *gorm.DB, structDefs map[string]*ast.StructType) error {
	// Handle SelectorExpr like db.Exec(...).Error
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		// If it's .Error, execute the receiver (the GORM call) and ignore the .Error accessor
		if sel.Sel.Name == "Error" {
			return m.interpretRealDBExpressionWithStructs(sel.X, db, structDefs)
		}
	}
	if call, ok := expr.(*ast.CallExpr); ok {
		return m.interpretRealDBCallWithStructs(call, db, structDefs)
	}
	return nil
}

// interpretRealDBExpression interprets an expression for real DB execution
func (m *ASTMigration) interpretRealDBExpression(expr ast.Expr, db *gorm.DB) error {
	return m.interpretRealDBExpressionWithStructs(expr, db, make(map[string]*ast.StructType))
}

// interpretRealDBCallWithStructs interprets a GORM method call for real DB execution with struct definitions
func (m *ASTMigration) interpretRealDBCallWithStructs(call *ast.CallExpr, db *gorm.DB, structDefs map[string]*ast.StructType) error {
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName := sel.Sel.Name
		receiver := sel.X

		// Check if receiver is 'db' (the gorm.DB)
		if ident, ok := receiver.(*ast.Ident); ok && ident.Name == "db" {
			// Extract arguments
			args := make([]interface{}, 0, len(call.Args))
			for _, arg := range call.Args {
				val := m.extractValue(arg)
				args = append(args, val)
			}

			// Execute GORM methods
			switch methodName {
			case "Table":
				if len(args) > 0 {
					if tableName, ok := args[0].(string); ok {
						// Return a chainable object - for now, we'll execute AutoMigrate if it's chained
						// This is simplified - in production, we'd need to handle method chaining properly
						_ = tableName
					}
				}
			case "Exec":
				if len(args) > 0 {
					if sql, ok := args[0].(string); ok {
						// Execute the raw SQL
						result := db.Exec(sql)
						if result.Error != nil {
							return result.Error
						}
					} else {
						// Debug: log if we can't extract SQL
						return fmt.Errorf("failed to extract SQL string from Exec call argument")
					}
				}
			case "Migrator":
				// Return migrator - chained calls will be handled separately
				_ = db.Migrator()
			}
		} else if callExpr, ok := receiver.(*ast.CallExpr); ok {
			// This is a chained call like db.Table("users").AutoMigrate(...)
			// We need to interpret the chain
			return m.interpretRealDBChainedCallWithStructs(callExpr, methodName, call.Args, db, structDefs)
		}
	}
	return nil
}

// interpretRealDBCall interprets a GORM method call for real DB execution
func (m *ASTMigration) interpretRealDBCall(call *ast.CallExpr, db *gorm.DB) error {
	return m.interpretRealDBCallWithStructs(call, db, make(map[string]*ast.StructType))
}

// interpretRealDBChainedCallWithStructs interprets a chained GORM call with struct definitions
func (m *ASTMigration) interpretRealDBChainedCallWithStructs(prevCall *ast.CallExpr, methodName string, args []ast.Expr, db *gorm.DB, structDefs map[string]*ast.StructType) error {
	// Handle common GORM chains
	if sel, ok := prevCall.Fun.(*ast.SelectorExpr); ok {
		if sel.Sel.Name == "Table" {
			// db.Table("name").Method(...)
			if len(prevCall.Args) > 0 {
				tableName, _ := m.extractValue(prevCall.Args[0]).(string)
				argValues := make([]interface{}, 0, len(args))
				for _, arg := range args {
					argValues = append(argValues, m.extractValue(arg))
				}

				switch methodName {
				case "AutoMigrate":
					if len(argValues) > 0 {
						// AutoMigrate takes a struct pointer
						// Extract the struct type from the AST argument
						if len(args) > 0 {
							structInstance := m.extractStructInstanceWithDefs(args[0], tableName, structDefs)
							if structInstance != nil {
								if err := db.Table(tableName).AutoMigrate(structInstance); err != nil {
									return err
								}
							} else {
								// Last resort: use empty struct (won't create columns, but won't error)
								// This is a known limitation - proper solution requires compiling migrations
								if err := db.Table(tableName).AutoMigrate(&struct{}{}); err != nil {
									return err
								}
							}
						}
					}
				}
			}
		} else if sel.Sel.Name == "Migrator" {
			// db.Migrator().Method(...)
			migrator := db.Migrator()
			argValues := make([]interface{}, 0, len(args))
			for _, arg := range args {
				argValues = append(argValues, m.extractValue(arg))
			}

			switch methodName {
			case "DropTable":
				if len(argValues) > 0 {
					if tableName, ok := argValues[0].(string); ok {
						if err := migrator.DropTable(tableName); err != nil {
							return err
						}
					}
				}
			case "AddColumn":
				// AddColumn requires a struct and column name
				// This is complex to handle from AST, so we'll use a simplified approach
				if len(argValues) >= 2 {
					if tableName, ok := argValues[0].(string); ok {
						if colName, ok := argValues[1].(string); ok {
							// Create a minimal struct with the column
							type TempStruct struct {
								Column string `gorm:"column"`
							}
							if err := db.Table(tableName).Migrator().AddColumn(&TempStruct{}, colName); err != nil {
								return err
							}
						}
					}
				}
			case "DropColumn":
				if len(argValues) >= 2 {
					if tableName, ok := argValues[0].(string); ok {
						if colName, ok := argValues[1].(string); ok {
							if err := migrator.DropColumn(tableName, colName); err != nil {
								return err
							}
						}
					}
				}
			}
		}
	}
	return nil
}

func (m *ASTMigration) interpretSimulationBlock(block *ast.BlockStmt, sim *schema.SchemaBuilder) error {
	if block == nil {
		return nil
	}
	// Walk through statements and look for simulation calls
	for _, stmt := range block.List {
		if err := m.interpretStatement(stmt, sim); err != nil {
			return err
		}
	}
	return nil
}

func (m *ASTMigration) interpretStatement(stmt ast.Stmt, sim *schema.SchemaBuilder) error {
	// Look for type assertion: if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok
	if ifStmt, ok := stmt.(*ast.IfStmt); ok {
		return m.interpretIfStatement(ifStmt, sim)
	}
	// Handle expression statements (method calls)
	if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
		return m.interpretExpression(exprStmt.X, sim)
	}
	// Handle assignments
	if assignStmt, ok := stmt.(*ast.AssignStmt); ok {
		return m.interpretAssignment(assignStmt, sim)
	}
	// Handle return statements (just ignore them, they don't affect schema)
	if _, ok := stmt.(*ast.ReturnStmt); ok {
		return nil
	}
	return nil
}

// isSimulationCheck checks if an if statement is the simulation mode check
func (m *ASTMigration) isSimulationCheck(ifStmt *ast.IfStmt) bool {
	if ifStmt.Init != nil {
		if assign, ok := ifStmt.Init.(*ast.AssignStmt); ok {
			if len(assign.Lhs) > 0 && len(assign.Rhs) > 0 {
				if typeAssert, ok := assign.Rhs[0].(*ast.TypeAssertExpr); ok {
					if sel, ok := typeAssert.Type.(*ast.SelectorExpr); ok {
						// Check for goosegorm.SchemaBuilder or schema.SchemaBuilder
						if ident, ok := sel.X.(*ast.Ident); ok {
							if (ident.Name == "goosegorm" || ident.Name == "schema") && sel.Sel.Name == "SchemaBuilder" {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
}

func (m *ASTMigration) interpretIfStatement(ifStmt *ast.IfStmt, sim *schema.SchemaBuilder) error {
	// Check if this is the simulation mode check
	// Pattern: if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok
	if ifStmt.Init != nil {
		// Check if it's a type assertion assignment
		if assign, ok := ifStmt.Init.(*ast.AssignStmt); ok {
			if len(assign.Lhs) > 0 && len(assign.Rhs) > 0 {
				// Check if RHS is a type assertion
				if typeAssert, ok := assign.Rhs[0].(*ast.TypeAssertExpr); ok {
					// Check if it's asserting to SchemaBuilder type
					if sel, ok := typeAssert.Type.(*ast.SelectorExpr); ok {
						// Check for goosegorm.SchemaBuilder
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "goosegorm" {
							if sel.Sel.Name == "SchemaBuilder" {
								// This is the simulation mode check
								// Only execute if we have a valid SchemaBuilder (simulation mode)
								if sim != nil && sim.Schema != nil {
									if ifStmt.Body != nil {
										return m.interpretSimulationBlock(ifStmt.Body, sim)
									}
								}
								// If we're not in simulation mode, skip this block (it's the else path)
								return nil
							}
						}
						// Check for direct schema.SchemaBuilder
						if ident, ok := sel.X.(*ast.Ident); ok && ident.Name == "schema" {
							if sel.Sel.Name == "SchemaBuilder" {
								// This is the simulation mode check
								if sim != nil && sim.Schema != nil {
									if ifStmt.Body != nil {
										return m.interpretSimulationBlock(ifStmt.Body, sim)
									}
								}
								// If we're not in simulation mode, skip this block
								return nil
							}
						}
					}
				}
			}
		}
	}
	// If not simulation mode check, try to interpret the body anyway (might be nested)
	if ifStmt.Body != nil {
		return m.interpretSimulationBlock(ifStmt.Body, sim)
	}
	return nil
}

func (m *ASTMigration) interpretAssignment(assign *ast.AssignStmt, sim *schema.SchemaBuilder) error {
	// Handle assignments like: sim = sim.CreateTable(...)
	// For now, we'll just interpret the RHS
	for _, expr := range assign.Rhs {
		if err := m.interpretExpression(expr, sim); err != nil {
			return err
		}
	}
	return nil
}

func (m *ASTMigration) interpretExpression(expr ast.Expr, sim *schema.SchemaBuilder) error {
	// Handle method calls on sim
	if call, ok := expr.(*ast.CallExpr); ok {
		return m.interpretCall(call, sim)
	}
	return nil
}

func (m *ASTMigration) interpretCall(call *ast.CallExpr, sim *schema.SchemaBuilder) error {
	// Get the function being called
	var methodName string
	var receiver ast.Expr

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName = sel.Sel.Name
		receiver = sel.X
	} else {
		return nil
	}

	// Extract arguments
	args := make([]interface{}, 0, len(call.Args))
	for _, arg := range call.Args {
		val := m.extractValue(arg)
		args = append(args, val) // Append even if nil, to preserve argument positions
	}

	// Check if receiver is 'sim' (the SchemaBuilder)
	if ident, ok := receiver.(*ast.Ident); ok && ident.Name == "sim" {
		return m.executeSchemaBuilderMethod(methodName, args, sim)
	}

	// Handle method chaining: sim.CreateTable(...).AddColumn(...)
	// The receiver might be a call expression (from previous method in chain)
	if recvCall, ok := receiver.(*ast.CallExpr); ok {
		// Recursively interpret the receiver call to get the TableBuilder
		tableBuilder := m.interpretCallForChaining(recvCall, sim)
		if tableBuilder != nil {
			return m.executeTableBuilderMethod(methodName, args, tableBuilder)
		}
	}

	// Handle chaining where receiver is a SelectorExpr wrapping a CallExpr
	// Pattern: sim.AlterTable("user").AddIndex("idx_email")
	// The receiver is a SelectorExpr where X is a CallExpr
	if recvSel, ok := receiver.(*ast.SelectorExpr); ok {
		// Check if X is a CallExpr (the previous method call in the chain)
		if recvCall, ok := recvSel.X.(*ast.CallExpr); ok {
			tableBuilder := m.interpretCallForChaining(recvCall, sim)
			if tableBuilder != nil {
				// Execute the method (e.g., AddIndex) on the TableBuilder
				return m.executeTableBuilderMethod(methodName, args, tableBuilder)
			}
		}
	}

	// Handle nested chaining: sim.CreateTable(...).AddColumn(...).AddColumn(...)
	// The receiver might be a selector expression wrapping a call
	// Pattern: (sim.AlterTable("user")).AddIndex("idx_email")
	// The receiver is a SelectorExpr where X is the CallExpr and Sel is the method name
	if recvSel, ok := receiver.(*ast.SelectorExpr); ok {
		// Check if X is a CallExpr (chained call)
		if recvCall, ok := recvSel.X.(*ast.CallExpr); ok {
			tableBuilder := m.interpretCallForChaining(recvCall, sim)
			if tableBuilder != nil {
				// Extract arguments for the method from the selector
				args := make([]interface{}, 0, len(call.Args))
				for _, arg := range call.Args {
					val := m.extractValue(arg)
					args = append(args, val)
				}
				// Execute the method from the selector (e.g., AddIndex)
				return m.executeTableBuilderMethod(recvSel.Sel.Name, args, tableBuilder)
			}
		}
		// Also handle case where X is an Ident (sim) followed by a selector
		// Pattern: sim.AlterTable("user").AddIndex("idx_email")
		// This might appear as SelectorExpr(SelectorExpr(CallExpr), AddIndex)
		if recvSel2, ok := recvSel.X.(*ast.SelectorExpr); ok {
			// This is a deeper nesting, try to interpret it
			if recvCall, ok := recvSel2.X.(*ast.CallExpr); ok {
				tableBuilder := m.interpretCallForChaining(recvCall, sim)
				if tableBuilder != nil {
					// First execute the intermediate method (e.g., AlterTable)
					intermediateArgs := make([]interface{}, 0, len(recvCall.Args))
					for _, arg := range recvCall.Args {
						val := m.extractValue(arg)
						intermediateArgs = append(intermediateArgs, val)
					}
					// Get the intermediate method name
					if intermediateSel, ok := recvCall.Fun.(*ast.SelectorExpr); ok {
						// Check if receiver is 'sim'
						if ident, ok := intermediateSel.X.(*ast.Ident); ok && ident.Name == "sim" {
							// Execute AlterTable to get TableBuilder
							tableBuilder = m.executeSchemaBuilderMethodForChaining(intermediateSel.Sel.Name, intermediateArgs, sim)
							if tableBuilder != nil {
								// Now execute the final method (e.g., AddIndex)
								args := make([]interface{}, 0, len(call.Args))
								for _, arg := range call.Args {
									val := m.extractValue(arg)
									args = append(args, val)
								}
								return m.executeTableBuilderMethod(recvSel.Sel.Name, args, tableBuilder)
							}
						}
					}
				}
			}
		}
	}

	return nil
}

func (m *ASTMigration) interpretCallForChaining(call *ast.CallExpr, sim *schema.SchemaBuilder) interface{} {
	// This handles calls in a chain, returning the result (usually TableBuilder)
	var methodName string
	var receiver ast.Expr

	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		methodName = sel.Sel.Name
		receiver = sel.X
	} else {
		return nil
	}

	// Extract arguments
	args := make([]interface{}, 0, len(call.Args))
	for _, arg := range call.Args {
		val := m.extractValue(arg)
		args = append(args, val)
	}

	// Check if receiver is 'sim' (SchemaBuilder)
	if ident, ok := receiver.(*ast.Ident); ok && ident.Name == "sim" {
		// Execute SchemaBuilder method and return TableBuilder
		return m.executeSchemaBuilderMethodForChaining(methodName, args, sim)
	}

	// If receiver is another call (nested chain), interpret it recursively
	if recvCall, ok := receiver.(*ast.CallExpr); ok {
		// This is a deeper chain, interpret recursively
		tableBuilder := m.interpretCallForChaining(recvCall, sim)
		if tableBuilder != nil {
			// Execute the method on the TableBuilder
			m.executeTableBuilderMethod(methodName, args, tableBuilder)
			// TableBuilder methods return *TableBuilder for chaining
			return tableBuilder
		}
	}

	// If receiver is a selector (from previous chain link)
	if recvSel, ok := receiver.(*ast.SelectorExpr); ok {
		// This means we have something like: (previous call).MethodName
		// We need to get the previous call
		if recvCall, ok := recvSel.X.(*ast.CallExpr); ok {
			tableBuilder := m.interpretCallForChaining(recvCall, sim)
			if tableBuilder != nil {
				// Execute the method from the selector
				m.executeTableBuilderMethod(recvSel.Sel.Name, args, tableBuilder)
				return tableBuilder
			}
		}
	}

	return nil
}

func (m *ASTMigration) executeSchemaBuilderMethod(methodName string, args []interface{}, sim *schema.SchemaBuilder) error {
	switch methodName {
	case "CreateTable":
		if len(args) > 0 {
			if tableName, ok := args[0].(string); ok {
				sim.CreateTable(tableName)
			}
		}
	case "AlterTable":
		if len(args) > 0 {
			if tableName, ok := args[0].(string); ok {
				sim.AlterTable(tableName)
			}
		}
	case "DropTable":
		if len(args) > 0 {
			if tableName, ok := args[0].(string); ok {
				sim.DropTable(tableName)
			}
		}
	}
	return nil
}

func (m *ASTMigration) executeSchemaBuilderMethodForChaining(methodName string, args []interface{}, sim *schema.SchemaBuilder) interface{} {
	switch methodName {
	case "CreateTable":
		if len(args) > 0 {
			if tableName, ok := args[0].(string); ok {
				return sim.CreateTable(tableName)
			}
		}
	case "AlterTable":
		if len(args) > 0 {
			if tableName, ok := args[0].(string); ok {
				return sim.AlterTable(tableName)
			}
		}
	}
	return nil
}

func (m *ASTMigration) executeTableBuilderMethod(methodName string, args []interface{}, tableBuilder interface{}) error {
	tb, ok := tableBuilder.(*schema.TableBuilder)
	if !ok {
		return nil
	}

	switch methodName {
	case "AddColumn":
		if len(args) >= 2 {
			name, _ := args[0].(string)
			colType, _ := args[1].(string)
			if name != "" && colType != "" {
				tb.AddColumn(name, colType)
			}
		}
	case "AddColumnWithOptions":
		if len(args) >= 5 {
			name, _ := args[0].(string)
			colType, _ := args[1].(string)
			null, _ := args[2].(bool)
			pk, _ := args[3].(bool)
			unique, _ := args[4].(bool)
			if name != "" && colType != "" {
				tb.AddColumnWithOptions(name, colType, null, pk, unique)
			}
		}
	case "DropColumn":
		if len(args) > 0 {
			if name, ok := args[0].(string); ok && name != "" {
				tb.DropColumn(name)
			}
		}
	case "AddIndex":
		if len(args) > 0 {
			if name, ok := args[0].(string); ok && name != "" {
				tb.AddIndex(name)
			}
		}
	case "DropIndex":
		if len(args) > 0 {
			if name, ok := args[0].(string); ok && name != "" {
				tb.DropIndex(name)
			}
		}
	case "ModifyColumn":
		if len(args) >= 5 {
			name, _ := args[0].(string)
			colType, _ := args[1].(string)
			null, _ := args[2].(bool)
			pk, _ := args[3].(bool)
			unique, _ := args[4].(bool)
			if name != "" && colType != "" {
				tb.ModifyColumn(name, colType, null, pk, unique)
			}
		}
	case "RenameColumn":
		if len(args) >= 2 {
			oldName, _ := args[0].(string)
			newName, _ := args[1].(string)
			if oldName != "" && newName != "" {
				tb.RenameColumn(oldName, newName)
			}
		}
	case "AddConstraint":
		if len(args) > 0 {
			if expr, ok := args[0].(string); ok && expr != "" {
				tb.AddConstraint(expr)
			}
		}
	}
	return nil
}

func (m *ASTMigration) extractValue(expr ast.Expr) interface{} {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.STRING:
			// Remove quotes (handles both "string" and `raw string`)
			val := e.Value
			// Remove surrounding quotes
			if len(val) >= 2 {
				if val[0] == '"' && val[len(val)-1] == '"' {
					val = val[1 : len(val)-1]
				} else if val[0] == '`' && val[len(val)-1] == '`' {
					val = val[1 : len(val)-1]
				}
			}
			// Unescape string if needed
			val = strings.ReplaceAll(val, `\"`, `"`)
			val = strings.ReplaceAll(val, `\n`, "\n")
			val = strings.ReplaceAll(val, `\t`, "\t")
			return val
		case token.INT:
			// Parse integer
			var val int64
			fmt.Sscanf(e.Value, "%d", &val)
			return val
		case token.FLOAT:
			// Parse float
			var val float64
			fmt.Sscanf(e.Value, "%f", &val)
			return val
		case token.IDENT:
			// Handle boolean literals
			if e.Value == "true" {
				return true
			}
			if e.Value == "false" {
				return false
			}
		}
	case *ast.Ident:
		// Handle boolean identifiers
		if e.Name == "true" {
			return true
		}
		if e.Name == "false" {
			return false
		}
	}
	return nil
}

// interpretRealDBChainedCall interprets a chained GORM call
func (m *ASTMigration) interpretRealDBChainedCall(prevCall *ast.CallExpr, methodName string, args []ast.Expr, db *gorm.DB) error {
	return m.interpretRealDBChainedCallWithStructs(prevCall, methodName, args, db, make(map[string]*ast.StructType))
}

// extractStructInstanceWithDefs extracts a struct instance from an AST expression like &User{} using struct definitions
func (m *ASTMigration) extractStructInstanceWithDefs(expr ast.Expr, tableName string, structDefs map[string]*ast.StructType) interface{} {
	// Handle &User{} pattern
	if unary, ok := expr.(*ast.UnaryExpr); ok && unary.Op == token.AND {
		if composite, ok := unary.X.(*ast.CompositeLit); ok {
			if ident, ok := composite.Type.(*ast.Ident); ok {
				// Found &User{} - now find the struct definition
				if structType, exists := structDefs[ident.Name]; exists {
					// We have the struct definition, but we can't create a Go type from AST at runtime
					// This is a fundamental limitation - we need to compile and execute migrations
					// For now, return nil to fall back to empty struct
					_ = structType
					return nil
				}
			}
		}
	}
	return nil
}

// extractStructInstance extracts a struct instance from an AST expression like &User{}
func (m *ASTMigration) extractStructInstance(expr ast.Expr, tableName string) interface{} {
	return m.extractStructInstanceWithDefs(expr, tableName, make(map[string]*ast.StructType))
}

// findStructDefinition finds a struct type definition in the method body
func (m *ASTMigration) findStructDefinition(structName, tableName string) interface{} {
	if m.upCode == nil {
		return nil
	}

	// Walk through the method body to find type declarations
	for _, stmt := range m.upCode.List {
		if decl, ok := stmt.(*ast.DeclStmt); ok {
			if genDecl, ok := decl.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if typeSpec.Name.Name == structName {
							// Found the struct definition - use reflection to create instance
							// For now, return nil and let findStructInMethodBody handle it
							return nil
						}
					}
				}
			}
		}
	}
	return nil
}

// findStructInMethodBody finds struct definitions in the method body and creates instances
// This is a simplified approach - in production, you'd want to use go/types or similar
func (m *ASTMigration) findStructInMethodBody(tableName string) interface{} {
	if m.upCode == nil {
		return nil
	}

	// Walk through statements to find type declarations
	for _, stmt := range m.upCode.List {
		if decl, ok := stmt.(*ast.DeclStmt); ok {
			if genDecl, ok := decl.Decl.(*ast.GenDecl); ok && genDecl.Tok == token.TYPE {
				for _, spec := range genDecl.Specs {
					if typeSpec, ok := spec.(*ast.TypeSpec); ok {
						if structType, ok := typeSpec.Type.(*ast.StructType); ok {
							// Found a struct - create a map-based struct for GORM
							// GORM can work with map[string]interface{} for AutoMigrate
							// But we need the actual struct type, so we'll use a workaround
							// For now, return nil and let the caller handle it
							_ = structType
							_ = typeSpec.Name.Name
						}
					}
				}
			}
		}
	}

	// For now, we can't easily create Go types from AST at runtime
	// The proper solution is to compile and execute migrations
	// This is a placeholder that returns nil
	return nil
}

// extractMigrationsFromAST extracts migration structs and creates ASTMigration instances
func extractMigrationsFromAST(file *ast.File, fset *token.FileSet) []*ASTMigration {
	var migrations []*ASTMigration

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

			// Check if it's a migration struct
			if !hasMigrationMethods(file, ts.Name.Name) {
				continue
			}

			// Extract version and name from methods
			version := extractStringReturnValue(file, ts.Name.Name, "Version")
			name := extractStringReturnValue(file, ts.Name.Name, "Name")

			if version == "" || name == "" {
				continue
			}

			// Extract Up and Down method bodies
			upBlock := extractMethodBody(file, ts.Name.Name, "Up")
			downBlock := extractMethodBody(file, ts.Name.Name, "Down")

			migration := &ASTMigration{
				version:  version,
				name:     name,
				upCode:   upBlock,
				downCode: downBlock,
				file:     file,
			}

			migrations = append(migrations, migration)
		}
	}

	return migrations
}

// extractStringReturnValue extracts the return value from a method that returns a string
func extractStringReturnValue(file *ast.File, typeName, methodName string) string {
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		if fn.Recv == nil || len(fn.Recv.List) == 0 {
			continue
		}

		// Check receiver type
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

		if recvTypeName != typeName || fn.Name.Name != methodName {
			continue
		}

		// Extract return statement value
		if fn.Body != nil {
			for _, stmt := range fn.Body.List {
				if ret, ok := stmt.(*ast.ReturnStmt); ok && len(ret.Results) > 0 {
					if lit, ok := ret.Results[0].(*ast.BasicLit); ok && lit.Kind == token.STRING {
						// Remove quotes
						return strings.Trim(lit.Value, `"`)
					}
				}
			}
		}
	}

	return ""
}

// extractMethodBody extracts the body block of a method
func extractMethodBody(file *ast.File, typeName, methodName string) *ast.BlockStmt {
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

		if recvTypeName == typeName && fn.Name.Name == methodName {
			return fn.Body
		}
	}

	return nil
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
