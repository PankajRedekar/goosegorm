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

			models = append(models, ParsedModel{
				Name:       modelName,
				Package:    pkgName,
				Managed:    managed,
				Fields:     fields,
				File:       fileName,
				StructNode: st,
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

// GetTableName gets the table name for a model (from GORM tag or default)
func (m *ParsedModel) GetTableName() string {
	// This would require more sophisticated parsing
	// For now, return snake_case of model name
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
