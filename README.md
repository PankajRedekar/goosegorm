# GooseGORM

Django-style migration framework for GORM — pure Go, no SQL files, no external dependencies.

## Features

- ✅ Go-based migration files (no SQL)
- ✅ In-memory schema simulation (SchemaBuilder)
- ✅ Model-level control with `goosegorm:"managed:false"`
- ✅ Automatic migration generation from model changes
- ✅ Custom table names via `TableName()` method (supports constants)
- ✅ PostgreSQL reserved keyword handling (automatic identifier quoting)
- ✅ Temporary compiled migrator for reliable execution (auto-generated and cleaned up)
- ✅ Production-ready migrator binary via `build` command
- ✅ Version tracking in database
- ✅ Rollback support
- ✅ Full GORM struct support (real compiled code, not AST interpretation)

## Installation

Install the GooseGORM CLI tool:

```bash
# Install latest version
go install github.com/pankajredekar/goosegorm/cmd/goosegorm@latest

# Or install a specific version
go install github.com/pankajredekar/goosegorm/cmd/goosegorm@v0.5.9
```

Make sure `$GOPATH/bin` or `$HOME/go/bin` is in your `PATH` to use the `goosegorm` command.

Alternatively, for development, you can run directly:
```bash
go run github.com/pankajredekar/goosegorm/cmd/goosegorm
```

## Quick Start

1. **Initialize a project:**
   ```bash
   goosegorm init
   ```

2. **Create models:**
   ```go
   // models/user.go
   package models

   type User struct {
       ID    uint   `gorm:"primaryKey"`
       Name  string
       Email string `gorm:"uniqueIndex"`
   }
   
   // Optional: Custom table name
   func (User) TableName() string {
       return "custom_users"
   }
   ```

3. **Generate migrations:**
   ```bash
   # Auto-generate from model changes
   goosegorm makemigrations
   
   # Or create an empty migration file to fill in manually
   goosegorm makemigrations --empty add_user_preferences
   ```

4. **Apply migrations:**
   
   **Development:**
   ```bash
   goosegorm migrate
   ```
   
   This will:
   - Create a temporary migrator package (`.goosegorm_migrator`)
   - Build the migrator binary
   - Run the compiled migrator to execute migrations
   - Automatically clean up the temporary package
   - Execute migrations using real compiled code with proper struct definitions
   
   **Production:**
   ```bash
   # Step 1: Build the migrator binary
   goosegorm build
   
   # Step 2: Deploy the binary to your production server
   # (The binary is saved to the path specified in goosegorm.yml build_path)
   
   # Step 3: Run migrations in production
   ./bin/goosegorm migrate
   
   # Or use other commands
   ./bin/goosegorm show      # Check migration status
   ./bin/goosegorm rollback  # Rollback if needed
   ```
   
   The production binary is standalone and doesn't require the `goosegorm` CLI tool to be installed on the server.

## Configuration (goosegorm.yml)

```yaml
database_url: sqlite://:memory:  # or postgres://user:pass@localhost:5432/myapp?sslmode=disable
models_dir: ./models
migrations_dir: ./migrations
package_name: migrations
migration_table: _goosegorm_migrations
ignore_models: []
build_path: ./bin/goosegorm  # Optional: path for build command output
```

**Note:** The `main_pkg_path` option is no longer used. Migrations are executed using a temporary compiled migrator that is automatically created and cleaned up during the `migrate` command.

**Supported databases:**
- SQLite: `sqlite://:memory:` (in-memory) or `sqlite://./db.sqlite` (file)
- PostgreSQL: `postgres://user:pass@localhost:5432/myapp?sslmode=disable`

## Model-Level Control

### Custom Table Names

GooseGORM automatically detects and uses custom table names from `TableName()` methods:

```go
const UserTableName string = "auth_customuser"

type User struct {
    ID    uint   `gorm:"primaryKey"`
    Email string `gorm:"uniqueIndex"`
}

func (User) TableName() string {
    return UserTableName  // Supports constants too
}
```

The generated migrations will use `auth_customuser` instead of the default `user` table name.

### Excluding Models

Exclude models from migrations using the `goosegorm:"managed:false"` tag:

```go
// Excluded from migrations
//goosegorm:"managed:false"
type LegacyTransaction struct {
    ID   int
    Note string
}
```

## How It Works

### Temporary Compiled Migrator Approach

GooseGORM uses a **temporary compiled migrator** approach for reliable migration execution:

1. **Migration Generation**: `goosegorm makemigrations` generates:
   - Migration files in `migrations/` directory
   - No permanent migrator files are created

2. **Migration Execution**: `goosegorm migrate`:
   - Creates a temporary package (`.goosegorm_migrator`) in the same directory as `goosegorm.yml`
   - Generates `main.go` that imports your migrations package
   - Creates `go.mod` with proper dependencies and replace directives
   - Builds the migrator binary
   - Runs the compiled migrator to execute migrations
   - Automatically deletes the temporary package after execution
   - Migrations register themselves via `init()` functions
   - Real compiled code executes with proper struct definitions

**Benefits:**
- ✅ Real compiled Go code (not AST interpretation)
- ✅ Proper struct definitions work correctly
- ✅ Full GORM feature support
- ✅ Works on all platforms (Windows, Linux, macOS)
- ✅ Type-safe and reliable
- ✅ No project pollution - temporary files are automatically cleaned up
- ✅ No permanent migrator files in your project

### Generated Files

After running `goosegorm makemigrations`, you'll have:

```
your-project/
├── migrations/
│   ├── 202511070928440001_create_product.go
│   └── 202511070928440002_add_index.go
└── models/
    └── product.go
```

**Note:** No permanent migrator files are generated. During `goosegorm migrate`, a temporary `.goosegorm_migrator` directory is created, used, and automatically deleted. This keeps your project clean while still using real compiled code for reliable migration execution.

## Migration Format

Migrations implement the `Migration` interface and support dual execution mode:

```go
package migrations

import (
    "gorm.io/gorm"
    "github.com/pankajredekar/goosegorm"
)

type AddEmailToUsers struct{}

func (m AddEmailToUsers) Version() string { return "20251107193000" }
func (m AddEmailToUsers) Name() string    { return "add_email_to_users" }

func (m AddEmailToUsers) Up(db *gorm.DB) error {
    // Simulation mode (for makemigrations)
    if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
        sim.AlterTable("users").AddColumn("email", "string")
        return nil
    }
    
    // Real DB mode (compiled migrator execution)
    type UserEmail struct {
        Email string `gorm:"column:email;size:255"`
    }
    return db.Table("users").Migrator().AddColumn(&UserEmail{}, "email")
}

// Example: Creating indexes (quotes are automatically escaped for reserved keywords)
func (m AddIndexToUsers) Up(db *gorm.DB) error {
    if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
        sim.AlterTable("user").AddIndex("idx_email")  // "user" is a reserved keyword
        return nil
    }
    
    // Generated SQL automatically quotes identifiers: 
    // CREATE INDEX IF NOT EXISTS idx_email ON "user" ("email")
    if err := db.Exec("CREATE INDEX IF NOT EXISTS idx_email ON \"user\" (\"email\")").Error; err != nil {
        return err
    }
    return nil
}

func (m AddEmailToUsers) Down(db *gorm.DB) error {
    // Simulation mode (for makemigrations)
    if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
        sim.AlterTable("users").DropColumn("email")
        return nil
    }
    
    // Real DB mode (compiled migrator execution)
    return db.Migrator().DropColumn("users", "email")
}

func init() {
    goosegorm.RegisterMigration(AddEmailToUsers{})
}
```

### Empty Migration Template

When using `goosegorm makemigrations --empty`, you get a pre-populated template:

```go
package migrations

import (
    "gorm.io/gorm"
    "github.com/pankajredekar/goosegorm"
)

type AddUserPreferences struct{}

func (m AddUserPreferences) Version() string { return "202511071215200001" }
func (m AddUserPreferences) Name() string { return "add_user_preferences" }

func (m AddUserPreferences) Up(db *gorm.DB) error {
    if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
        // Simulation mode
        // TODO: Add your simulation logic here
        // Example: sim.AlterTable("users").AddColumn("preferences", "string")
        return nil
    }

    // Real DB mode
    // TODO: Add your migration logic here
    // Example:
    // type UserPreferences struct {
    //     Preferences string `gorm:"type:text"`
    // }
    // if err := db.Table("users").Migrator().AddColumn(&UserPreferences{}, "preferences"); err != nil {
    //     return err
    // }
    return nil
}

func (m AddUserPreferences) Down(db *gorm.DB) error {
    if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
        // Simulation mode - reverse operations
        // TODO: Add reverse simulation logic here
        // Example: sim.AlterTable("users").DropColumn("preferences")
        return nil
    }

    // Real DB mode - reverse operations
    // TODO: Add reverse migration logic here
    // Example:
    // if err := db.Migrator().DropColumn("users", "preferences"); err != nil {
    //     return err
    // }
    return nil
}

func init() {
    goosegorm.RegisterMigration(AddUserPreferences{})
}
```

## CLI Commands

- `goosegorm init` - Initialize project
- `goosegorm makemigrations` - Generate migration files from model changes
- `goosegorm makemigrations --empty [name]` - Create an empty migration file (optional name)
- `goosegorm migrate` - Apply pending migrations (requires migrations to exist)
- `goosegorm rollback [n]` - Rollback last N migrations (default: 1)
- `goosegorm show` - Show migration status (applied and pending)
- `goosegorm build` - Build migrator binary for production (requires migrations to exist)

### Empty Migrations

Create empty migration files for custom migrations (data migrations, complex schema changes, etc.):

```bash
# With a name (recommended)
goosegorm makemigrations --empty add_user_preferences
# Creates: 202511071215200001_add_user_preferences.go
# Struct: AddUserPreferences

# Without a name (uses Migration{timestamp} format)
goosegorm makemigrations --empty
# Creates: 202511071215200002_migration.go
# Struct: Migration202511071215200002
```

Empty migrations include:
- Pre-populated `Up()` and `Down()` methods with simulation mode checks
- TODO comments with example code
- Proper struct naming and registration
- Ready-to-fill template for your custom migration logic

### Build Command

The `build` command creates a standalone migrator binary for production use:

```bash
# Add build_path to goosegorm.yml first
goosegorm build
```

The binary can then be used independently:
```bash
./bin/goosegorm migrate
./bin/goosegorm rollback 2
./bin/goosegorm show
```

**Note:** Both `build` and `migrate` commands require migrations to exist. Run `goosegorm makemigrations` first if you see errors about missing migrations.

## Examples

The repository includes complete examples in the `examples/` folder:

### Basic Example (`examples/`)
- **`examples/models/user.go`** - Example User model with GORM tags, indexes, and soft deletes
- **`examples/migrations/`** - Example migration files showing:
  - Creating tables with columns and options
  - Adding indexes (unique and regular)
  - Adding columns to existing tables
- **`examples/goosegorm.yml.example`** - Example configuration file

### Full App Example (`examples/app/`)
A complete working application demonstrating the temporary compiled migrator approach:

- **`examples/app/models/`** - Product and Category models
- **`examples/app/migrations/`** - Generated migrations with GORM struct definitions
- **`examples/app/goosegorm.yml`** - Configuration using SQLite file database

To test the example app:
```bash
cd examples/app
goosegorm makemigrations  # Generates migration files only
goosegorm migrate         # Creates temporary migrator, runs migrations, cleans up
```

You can explore these examples to see how migrations are structured and how the temporary compiled migrator approach works with real GORM struct definitions. Notice that no permanent migrator files are created - the migrator is generated temporarily during `migrate` and automatically deleted afterward.

## Troubleshooting

### "No migration files found" Error

If you see this error when running `goosegorm build` or `goosegorm migrate`:

```
✗ No migration files found in: ./migrations
ℹ Please run 'goosegorm makemigrations' first to create initial migrations
```

**Solution:** Run `goosegorm makemigrations` first to generate migration files from your models.

### PostgreSQL Reserved Keywords

GooseGORM automatically quotes SQL identifiers to handle PostgreSQL reserved keywords like `user`, `order`, etc. The generated migrations will properly escape quotes in SQL strings:

```go
// Generated code automatically handles reserved keywords
db.Exec("CREATE INDEX IF NOT EXISTS idx_email ON \"user\" (\"email\")")
```

### Custom Table Names Not Working

Make sure your `TableName()` method returns a string literal or a constant:

```go
// ✅ Works
func (User) TableName() string {
    return "custom_users"
}

// ✅ Works (with constant)
const UserTableName = "custom_users"
func (User) TableName() string {
    return UserTableName
}

// ❌ Doesn't work (complex expressions)
func (User) TableName() string {
    return fmt.Sprintf("prefix_%s", "users")
}
```

## License

MIT
