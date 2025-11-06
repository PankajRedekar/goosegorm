package diff

import (
	"strings"

	"github.com/pankajredekar/goosegorm/internal/modelreflect"
	"github.com/pankajredekar/goosegorm/internal/schema"
)

// Diff represents a difference between the schema and models
type Diff struct {
	Type      string // "create_table", "drop_table", "add_column", "drop_column", "modify_column"
	TableName string
	Column    *ColumnDiff
	Table     *TableDiff
}

// ColumnDiff represents a column difference
type ColumnDiff struct {
	Name    string
	Type    string
	OldType string
	Null    bool
	PK      bool
	Unique  bool
}

// TableDiff represents a table difference
type TableDiff struct {
	Name    string
	Columns []*ColumnDiff
}

// CompareSchema compares the simulated schema with the parsed models
func CompareSchema(simulatedSchema *schema.SchemaState, models []modelreflect.ParsedModel) ([]Diff, error) {
	var diffs []Diff

	// Build expected schema from models
	expectedSchema := buildExpectedSchema(models)

	// Find tables that exist in expected but not in simulated
	for tableName, expectedTable := range expectedSchema {
		if !schemaHasTable(simulatedSchema, tableName) {
			diffs = append(diffs, Diff{
				Type:      "create_table",
				TableName: tableName,
				Table:     expectedTable,
			})
		} else {
			// Table exists, check columns
			simulatedTable, _ := simulatedSchema.Tables[tableName]
			columnDiffs := compareColumns(simulatedTable, expectedTable)
			diffs = append(diffs, columnDiffs...)
		}
	}

	// Find tables that exist in simulated but not in expected (should be dropped)
	for tableName := range simulatedSchema.Tables {
		if !expectedSchemaHasTable(expectedSchema, tableName) {
			diffs = append(diffs, Diff{
				Type:      "drop_table",
				TableName: tableName,
			})
		}
	}

	return diffs, nil
}

func buildExpectedSchema(models []modelreflect.ParsedModel) map[string]*TableDiff {
	schema := make(map[string]*TableDiff)

	for _, model := range models {
		if !model.Managed {
			continue
		}

		tableName := model.GetTableName()
		table := &TableDiff{
			Name:    tableName,
			Columns: []*ColumnDiff{},
		}

		for _, field := range model.Fields {
			colType := mapGoTypeToSQLType(field.Type)
			col := &ColumnDiff{
				Name:   toSnakeCase(field.Name),
				Type:   colType,
				Null:   !isRequired(field.GormTag),
				PK:     isPrimaryKey(field.GormTag),
				Unique: isUnique(field.GormTag),
			}
			table.Columns = append(table.Columns, col)
		}

		schema[tableName] = table
	}

	return schema
}

func compareColumns(simulatedTable *schema.Table, expectedTable *TableDiff) []Diff {
	var diffs []Diff

	// Track which columns exist in expected
	expectedCols := make(map[string]*ColumnDiff)
	for _, col := range expectedTable.Columns {
		expectedCols[col.Name] = col
	}

	// Find columns to add or modify
	for _, expectedCol := range expectedTable.Columns {
		simCol, exists := simulatedTable.Columns[expectedCol.Name]
		if !exists {
			// Column doesn't exist, add it
			diffs = append(diffs, Diff{
				Type:      "add_column",
				TableName: expectedTable.Name,
				Column:    expectedCol,
			})
		} else {
			// Column exists, check if it needs modification
			if simCol.Type != expectedCol.Type ||
				simCol.Null != expectedCol.Null ||
				simCol.PK != expectedCol.PK ||
				simCol.Unique != expectedCol.Unique {
				diffs = append(diffs, Diff{
					Type:      "modify_column",
					TableName: expectedTable.Name,
					Column: &ColumnDiff{
						Name:    expectedCol.Name,
						Type:    expectedCol.Type,
						OldType: simCol.Type,
						Null:    expectedCol.Null,
						PK:      expectedCol.PK,
						Unique:  expectedCol.Unique,
					},
				})
			}
		}
	}

	// Find columns to drop
	for colName := range simulatedTable.Columns {
		if _, exists := expectedCols[colName]; !exists {
			diffs = append(diffs, Diff{
				Type:      "drop_column",
				TableName: expectedTable.Name,
				Column: &ColumnDiff{
					Name: colName,
				},
			})
		}
	}

	return diffs
}

func schemaHasTable(s *schema.SchemaState, tableName string) bool {
	_, exists := s.Tables[tableName]
	return exists
}

func expectedSchemaHasTable(schema map[string]*TableDiff, tableName string) bool {
	_, exists := schema[tableName]
	return exists
}

func mapGoTypeToSQLType(goType string) string {
	goType = strings.TrimSpace(goType)
	goType = strings.TrimPrefix(goType, "*")

	switch goType {
	case "string":
		return "string"
	case "int", "int64":
		return "bigint"
	case "int8":
		return "tinyint"
	case "int16":
		return "smallint"
	case "int32":
		return "integer"
	case "uint", "uint64":
		return "bigint"
	case "uint8":
		return "tinyint"
	case "uint16":
		return "smallint"
	case "uint32":
		return "integer"
	case "float32", "float64":
		return "float"
	case "bool":
		return "bool"
	case "time.Time":
		return "timestamp"
	default:
		return "string"
	}
}

func isRequired(gormTag string) bool {
	return !strings.Contains(gormTag, "not null") && !strings.Contains(gormTag, "NOT NULL")
}

func isPrimaryKey(gormTag string) bool {
	return strings.Contains(gormTag, "primaryKey") || strings.Contains(gormTag, "primary_key")
}

func isUnique(gormTag string) bool {
	return strings.Contains(gormTag, "unique") || strings.Contains(gormTag, "uniqueIndex")
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
