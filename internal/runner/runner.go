package runner

import (
	"fmt"
	"reflect"
	"sort"

	"github.com/pankajredekar/goosegorm/internal/schema"
	"github.com/pankajredekar/goosegorm/internal/versioner"
	"gorm.io/gorm"
)

// Migration interface that all migrations must implement
type Migration interface {
	Version() string
	Name() string
	Up(db *gorm.DB) error
	Down(db *gorm.DB) error
}

// Registry holds all registered migrations
type Registry struct {
	migrations map[string]Migration
}

// NewRegistry creates a new migration registry
func NewRegistry() *Registry {
	return &Registry{
		migrations: make(map[string]Migration),
	}
}

// RegisterMigration registers a migration
func (r *Registry) RegisterMigration(m Migration) {
	r.migrations[m.Version()] = m
}

// GetMigration returns a migration by version
func (r *Registry) GetMigration(version string) (Migration, bool) {
	m, ok := r.migrations[version]
	return m, ok
}

// GetAllMigrations returns all migrations sorted by version
func (r *Registry) GetAllMigrations() []Migration {
	var migrations []Migration
	for _, m := range r.migrations {
		migrations = append(migrations, m)
	}
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version() < migrations[j].Version()
	})
	return migrations
}

// Runner executes migrations
type Runner struct {
	db        *gorm.DB
	registry  *Registry
	versioner *versioner.Versioner
}

// NewRunner creates a new migration runner
func NewRunner(db *gorm.DB, registry *Registry, versioner *versioner.Versioner) *Runner {
	return &Runner{
		db:        db,
		registry:  registry,
		versioner: versioner,
	}
}

// RunUp executes a migration's Up method
func (r *Runner) RunUp(m Migration) error {
	return m.Up(r.db)
}

// RunDown executes a migration's Down method
func (r *Runner) RunDown(m Migration) error {
	return m.Down(r.db)
}

// Migrate applies all pending migrations
func (r *Runner) Migrate() error {
	applied, err := r.versioner.GetAppliedVersions()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	appliedMap := make(map[string]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	allMigrations := r.registry.GetAllMigrations()
	var pending []Migration

	for _, m := range allMigrations {
		if !appliedMap[m.Version()] {
			pending = append(pending, m)
		}
	}

	for _, m := range pending {
		if err := r.RunUp(m); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", m.Version(), err)
		}
		if err := r.versioner.RecordApplied(m.Version(), m.Name()); err != nil {
			return fmt.Errorf("failed to record migration %s: %w", m.Version(), err)
		}
	}

	return nil
}

// Rollback rolls back the last N migrations
func (r *Runner) Rollback(n int) error {
	applied, err := r.versioner.GetAppliedVersions()
	if err != nil {
		return fmt.Errorf("failed to get applied migrations: %w", err)
	}

	if len(applied) == 0 {
		return fmt.Errorf("no migrations to rollback")
	}

	if n > len(applied) {
		n = len(applied)
	}

	// Rollback in reverse order
	for i := len(applied) - 1; i >= len(applied)-n; i-- {
		version := applied[i]
		m, ok := r.registry.GetMigration(version)
		if !ok {
			return fmt.Errorf("migration %s not found in registry", version)
		}

		if err := r.RunDown(m); err != nil {
			return fmt.Errorf("failed to rollback migration %s: %w", version, err)
		}

		if err := r.versioner.RemoveApplied(version); err != nil {
			return fmt.Errorf("failed to remove migration record %s: %w", version, err)
		}
	}

	return nil
}

// GetPendingMigrations returns migrations that haven't been applied
func (r *Runner) GetPendingMigrations() ([]Migration, error) {
	applied, err := r.versioner.GetAppliedVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	appliedMap := make(map[string]bool)
	for _, v := range applied {
		appliedMap[v] = true
	}

	allMigrations := r.registry.GetAllMigrations()
	var pending []Migration

	for _, m := range allMigrations {
		if !appliedMap[m.Version()] {
			pending = append(pending, m)
		}
	}

	return pending, nil
}

// GetAppliedMigrations returns migrations that have been applied
func (r *Runner) GetAppliedMigrations() ([]Migration, error) {
	applied, err := r.versioner.GetAppliedVersions()
	if err != nil {
		return nil, fmt.Errorf("failed to get applied migrations: %w", err)
	}

	var migrations []Migration
	for _, v := range applied {
		if m, ok := r.registry.GetMigration(v); ok {
			migrations = append(migrations, m)
		}
	}

	return migrations, nil
}

// SimulateSchema simulates all migrations to build up the schema state
func (r *Runner) SimulateSchema() (*schema.SchemaBuilder, error) {
	builder := schema.NewSchemaBuilder()
	allMigrations := r.registry.GetAllMigrations()

	for _, m := range allMigrations {
		// Pass the SchemaBuilder directly - migrations will check the type
		// using type assertion: if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok
		if err := callMigrationUp(m, builder); err != nil {
			return nil, fmt.Errorf("failed to simulate migration %s: %w", m.Version(), err)
		}
	}

	return builder, nil
}

// callMigrationUp calls the migration's Up method using reflection
func callMigrationUp(m Migration, db interface{}) error {
	val := reflect.ValueOf(m)
	method := val.MethodByName("Up")
	if !method.IsValid() {
		return fmt.Errorf("migration does not have Up method")
	}

	// Call Up method with db as argument
	results := method.Call([]reflect.Value{reflect.ValueOf(db)})

	if len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return err
		}
	}

	return nil
}

// callMigrationDown calls the migration's Down method using reflection
func callMigrationDown(m Migration, db interface{}) error {
	val := reflect.ValueOf(m)
	method := val.MethodByName("Down")
	if !method.IsValid() {
		return fmt.Errorf("migration does not have Down method")
	}

	// Call Down method with db as argument
	results := method.Call([]reflect.Value{reflect.ValueOf(db)})

	if len(results) > 0 && !results[0].IsNil() {
		if err, ok := results[0].Interface().(error); ok {
			return err
		}
	}

	return nil
}
