package schema

import (
	"fmt"
	"strings"
)

// SchemaBuilder provides in-memory schema simulation
type SchemaBuilder struct {
	Schema *SchemaState
}

// SchemaState represents the current database schema state
type SchemaState struct {
	Tables map[string]*Table
}

// Table represents a database table
type Table struct {
	Name        string
	Columns     map[string]*Column
	Constraints []string
	Indexes     []string
}

// Column represents a database column
type Column struct {
	Name   string
	Type   string
	Null   bool
	PK     bool
	Unique bool
}

// TableBuilder provides fluent API for building tables
type TableBuilder struct {
	builder *SchemaBuilder
	table   *Table
}

// NewSchemaBuilder creates a new schema builder
func NewSchemaBuilder() *SchemaBuilder {
	return &SchemaBuilder{
		Schema: &SchemaState{
			Tables: make(map[string]*Table),
		},
	}
}

// CreateTable creates a new table
func (b *SchemaBuilder) CreateTable(name string) *TableBuilder {
	table := &Table{
		Name:        name,
		Columns:     make(map[string]*Column),
		Constraints: []string{},
		Indexes:     []string{},
	}
	b.Schema.Tables[name] = table
	return &TableBuilder{
		builder: b,
		table:   table,
	}
}

// AlterTable gets an existing table for modification
func (b *SchemaBuilder) AlterTable(name string) *TableBuilder {
	table, exists := b.Schema.Tables[name]
	if !exists {
		// Create if doesn't exist (for backward compatibility)
		table = &Table{
			Name:        name,
			Columns:     make(map[string]*Column),
			Constraints: []string{},
			Indexes:     []string{},
		}
		b.Schema.Tables[name] = table
	}
	return &TableBuilder{
		builder: b,
		table:   table,
	}
}

// DropTable removes a table
func (b *SchemaBuilder) DropTable(name string) {
	delete(b.Schema.Tables, name)
}

// TableExists checks if a table exists
func (b *SchemaBuilder) TableExists(name string) bool {
	_, exists := b.Schema.Tables[name]
	return exists
}

// GetTable returns a table by name
func (b *SchemaBuilder) GetTable(name string) (*Table, bool) {
	table, exists := b.Schema.Tables[name]
	return table, exists
}

// AddColumn adds a column to the table
func (t *TableBuilder) AddColumn(name, colType string) *TableBuilder {
	col := &Column{
		Name:   name,
		Type:   colType,
		Null:   false,
		PK:     false,
		Unique: false,
	}
	t.table.Columns[name] = col
	return t
}

// AddColumnWithOptions adds a column with specific options
func (t *TableBuilder) AddColumnWithOptions(name, colType string, null, pk, unique bool) *TableBuilder {
	col := &Column{
		Name:   name,
		Type:   colType,
		Null:   null,
		PK:     pk,
		Unique: unique,
	}
	t.table.Columns[name] = col
	return t
}

// DropColumn removes a column from the table
func (t *TableBuilder) DropColumn(name string) *TableBuilder {
	delete(t.table.Columns, name)
	return t
}

// AddConstraint adds a constraint to the table
func (t *TableBuilder) AddConstraint(expr string) *TableBuilder {
	t.table.Constraints = append(t.table.Constraints, expr)
	return t
}

// AddIndex adds an index to the table
func (t *TableBuilder) AddIndex(name string) *TableBuilder {
	t.table.Indexes = append(t.table.Indexes, name)
	return t
}

// RenameColumn renames a column
func (t *TableBuilder) RenameColumn(oldName, newName string) *TableBuilder {
	if col, exists := t.table.Columns[oldName]; exists {
		col.Name = newName
		t.table.Columns[newName] = col
		delete(t.table.Columns, oldName)
	}
	return t
}

// ModifyColumn modifies a column's type or options
func (t *TableBuilder) ModifyColumn(name, colType string, null, pk, unique bool) *TableBuilder {
	if col, exists := t.table.Columns[name]; exists {
		col.Type = colType
		col.Null = null
		col.PK = pk
		col.Unique = unique
	}
	return t
}

// String returns a string representation of the schema
func (s *SchemaState) String() string {
	var sb strings.Builder
	for name, table := range s.Tables {
		sb.WriteString(fmt.Sprintf("Table: %s\n", name))
		for _, col := range table.Columns {
			attrs := []string{}
			if col.PK {
				attrs = append(attrs, "PRIMARY KEY")
			}
			if col.Unique {
				attrs = append(attrs, "UNIQUE")
			}
			if col.Null {
				attrs = append(attrs, "NULL")
			} else {
				attrs = append(attrs, "NOT NULL")
			}
			sb.WriteString(fmt.Sprintf("  Column: %s %s [%s]\n", col.Name, col.Type, strings.Join(attrs, ", ")))
		}
		if len(table.Constraints) > 0 {
			sb.WriteString(fmt.Sprintf("  Constraints: %s\n", strings.Join(table.Constraints, ", ")))
		}
		if len(table.Indexes) > 0 {
			sb.WriteString(fmt.Sprintf("  Indexes: %s\n", strings.Join(table.Indexes, ", ")))
		}
	}
	return sb.String()
}
