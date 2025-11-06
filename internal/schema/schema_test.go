package schema

import (
	"testing"
)

func TestNewSchemaBuilder(t *testing.T) {
	builder := NewSchemaBuilder()
	if builder == nil {
		t.Fatal("NewSchemaBuilder returned nil")
	}
	if builder.Schema == nil {
		t.Fatal("Schema is nil")
	}
	if builder.Schema.Tables == nil {
		t.Fatal("Tables map is nil")
	}
}

func TestCreateTable(t *testing.T) {
	builder := NewSchemaBuilder()
	tableBuilder := builder.CreateTable("users")

	if tableBuilder == nil {
		t.Fatal("CreateTable returned nil")
	}

	table, exists := builder.Schema.Tables["users"]
	if !exists {
		t.Fatal("Table 'users' was not created")
	}
	if table.Name != "users" {
		t.Errorf("Expected table name 'users', got '%s'", table.Name)
	}
}

func TestAddColumn(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumn("id", "uint").
		AddColumn("name", "string").
		AddColumn("email", "string")

	table := builder.Schema.Tables["users"]
	if len(table.Columns) != 3 {
		t.Errorf("Expected 3 columns, got %d", len(table.Columns))
	}

	col, exists := table.Columns["id"]
	if !exists {
		t.Fatal("Column 'id' was not added")
	}
	if col.Type != "uint" {
		t.Errorf("Expected column type 'uint', got '%s'", col.Type)
	}
}

func TestAddColumnWithOptions(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumnWithOptions("id", "uint", false, true, false).
		AddColumnWithOptions("email", "string", false, false, true)

	table := builder.Schema.Tables["users"]

	idCol := table.Columns["id"]
	if !idCol.PK {
		t.Error("Expected id column to be primary key")
	}
	if idCol.Null {
		t.Error("Expected id column to be NOT NULL")
	}

	emailCol := table.Columns["email"]
	if !emailCol.Unique {
		t.Error("Expected email column to be unique")
	}
}

func TestDropColumn(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumn("id", "uint").
		AddColumn("name", "string")

	table := builder.Schema.Tables["users"]
	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}

	builder.AlterTable("users").DropColumn("name")

	if len(table.Columns) != 1 {
		t.Errorf("Expected 1 column after drop, got %d", len(table.Columns))
	}
	if _, exists := table.Columns["name"]; exists {
		t.Error("Column 'name' should have been dropped")
	}
}

func TestAlterTable(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").AddColumn("id", "uint")

	// Alter existing table
	builder.AlterTable("users").AddColumn("email", "string")

	table := builder.Schema.Tables["users"]
	if len(table.Columns) != 2 {
		t.Errorf("Expected 2 columns, got %d", len(table.Columns))
	}
}

func TestAlterTableNonExistent(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.AlterTable("users").AddColumn("id", "uint")

	// Should create table if it doesn't exist
	table, exists := builder.Schema.Tables["users"]
	if !exists {
		t.Fatal("Table should have been created")
	}
	if len(table.Columns) != 1 {
		t.Errorf("Expected 1 column, got %d", len(table.Columns))
	}
}

func TestDropTable(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users")
	builder.CreateTable("posts")

	if len(builder.Schema.Tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(builder.Schema.Tables))
	}

	builder.DropTable("users")

	if len(builder.Schema.Tables) != 1 {
		t.Errorf("Expected 1 table after drop, got %d", len(builder.Schema.Tables))
	}
	if _, exists := builder.Schema.Tables["users"]; exists {
		t.Error("Table 'users' should have been dropped")
	}
}

func TestTableExists(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users")

	if !builder.TableExists("users") {
		t.Error("Table 'users' should exist")
	}
	if builder.TableExists("nonexistent") {
		t.Error("Table 'nonexistent' should not exist")
	}
}

func TestAddConstraint(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddConstraint("PRIMARY KEY (id)")

	table := builder.Schema.Tables["users"]
	if len(table.Constraints) != 1 {
		t.Errorf("Expected 1 constraint, got %d", len(table.Constraints))
	}
	if table.Constraints[0] != "PRIMARY KEY (id)" {
		t.Errorf("Expected constraint 'PRIMARY KEY (id)', got '%s'", table.Constraints[0])
	}
}

func TestAddIndex(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddIndex("idx_email")

	table := builder.Schema.Tables["users"]
	if len(table.Indexes) != 1 {
		t.Errorf("Expected 1 index, got %d", len(table.Indexes))
	}
	if table.Indexes[0] != "idx_email" {
		t.Errorf("Expected index 'idx_email', got '%s'", table.Indexes[0])
	}
}

func TestRenameColumn(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumn("old_name", "string")

	builder.AlterTable("users").RenameColumn("old_name", "new_name")

	table := builder.Schema.Tables["users"]
	if _, exists := table.Columns["old_name"]; exists {
		t.Error("Old column name should not exist")
	}
	if _, exists := table.Columns["new_name"]; !exists {
		t.Error("New column name should exist")
	}
}

func TestModifyColumn(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumnWithOptions("age", "int", true, false, false)

	builder.AlterTable("users").ModifyColumn("age", "bigint", false, false, true)

	table := builder.Schema.Tables["users"]
	col := table.Columns["age"]
	if col.Type != "bigint" {
		t.Errorf("Expected type 'bigint', got '%s'", col.Type)
	}
	if col.Null {
		t.Error("Expected column to be NOT NULL")
	}
	if !col.Unique {
		t.Error("Expected column to be unique")
	}
}

func TestSchemaString(t *testing.T) {
	builder := NewSchemaBuilder()
	builder.CreateTable("users").
		AddColumnWithOptions("id", "uint", false, true, false).
		AddColumn("name", "string").
		AddConstraint("PRIMARY KEY (id)")

	str := builder.Schema.String()
	if str == "" {
		t.Error("Schema.String() should not return empty string")
	}
	if len(str) < 10 {
		t.Error("Schema.String() should return meaningful output")
	}
}
