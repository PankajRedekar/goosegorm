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

		if field.Tag != nil {
			tagValue := strings.Trim(field.Tag.Value, "`")
			tags := parseTag(tagValue)
			gormTag = tags["gorm"]
			gooseTag = tags["goosegorm"]
		}

		fields = append(fields, Field{
			Name:     fieldName,
			Type:     fieldType,
			GormTag:  gormTag,
			GooseTag: gooseTag,
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

// GetTableName gets the table name for a model (from GORM tag or default)
func (m *ParsedModel) GetTableName() string {
	// This would require more sophisticated parsing
	// For now, return snake_case of model name
	return toSnakeCase(m.Name)
}

func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
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
