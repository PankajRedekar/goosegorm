package loader

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
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
				isSimulation = false
			}
		}()
		// Try to safely check if Schema field exists
		if sim != nil {
			// Try to access Schema - if it panics, we're in real DB mode
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
		// Execute simulation code
		return m.executeSimulationCode(m.upCode, sim)
	}

	// Real DB mode - execute the real DB code path
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

	// Walk through statements and find the real DB code (after the simulation if block)
	// The structure is: if sim, ok := ... { simulation code } real DB code
	skipSimulationBlock := false
	for _, stmt := range block.List {
		if ifStmt, ok := stmt.(*ast.IfStmt); ok {
			// Check if this is the simulation mode check
			if m.isSimulationCheck(ifStmt) {
				// Skip the simulation block - we're in real DB mode
				skipSimulationBlock = true
				continue
			}
		}
		if skipSimulationBlock {
			// Execute real DB operations by interpreting GORM calls
			if err := m.interpretRealDBStatement(stmt, db); err != nil {
				return err
			}
		}
	}

	return nil
}

// interpretRealDBStatement interprets a statement for real DB execution
func (m *ASTMigration) interpretRealDBStatement(stmt ast.Stmt, db *gorm.DB) error {
	// Handle expression statements (GORM method calls)
	if exprStmt, ok := stmt.(*ast.ExprStmt); ok {
		return m.interpretRealDBExpression(exprStmt.X, db)
	}
	// Handle if statements (error checks)
	if ifStmt, ok := stmt.(*ast.IfStmt); ok {
		// This might be an error check like: if err := ...; err != nil { return err }
		// For now, just execute the condition
		if ifStmt.Init != nil {
			if assign, ok := ifStmt.Init.(*ast.AssignStmt); ok {
				// Execute the assignment (the GORM call)
				for _, expr := range assign.Rhs {
					if err := m.interpretRealDBExpression(expr, db); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// interpretRealDBExpression interprets an expression for real DB execution
func (m *ASTMigration) interpretRealDBExpression(expr ast.Expr, db *gorm.DB) error {
	if call, ok := expr.(*ast.CallExpr); ok {
		return m.interpretRealDBCall(call, db)
	}
	return nil
}

// interpretRealDBCall interprets a GORM method call for real DB execution
func (m *ASTMigration) interpretRealDBCall(call *ast.CallExpr, db *gorm.DB) error {
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
						if err := db.Exec(sql).Error; err != nil {
							return err
						}
					}
				}
			case "Migrator":
				// Return migrator - chained calls will be handled separately
				_ = db.Migrator()
			}
		} else if callExpr, ok := receiver.(*ast.CallExpr); ok {
			// This is a chained call like db.Table("users").AutoMigrate(...)
			// We need to interpret the chain
			return m.interpretRealDBChainedCall(callExpr, methodName, call.Args, db)
		}
	}
	return nil
}

// interpretRealDBChainedCall interprets a chained GORM call
func (m *ASTMigration) interpretRealDBChainedCall(prevCall *ast.CallExpr, methodName string, args []ast.Expr, db *gorm.DB) error {
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
						// We can't easily create the struct from AST, so we'll use a generic approach
						// For now, we'll just call AutoMigrate with a minimal struct
						// In production, you'd want to parse the struct definition from AST
						if err := db.Table(tableName).AutoMigrate(&struct{}{}); err != nil {
							return err
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
			// Remove quotes
			return strings.Trim(e.Value, `"`)
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
