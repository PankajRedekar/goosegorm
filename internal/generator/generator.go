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
	sb.WriteString(g.generateUpRealDB(diffs))
	sb.WriteString("\treturn nil\n")
	sb.WriteString("}\n\n")

	// Down method
	sb.WriteString(fmt.Sprintf("func (m %s) Down(db *gorm.DB) error {\n", structName))
	sb.WriteString("\tif sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {\n")
	sb.WriteString(g.generateDownSimulation(diffs))
	sb.WriteString("\t\treturn nil\n")
	sb.WriteString("\t}\n\n")
	sb.WriteString(g.generateDownRealDB(diffs))
	sb.WriteString("\treturn nil\n")
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
			sb.WriteString(fmt.Sprintf("\t\tsim.CreateTable(\"%s\")\n", d.TableName))
			for _, col := range d.Table.Columns {
				sb.WriteString(fmt.Sprintf("\t\t\t.AddColumnWithOptions(\"%s\", \"%s\", %v, %v, %v)\n",
					col.Name, col.Type, col.Null, col.PK, col.Unique))
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
		}
	}

	return sb.String()
}

func (g *Generator) generateUpRealDB(diffs []diff.Diff) string {
	var sb strings.Builder
	sb.WriteString("\t// Real DB mode\n")

	for _, d := range diffs {
		switch d.Type {
		case "create_table":
			// Generate AutoMigrate call - would need model type
			sb.WriteString(fmt.Sprintf("\t// TODO: AutoMigrate model for table %s\n", d.TableName))
		case "drop_table":
			sb.WriteString(fmt.Sprintf("\t// TODO: Drop table %s\n", d.TableName))
		case "add_column":
			sb.WriteString(fmt.Sprintf("\t// TODO: Add column %s to table %s\n", d.Column.Name, d.TableName))
		case "drop_column":
			sb.WriteString(fmt.Sprintf("\t// TODO: Drop column %s from table %s\n", d.Column.Name, d.TableName))
		case "modify_column":
			sb.WriteString(fmt.Sprintf("\t// TODO: Modify column %s in table %s\n", d.Column.Name, d.TableName))
		}
	}

	return sb.String()
}

func (g *Generator) generateDownRealDB(diffs []diff.Diff) string {
	var sb strings.Builder
	sb.WriteString("\t// Real DB mode - reverse operations\n")

	for i := len(diffs) - 1; i >= 0; i-- {
		d := diffs[i]
		switch d.Type {
		case "create_table":
			sb.WriteString(fmt.Sprintf("\t// TODO: Drop table %s\n", d.TableName))
		case "drop_table":
			sb.WriteString(fmt.Sprintf("\t// TODO: Recreate table %s\n", d.TableName))
		case "add_column":
			sb.WriteString(fmt.Sprintf("\t// TODO: Drop column %s from table %s\n", d.Column.Name, d.TableName))
		case "drop_column":
			sb.WriteString(fmt.Sprintf("\t// TODO: Add column %s to table %s\n", d.Column.Name, d.TableName))
		case "modify_column":
			sb.WriteString(fmt.Sprintf("\t// TODO: Revert column %s in table %s\n", d.Column.Name, d.TableName))
		}
	}

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
