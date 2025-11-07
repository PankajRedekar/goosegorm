package modelreflect

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
)

// ParsedModel represents a parsed model struct
type ParsedModel struct {
	Name       string
	Package    string
	Managed    bool
	Fields     []Field
	File       string
	StructNode *ast.StructType
	TableName  string // Custom table name from TableName() method, empty if not found
}

// Field represents a struct field
type Field struct {
	Name     string
	Type     string
	GormTag  string
	GooseTag string
	Indexes  []IndexInfo // Named indexes from GORM tags
}

// IndexInfo represents index information from GORM tags
type IndexInfo struct {
	Name     string
	Unique   bool
	Priority int
}

// ParseModelsFromDir parses all Go files in the directory and extracts model structs
func ParseModelsFromDir(dir string, ignoreModels []string) ([]ParsedModel, error) {
	ignoreMap := make(map[string]bool)
	for _, name := range ignoreModels {
		ignoreMap[name] = true
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("failed to parse directory: %w", err)
	}

	var models []ParsedModel

	for pkgName, pkg := range pkgs {
		for fileName, file := range pkg.Files {
			fileModels := parseFileModels(fset, file, pkgName, fileName, ignoreMap)
			models = append(models, fileModels...)
		}
	}

	return models, nil
}

func parseFileModels(fset *token.FileSet, file *ast.File, pkgName, fileName string, ignoreMap map[string]bool) []ParsedModel {
	var models []ParsedModel

	// First pass: collect all struct types
	structTypes := make(map[string]*ast.StructType)
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

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}

			structTypes[ts.Name.Name] = st
		}
	}

	// Second pass: find TableName() methods and associate them with structs
	tableNameMethods := findTableNameMethods(file, structTypes)

	// Third pass: create ParsedModel instances
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}

		// Check for goosegorm tag in comments
		managed := true
		if gd.Doc != nil {
			for _, c := range gd.Doc.List {
				comment := c.Text
				if strings.Contains(comment, `goosegorm:"managed:false"`) {
					managed = false
				} else if strings.Contains(comment, `goosegorm:"managed:true"`) {
					managed = true
				}
			}
		}

		for _, spec := range gd.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}

			st, ok := ts.Type.(*ast.StructType)
			if !ok {
				continue
			}

			modelName := ts.Name.Name

			// Skip ignored models
			if ignoreMap[modelName] {
				continue
			}

			// Check struct tag on the struct itself (if supported)
			// Also check field-level tags
			fields := parseFields(st)

			// Check if struct has managed:false tag in fields (as a struct tag)
			// This is a bit unusual but we'll check struct-level comments and field tags
			for _, field := range fields {
				if strings.Contains(field.GooseTag, "managed:false") {
					managed = false
					break
				}
			}

			// Get custom table name if TableName() method exists
			customTableName := tableNameMethods[modelName]

			models = append(models, ParsedModel{
				Name:       modelName,
				Package:    pkgName,
				Managed:    managed,
				Fields:     fields,
				File:       fileName,
				StructNode: st,
				TableName:  customTableName,
			})
		}
	}

	return models
}

func parseFields(st *ast.StructType) []Field {
	var fields []Field

	if st.Fields == nil {
		return fields
	}

	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			// Embedded field
			continue
		}

		fieldName := field.Names[0].Name
		fieldType := exprToString(field.Type)

		gormTag := ""
		gooseTag := ""
		var indexes []IndexInfo

		if field.Tag != nil {
			tagValue := strings.Trim(field.Tag.Value, "`")
			tags := parseTag(tagValue)
			gormTag = tags["gorm"]
			gooseTag = tags["goosegorm"]

			// Parse indexes from GORM tag
			indexes = parseIndexesFromGormTag(gormTag, fieldName)
		}

		fields = append(fields, Field{
			Name:     fieldName,
			Type:     fieldType,
			GormTag:  gormTag,
			GooseTag: gooseTag,
			Indexes:  indexes,
		})
	}

	return fields
}

func exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprToString(e.X) + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + exprToString(e.Elt)
	case *ast.MapType:
		return "map[" + exprToString(e.Key) + "]" + exprToString(e.Value)
	case *ast.StarExpr:
		return "*" + exprToString(e.X)
	default:
		return "unknown"
	}
}

func parseTag(tag string) map[string]string {
	result := make(map[string]string)
	parts := strings.Split(tag, " ")
	for _, part := range parts {
		kv := strings.SplitN(part, ":", 2)
		if len(kv) == 2 {
			key := kv[0]
			value := strings.Trim(kv[1], `"`)
			result[key] = value
		}
	}
	return result
}

// parseIndexesFromGormTag parses index information from GORM tag
// Supports: index:idx_name, index:idx_name,unique, index:idx_name,priority:1
func parseIndexesFromGormTag(gormTag, fieldName string) []IndexInfo {
	var indexes []IndexInfo

	if gormTag == "" {
		return indexes
	}

	// Look for index: patterns
	parts := strings.Split(gormTag, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "index:") {
			// Extract index name and options
			indexSpec := strings.TrimPrefix(part, "index:")
			options := strings.Split(indexSpec, ",")

			indexName := options[0]
			unique := false
			priority := 0

			for i := 1; i < len(options); i++ {
				opt := strings.TrimSpace(options[i])
				if opt == "unique" {
					unique = true
				} else if strings.HasPrefix(opt, "priority:") {
					// Parse priority if needed
					priority = 1 // Default priority
				}
			}

			indexes = append(indexes, IndexInfo{
				Name:     indexName,
				Unique:   unique,
				Priority: priority,
			})
		} else if strings.HasPrefix(part, "uniqueIndex:") {
			// Named unique index
			indexName := strings.TrimPrefix(part, "uniqueIndex:")
			indexes = append(indexes, IndexInfo{
				Name:     indexName,
				Unique:   true,
				Priority: 0,
			})
		} else if part == "uniqueIndex" {
			// Unnamed unique index - use default name
			indexName := "idx_" + toSnakeCase(fieldName)
			indexes = append(indexes, IndexInfo{
				Name:     indexName,
				Unique:   true,
				Priority: 0,
			})
		}
	}

	return indexes
}

// findTableNameMethods finds all TableName() methods in the file and extracts their return values
func findTableNameMethods(file *ast.File, structTypes map[string]*ast.StructType) map[string]string {
	result := make(map[string]string)

	// First, collect all string constants in the file
	constants := findStringConstants(file)

	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}

		// Check if this is a method (has a receiver)
		if fd.Recv == nil || len(fd.Recv.List) == 0 {
			continue
		}

		// Check if method name is TableName
		if fd.Name.Name != "TableName" {
			continue
		}

		// Check if return type is string
		if fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
			continue
		}

		returnType := exprToString(fd.Type.Results.List[0].Type)
		if returnType != "string" {
			continue
		}

		// Get receiver type name
		recvType := fd.Recv.List[0].Type
		var recvTypeName string
		switch rt := recvType.(type) {
		case *ast.Ident:
			recvTypeName = rt.Name
		case *ast.StarExpr:
			if ident, ok := rt.X.(*ast.Ident); ok {
				recvTypeName = ident.Name
			}
		}

		if recvTypeName == "" {
			continue
		}

		// Extract return value from method body
		if fd.Body != nil {
			tableName := extractTableNameFromMethod(fd.Body, constants)
			if tableName != "" {
				result[recvTypeName] = tableName
			}
		}
	}

	return result
}

// findStringConstants finds all string constants in the file
func findStringConstants(file *ast.File) map[string]string {
	result := make(map[string]string)

	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}

		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}

			// Check if type is string
			if vs.Type != nil {
				typeName := exprToString(vs.Type)
				if typeName != "string" {
					continue
				}
			}

			// Extract constant names and values
			for i, name := range vs.Names {
				if i < len(vs.Values) {
					value := vs.Values[i]
					if lit, ok := value.(*ast.BasicLit); ok && lit.Kind == token.STRING {
						result[name.Name] = strings.Trim(lit.Value, `"`)
					}
				}
			}
		}
	}

	return result
}

// extractTableNameFromMethod extracts the string return value from a TableName() method
func extractTableNameFromMethod(body *ast.BlockStmt, constants map[string]string) string {
	// Look for return statements
	for _, stmt := range body.List {
		retStmt, ok := stmt.(*ast.ReturnStmt)
		if !ok {
			continue
		}

		if len(retStmt.Results) == 0 {
			continue
		}

		// Try to extract string literal or constant reference
		result := retStmt.Results[0]
		switch r := result.(type) {
		case *ast.BasicLit:
			if r.Kind == token.STRING {
				// Remove quotes
				return strings.Trim(r.Value, `"`)
			}
		case *ast.Ident:
			// Check if it's a constant reference
			if constValue, ok := constants[r.Name]; ok {
				return constValue
			}
		}
	}

	return ""
}

// GetTableName gets the table name for a model (from TableName() method or default)
func (m *ParsedModel) GetTableName() string {
	// If custom table name is set from TableName() method, use it
	if m.TableName != "" {
		return m.TableName
	}
	// Otherwise, return snake_case of model name
	return toSnakeCase(m.Name)
}

func toSnakeCase(s string) string {
	// Special case: If all uppercase letters, convert to all lowercase (not snake_case)
	// e.g., "ID" -> "id", "UUID" -> "uuid", "API" -> "api"
	if isAllUppercase(s) {
		return strings.ToLower(s)
	}

	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

// isAllUppercase checks if a string contains only uppercase letters
func isAllUppercase(s string) bool {
	if len(s) == 0 {
		return false
	}
	for _, r := range s {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

// ShouldIgnore checks if a model should be ignored
func (m *ParsedModel) ShouldIgnore(ignoreList []string) bool {
	for _, name := range ignoreList {
		if m.Name == name {
			return true
		}
	}
	return false
}
