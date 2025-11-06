package generator

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pankajredekar/goosegorm/internal/diff"
)

// Generator generates migration files from diffs
type Generator struct {
	migrationsDir string
	packageName   string
}

// NewGenerator creates a new migration generator
func NewGenerator(migrationsDir, packageName string) *Generator {
	return &Generator{
		migrationsDir: migrationsDir,
		packageName:   packageName,
	}
}

// GenerateMigration generates a migration file from diffs
func (g *Generator) GenerateMigration(name string, diffs []diff.Diff) (string, error) {
	if len(diffs) == 0 {
		return "", fmt.Errorf("no diffs to generate migration from")
	}

	version := generateVersion()
	fileName := fmt.Sprintf("%s_%s.go", version, sanitizeName(name))
	filePath := filepath.Join(g.migrationsDir, fileName)

	// Ensure directory exists
	if err := os.MkdirAll(g.migrationsDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create migrations directory: %w", err)
	}

	content := g.generateMigrationContent(version, name, diffs)

	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("failed to write migration file: %w", err)
	}

	return filePath, nil
}

func (g *Generator) generateMigrationContent(version, name string, diffs []diff.Diff) string {
	var sb strings.Builder

	// Header
	sb.WriteString(fmt.Sprintf("package %s\n\n", g.packageName))
	sb.WriteString("import (\n")
	sb.WriteString("\t\"gorm.io/gorm\"\n")
	sb.WriteString("\t\"github.com/pankajredekar/goosegorm\"\n")
	if needsTimeImport(diffs) {
		sb.WriteString("\t\"time\"\n")
	}
	sb.WriteString(")\n\n")

	// Migration struct
	structName := toCamelCase(name)
	sb.WriteString(fmt.Sprintf("type %s struct{}\n\n", structName))

	// Version method
	sb.WriteString(fmt.Sprintf("func (m %s) Version() string { return \"%s\" }\n\n", structName, version))

	// Name method
	sb.WriteString(fmt.Sprintf("func (m %s) Name() string { return \"%s\" }\n\n", structName, name))

	// Up method
	sb.WriteString(fmt.Sprintf("func (m %s) Up(db *gorm.DB) error {\n", structName))
	sb.WriteString("\tif sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {\n")
	sb.WriteString(g.generateUpSimulation(diffs))
	sb.WriteString("\t\treturn nil\n")
	sb.WriteString("\t}\n\n")
	upRealDB := g.generateUpRealDB(diffs)
	sb.WriteString(upRealDB)
	if !strings.Contains(upRealDB, "return ") {
		sb.WriteString("\treturn nil\n")
	}
	sb.WriteString("}\n\n")

	// Down method
	sb.WriteString(fmt.Sprintf("func (m %s) Down(db *gorm.DB) error {\n", structName))
	sb.WriteString("\tif sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {\n")
	sb.WriteString(g.generateDownSimulation(diffs))
	sb.WriteString("\t\treturn nil\n")
	sb.WriteString("\t}\n\n")
	downRealDB := g.generateDownRealDB(diffs)
	sb.WriteString(downRealDB)
	if !strings.Contains(downRealDB, "return ") {
		sb.WriteString("\treturn nil\n")
	}
	sb.WriteString("}\n\n")

	// Init function
	sb.WriteString("func init() {\n")
	sb.WriteString(fmt.Sprintf("\tgoosegorm.RegisterMigration(%s{})\n", structName))
	sb.WriteString("}\n")

	return sb.String()
}

func (g *Generator) generateUpSimulation(diffs []diff.Diff) string {
	var sb strings.Builder
	sb.WriteString("\t\t// Simulation mode\n")

	for _, d := range diffs {
		switch d.Type {
		case "create_table":
			sb.WriteString(fmt.Sprintf("\t\tsim.CreateTable(\"%s\").\n", d.TableName))
			for i, col := range d.Table.Columns {
				if i == len(d.Table.Columns)-1 {
					// Last column, no trailing dot
					sb.WriteString(fmt.Sprintf("\t\t\tAddColumnWithOptions(\"%s\", \"%s\", %v, %v, %v)\n",
						col.Name, col.Type, col.Null, col.PK, col.Unique))
				} else {
					// Not last column, add trailing dot
					sb.WriteString(fmt.Sprintf("\t\t\tAddColumnWithOptions(\"%s\", \"%s\", %v, %v, %v).\n",
						col.Name, col.Type, col.Null, col.PK, col.Unique))
				}
			}
			sb.WriteString("\t\t\n")
		case "drop_table":
			sb.WriteString(fmt.Sprintf("\t\tsim.DropTable(\"%s\")\n", d.TableName))
		case "add_column":
			sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").AddColumnWithOptions(\"%s\", \"%s\", %v, %v, %v)\n",
				d.TableName, d.Column.Name, d.Column.Type, d.Column.Null, d.Column.PK, d.Column.Unique))
		case "drop_column":
			sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").DropColumn(\"%s\")\n",
				d.TableName, d.Column.Name))
		case "modify_column":
			sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").ModifyColumn(\"%s\", \"%s\", %v, %v, %v)\n",
				d.TableName, d.Column.Name, d.Column.Type, d.Column.Null, d.Column.PK, d.Column.Unique))
		case "add_index":
			// Add index to simulation
			if d.Index != nil {
				sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").AddIndex(\"%s\")\n", d.TableName, d.Index.Name))
			}
		case "drop_index":
			// Drop index from simulation
			if d.Index != nil {
				sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").DropIndex(\"%s\")\n", d.TableName, d.Index.Name))
			}
		}
	}

	return sb.String()
}

func (g *Generator) generateDownSimulation(diffs []diff.Diff) string {
	var sb strings.Builder
	sb.WriteString("\t\t// Simulation mode - reverse operations\n")

	// Reverse the order for down
	for i := len(diffs) - 1; i >= 0; i-- {
		d := diffs[i]
		switch d.Type {
		case "create_table":
			sb.WriteString(fmt.Sprintf("\t\tsim.DropTable(\"%s\")\n", d.TableName))
		case "drop_table":
			sb.WriteString(fmt.Sprintf("\t\tsim.CreateTable(\"%s\")\n", d.TableName))
			// Note: We'd need to store original table structure for proper reversal
			// For now, just create empty table
		case "add_column":
			sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").DropColumn(\"%s\")\n",
				d.TableName, d.Column.Name))
		case "drop_column":
			sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").AddColumnWithOptions(\"%s\", \"%s\", %v, %v, %v)\n",
				d.TableName, d.Column.Name, d.Column.Type, d.Column.Null, d.Column.PK, d.Column.Unique))
		case "modify_column":
			// Revert to old type
			sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").ModifyColumn(\"%s\", \"%s\", %v, %v, %v)\n",
				d.TableName, d.Column.Name, d.Column.OldType, d.Column.Null, d.Column.PK, d.Column.Unique))
		case "add_index":
			// Reverse: Drop index
			if d.Index != nil {
				sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").DropIndex(\"%s\")\n", d.TableName, d.Index.Name))
			}
		case "drop_index":
			// Reverse: Add index back
			if d.Index != nil {
				sb.WriteString(fmt.Sprintf("\t\tsim.AlterTable(\"%s\").AddIndex(\"%s\")\n", d.TableName, d.Index.Name))
			}
		}
	}

	return sb.String()
}

func (g *Generator) generateUpRealDB(diffs []diff.Diff) string {
	var sb strings.Builder
	sb.WriteString("\t// Real DB mode\n")

	// Track structs to avoid duplicates
	definedStructs := make(map[string]bool)

	for _, d := range diffs {
		switch d.Type {
		case "create_table":
			// Generate struct definition and AutoMigrate
			structName := toPascalCase(d.TableName)
			if !definedStructs[structName] {
				sb.WriteString(fmt.Sprintf("\ttype %s struct {\n", structName))
				for _, col := range d.Table.Columns {
					fieldName := toPascalCase(col.Name)
					goType := mapSQLTypeToGo(col.Type, col.PK)
					gormTags := buildGormTags(col)
					sb.WriteString(fmt.Sprintf("\t\t%s %s `%s`\n", fieldName, goType, gormTags))
				}
				sb.WriteString(fmt.Sprintf("\t}\n"))
				definedStructs[structName] = true
			}
			sb.WriteString(fmt.Sprintf("\tif err := db.Table(\"%s\").AutoMigrate(&%s{}); err != nil {\n", d.TableName, structName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "drop_table":
			// Use Migrator().DropTable with table name directly
			sb.WriteString(fmt.Sprintf("\tif err := db.Migrator().DropTable(\"%s\"); err != nil {\n", d.TableName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "add_column":
			// Use Migrator().AddColumn with a struct containing the field
			structName := toPascalCase(d.TableName)
			fieldName := toPascalCase(d.Column.Name)
			structKey := structName + "_" + fieldName
			if !definedStructs[structKey] {
				goType := mapSQLTypeToGo(d.Column.Type, d.Column.PK)
				gormTags := buildGormTags(d.Column)
				sb.WriteString(fmt.Sprintf("\ttype %s%s struct {\n", structName, fieldName))
				sb.WriteString(fmt.Sprintf("\t\t%s %s `%s`\n", fieldName, goType, gormTags))
				sb.WriteString(fmt.Sprintf("\t}\n"))
				definedStructs[structKey] = true
			}
			sb.WriteString(fmt.Sprintf("\tif err := db.Table(\"%s\").Migrator().AddColumn(&%s%s{}, \"%s\"); err != nil {\n", d.TableName, structName, fieldName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "drop_column":
			// Use Migrator().DropColumn with table name
			fieldName := toPascalCase(d.Column.Name)
			sb.WriteString(fmt.Sprintf("\tif err := db.Migrator().DropColumn(\"%s\", \"%s\"); err != nil {\n", d.TableName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "add_index":
			// Create index using raw SQL
			if d.Index != nil {
				indexExpr := strings.Join(d.Index.Fields, ", ")
				uniqueStr := ""
				if d.Index.Unique {
					uniqueStr = "UNIQUE "
				}
				sb.WriteString(fmt.Sprintf("\t// Create index %s on %s (%s)\n", d.Index.Name, d.TableName, indexExpr))
				sb.WriteString(fmt.Sprintf("\tif err := db.Exec(\"CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)\").Error; err != nil {\n", uniqueStr, d.Index.Name, d.TableName, indexExpr))
				sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
				sb.WriteString(fmt.Sprintf("\t}\n"))
			}
		case "drop_index":
			// Drop index using raw SQL
			if d.Index != nil {
				sb.WriteString(fmt.Sprintf("\tif err := db.Exec(\"DROP INDEX IF EXISTS %s\").Error; err != nil {\n", d.Index.Name))
				sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
				sb.WriteString(fmt.Sprintf("\t}\n"))
			}
		case "modify_column":
			// Use AutoMigrate with updated struct
			structName := toPascalCase(d.TableName)
			fieldName := toPascalCase(d.Column.Name)
			goType := mapSQLTypeToGo(d.Column.Type, d.Column.PK)
			gormTags := buildGormTags(d.Column)
			sb.WriteString(fmt.Sprintf("\ttype %s%s struct {\n", structName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\t%s %s `%s`\n", fieldName, goType, gormTags))
			sb.WriteString(fmt.Sprintf("\t}\n"))
			sb.WriteString(fmt.Sprintf("\tif err := db.Table(\"%s\").AutoMigrate(&%s%s{}); err != nil {\n", d.TableName, structName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		}
	}

	sb.WriteString("\treturn nil\n")

	return sb.String()
}

func (g *Generator) generateDownRealDB(diffs []diff.Diff) string {
	var sb strings.Builder
	sb.WriteString("\t// Real DB mode - reverse operations\n")

	// Track structs to avoid duplicates
	definedStructs := make(map[string]bool)

	for i := len(diffs) - 1; i >= 0; i-- {
		d := diffs[i]
		switch d.Type {
		case "create_table":
			// Reverse: Drop table
			sb.WriteString(fmt.Sprintf("\tif err := db.Migrator().DropTable(\"%s\"); err != nil {\n", d.TableName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "drop_table":
			// Reverse: Recreate table (would need original structure, simplified for now)
			sb.WriteString(fmt.Sprintf("\t// Note: Recreating table %s requires original structure\n", d.TableName))
			sb.WriteString(fmt.Sprintf("\t// This is a simplified implementation - adjust as needed\n"))
			structName := toPascalCase(d.TableName)
			if !definedStructs[structName] {
				sb.WriteString(fmt.Sprintf("\ttype %s struct {\n", structName))
				sb.WriteString(fmt.Sprintf("\t\tID uint `gorm:\"primaryKey\"`\n"))
				sb.WriteString(fmt.Sprintf("\t}\n"))
				definedStructs[structName] = true
			}
			sb.WriteString(fmt.Sprintf("\tif err := db.Table(\"%s\").AutoMigrate(&%s{}); err != nil {\n", d.TableName, structName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "add_column":
			// Reverse: Drop column
			fieldName := toPascalCase(d.Column.Name)
			sb.WriteString(fmt.Sprintf("\tif err := db.Migrator().DropColumn(\"%s\", \"%s\"); err != nil {\n", d.TableName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "drop_column":
			// Reverse: Add column back
			structName := toPascalCase(d.TableName)
			fieldName := toPascalCase(d.Column.Name)
			structKey := structName + "_" + fieldName
			if !definedStructs[structKey] {
				goType := mapSQLTypeToGo(d.Column.Type, d.Column.PK)
				gormTags := buildGormTags(d.Column)
				sb.WriteString(fmt.Sprintf("\ttype %s%s struct {\n", structName, fieldName))
				sb.WriteString(fmt.Sprintf("\t\t%s %s `%s`\n", fieldName, goType, gormTags))
				sb.WriteString(fmt.Sprintf("\t}\n"))
				definedStructs[structKey] = true
			}
			sb.WriteString(fmt.Sprintf("\tif err := db.Table(\"%s\").Migrator().AddColumn(&%s%s{}, \"%s\"); err != nil {\n", d.TableName, structName, fieldName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "modify_column":
			// Reverse: Revert to old type
			structName := toPascalCase(d.TableName)
			fieldName := toPascalCase(d.Column.Name)
			goType := mapSQLTypeToGo(d.Column.OldType, d.Column.PK)
			gormTags := buildGormTags(d.Column)
			sb.WriteString(fmt.Sprintf("\ttype %s%s struct {\n", structName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\t%s %s `%s`\n", fieldName, goType, gormTags))
			sb.WriteString(fmt.Sprintf("\t}\n"))
			sb.WriteString(fmt.Sprintf("\tif err := db.Table(\"%s\").AutoMigrate(&%s%s{}); err != nil {\n", d.TableName, structName, fieldName))
			sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
			sb.WriteString(fmt.Sprintf("\t}\n"))
		case "add_index":
			// Reverse: Drop index
			if d.Index != nil {
				sb.WriteString(fmt.Sprintf("\tif err := db.Exec(\"DROP INDEX IF EXISTS %s\").Error; err != nil {\n", d.Index.Name))
				sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
				sb.WriteString(fmt.Sprintf("\t}\n"))
			}
		case "drop_index":
			// Reverse: Recreate index
			if d.Index != nil {
				indexExpr := strings.Join(d.Index.Fields, ", ")
				uniqueStr := ""
				if d.Index.Unique {
					uniqueStr = "UNIQUE "
				}
				sb.WriteString(fmt.Sprintf("\tif err := db.Exec(\"CREATE %sINDEX IF NOT EXISTS %s ON %s (%s)\").Error; err != nil {\n", uniqueStr, d.Index.Name, d.TableName, indexExpr))
				sb.WriteString(fmt.Sprintf("\t\treturn err\n"))
				sb.WriteString(fmt.Sprintf("\t}\n"))
			}
		}
	}

	sb.WriteString("\treturn nil\n")

	return sb.String()
}

func generateVersion() string {
	now := time.Now()
	return fmt.Sprintf("%04d%02d%02d%02d%02d%02d",
		now.Year(), now.Month(), now.Day(),
		now.Hour(), now.Minute(), now.Second())
}

func sanitizeName(name string) string {
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, "-", "_")
	return name
}

func toCamelCase(s string) string {
	parts := strings.Split(s, "_")
	for i := 0; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// toPascalCase converts snake_case to PascalCase
func toPascalCase(s string) string {
	return toCamelCase(s)
}

// mapSQLTypeToGo converts SQL type string to Go type
func mapSQLTypeToGo(sqlType string, isPK bool) string {
	switch sqlType {
	case "bigint":
		if isPK {
			return "uint"
		}
		return "int64"
	case "integer":
		if isPK {
			return "uint"
		}
		return "int32"
	case "smallint":
		return "int16"
	case "tinyint":
		return "int8"
	case "string":
		return "string"
	case "float":
		return "float64"
	case "bool":
		return "bool"
	case "timestamp":
		return "time.Time"
	default:
		return "string"
	}
}

// buildGormTags builds GORM tags from column information
func buildGormTags(col *diff.ColumnDiff) string {
	var tags []string

	if col.PK {
		tags = append(tags, "primaryKey")
	}

	if col.Unique {
		tags = append(tags, "uniqueIndex")
	}

	if !col.Null {
		tags = append(tags, "not null")
	}

	if len(tags) == 0 {
		return "gorm:\"\""
	}

	return fmt.Sprintf("gorm:\"%s\"", strings.Join(tags, ";"))
}

// buildGormTagsWithTableName builds GORM tags with table name specification
func buildGormTagsWithTableName(col *diff.ColumnDiff, tableName string) string {
	// Table name is handled via db.Table() in AutoMigrate, not in tags
	return buildGormTags(col)
}

// needsTimeImport checks if any diff requires time.Time type
func needsTimeImport(diffs []diff.Diff) bool {
	for _, d := range diffs {
		if d.Table != nil {
			for _, col := range d.Table.Columns {
				if col.Type == "timestamp" {
					return true
				}
			}
		}
		if d.Column != nil && d.Column.Type == "timestamp" {
			return true
		}
	}
	return false
}
