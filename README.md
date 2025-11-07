# GooseGORM

Django-style migration framework for GORM — pure Go, no SQL files, no external dependencies.

## Features

- ✅ Go-based migration files (no SQL)
- ✅ In-memory schema simulation (SchemaBuilder)
- ✅ Model-level control with `goosegorm:"managed:false"`
- ✅ Automatic migration generation from model changes
- ✅ Temporary compiled migrator for reliable execution (auto-generated and cleaned up)
- ✅ Version tracking in database
- ✅ Rollback support
- ✅ Full GORM struct support (real compiled code, not AST interpretation)

## Installation

Install the GooseGORM CLI tool:

```bash
# Install latest version
go install github.com/pankajredekar/goosegorm/cmd/goosegorm@latest

# Or install a specific version
go install github.com/pankajredekar/goosegorm/cmd/goosegorm@v0.1.0
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
       Email string
   }
   ```

3. **Generate migrations:**
   ```bash
   goosegorm makemigrations
   ```

4. **Apply migrations:**
   ```bash
   goosegorm migrate
   ```
   
   This will:
   - Create a temporary migrator package (`.goosegorm_migrator`)
   - Build the migrator binary
   - Run the compiled migrator to execute migrations
   - Automatically clean up the temporary package
   - Execute migrations using real compiled code with proper struct definitions

## Configuration (goosegorm.yml)

```yaml
database_url: sqlite://:memory:  # or postgres://user:pass@localhost:5432/myapp?sslmode=disable
models_dir: ./models
migrations_dir: ./migrations
package_name: migrations
migration_table: _goosegorm_migrations
ignore_models: []
```

**Note:** The `main_pkg_path` option is no longer used. Migrations are executed using a temporary compiled migrator that is automatically created and cleaned up during the `migrate` command.

**Supported databases:**
- SQLite: `sqlite://:memory:` (in-memory) or `sqlite://./db.sqlite` (file)
- PostgreSQL: `postgres://user:pass@localhost:5432/myapp?sslmode=disable`

## Model-Level Control

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

## CLI Commands

- `goosegorm init` - Initialize project
- `goosegorm makemigrations` - Generate migration files
- `goosegorm migrate` - Apply pending migrations
- `goosegorm rollback [n]` - Rollback last N migrations
- `goosegorm show` - Show migration status

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

## License

MIT
