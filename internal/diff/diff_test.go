package diff

import (
	"testing"

	"github.com/pankajredekar/goosegorm/internal/modelreflect"
	"github.com/pankajredekar/goosegorm/internal/schema"
)

func TestCompareSchema_CreateTable(t *testing.T) {
	// Empty simulated schema
	simulated := schema.NewSchemaBuilder().Schema

	// Models with new table
	models := []modelreflect.ParsedModel{
		{
			Name:    "User",
			Managed: true,
			Fields: []modelreflect.Field{
				{Name: "ID", Type: "uint", GormTag: "primaryKey"},
				{Name: "Name", Type: "string"},
			},
		},
	}

	diffs, err := CompareSchema(simulated, models)
	if err != nil {
		t.Fatalf("CompareSchema failed: %v", err)
	}

	if len(diffs) != 1 {
		t.Errorf("Expected 1 diff, got %d", len(diffs))
	}

	if diffs[0].Type != "create_table" {
		t.Errorf("Expected diff type 'create_table', got '%s'", diffs[0].Type)
	}
	if diffs[0].TableName != "user" {
		t.Errorf("Expected table name 'user', got '%s'", diffs[0].TableName)
	}
}

func TestCompareSchema_AddColumn(t *testing.T) {
	// Simulated schema with existing table (must match model fields exactly)
	builder := schema.NewSchemaBuilder()
	builder.CreateTable("user"). // Note: GetTableName() returns "user" not "users"
					AddColumnWithOptions("id", "uint", false, true, false). // PK
					AddColumn("name", "string")

	// Models with additional column
	models := []modelreflect.ParsedModel{
		{
			Name:    "User",
			Managed: true,
			Fields: []modelreflect.Field{
				{Name: "ID", Type: "uint", GormTag: "primaryKey"},
				{Name: "Name", Type: "string"},
				{Name: "Email", Type: "string"},
			},
		},
	}

	diffs, err := CompareSchema(builder.Schema, models)
	if err != nil {
		t.Fatalf("CompareSchema failed: %v", err)
	}

	// Should have one add_column diff for email
	addColumnDiffs := 0
	for _, d := range diffs {
		if d.Type == "add_column" && d.Column.Name == "email" {
			addColumnDiffs++
		}
	}

	if addColumnDiffs == 0 {
		t.Errorf("Expected at least one add_column diff for email. All diffs: %+v", diffs)
	}
}

func TestCompareSchema_DropColumn(t *testing.T) {
	// Simulated schema with existing table
	builder := schema.NewSchemaBuilder()
	builder.CreateTable("user"). // Note: GetTableName() returns "user"
					AddColumnWithOptions("id", "uint", false, true, false). // PK
					AddColumn("name", "string").
					AddColumn("email", "string")

	// Models without email column
	models := []modelreflect.ParsedModel{
		{
			Name:    "User",
			Managed: true,
			Fields: []modelreflect.Field{
				{Name: "ID", Type: "uint", GormTag: "primaryKey"},
				{Name: "Name", Type: "string"},
			},
		},
	}

	diffs, err := CompareSchema(builder.Schema, models)
	if err != nil {
		t.Fatalf("CompareSchema failed: %v", err)
	}

	// Should have drop_column diff for email
	dropColumnDiffs := 0
	for _, d := range diffs {
		if d.Type == "drop_column" && d.Column.Name == "email" {
			dropColumnDiffs++
		}
	}

	if dropColumnDiffs == 0 {
		t.Errorf("Expected at least one drop_column diff for email. All diffs: %+v", diffs)
	}
}

func TestCompareSchema_ModifyColumn(t *testing.T) {
	// Simulated schema with existing table
	builder := schema.NewSchemaBuilder()
	builder.CreateTable("user"). // Note: GetTableName() returns "user"
					AddColumnWithOptions("id", "uint", false, true, false). // PK
					AddColumn("age", "int")

	// Models with different column type
	models := []modelreflect.ParsedModel{
		{
			Name:    "User",
			Managed: true,
			Fields: []modelreflect.Field{
				{Name: "ID", Type: "uint", GormTag: "primaryKey"},
				{Name: "Age", Type: "uint"},
			},
		},
	}

	diffs, err := CompareSchema(builder.Schema, models)
	if err != nil {
		t.Fatalf("CompareSchema failed: %v", err)
	}

	// Should have modify_column diff for age (int -> uint)
	modifyColumnDiffs := 0
	for _, d := range diffs {
		if d.Type == "modify_column" && d.Column.Name == "age" {
			modifyColumnDiffs++
			if d.Column.Type != "uint" {
				t.Errorf("Expected column type 'uint', got '%s'", d.Column.Type)
			}
		}
	}

	if modifyColumnDiffs == 0 {
		t.Errorf("Expected at least one modify_column diff for age. All diffs: %+v", diffs)
	}
}

func TestCompareSchema_DropTable(t *testing.T) {
	// Simulated schema with existing table
	builder := schema.NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumn("id", "uint")

	// No models (table should be dropped)
	models := []modelreflect.ParsedModel{}

	diffs, err := CompareSchema(builder.Schema, models)
	if err != nil {
		t.Fatalf("CompareSchema failed: %v", err)
	}

	// Should have drop_table diff
	dropTableDiffs := 0
	for _, d := range diffs {
		if d.Type == "drop_table" && d.TableName == "users" {
			dropTableDiffs++
		}
	}

	if dropTableDiffs == 0 {
		t.Errorf("Expected at least one drop_table diff for 'users'. All diffs: %+v", diffs)
	}
}

func TestCompareSchema_IgnoreUnmanagedModels(t *testing.T) {
	// Empty simulated schema
	simulated := schema.NewSchemaBuilder().Schema

	// Models with one managed and one unmanaged
	models := []modelreflect.ParsedModel{
		{
			Name:    "User",
			Managed: true,
			Fields: []modelreflect.Field{
				{Name: "ID", Type: "uint", GormTag: "primaryKey"},
			},
		},
		{
			Name:    "Legacy",
			Managed: false,
			Fields: []modelreflect.Field{
				{Name: "ID", Type: "int"},
			},
		},
	}

	diffs, err := CompareSchema(simulated, models)
	if err != nil {
		t.Fatalf("CompareSchema failed: %v", err)
	}

	// Should only create table for managed model
	createTableDiffs := 0
	for _, d := range diffs {
		if d.Type == "create_table" {
			createTableDiffs++
			if d.TableName != "user" {
				t.Errorf("Expected table name 'user', got '%s'", d.TableName)
			}
		}
	}

	if createTableDiffs != 1 {
		t.Errorf("Expected 1 create_table diff, got %d", createTableDiffs)
	}
}

func TestMapGoTypeToSQLType(t *testing.T) {
	tests := []struct {
		goType   string
		expected string
	}{
		{"string", "string"},
		{"int", "int"},
		{"uint", "uint"},
		{"int64", "int"},
		{"uint64", "uint"},
		{"float32", "float"},
		{"float64", "float"},
		{"bool", "bool"},
		{"time.Time", "timestamp"},
		{"*string", "string"},
		{"unknown", "string"},
	}

	for _, tt := range tests {
		t.Run(tt.goType, func(t *testing.T) {
			result := mapGoTypeToSQLType(tt.goType)
			if result != tt.expected {
				t.Errorf("mapGoTypeToSQLType(%s) = %s, expected %s", tt.goType, result, tt.expected)
			}
		})
	}
}
