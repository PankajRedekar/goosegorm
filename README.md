# GooseGORM

Django-style migration framework for GORM — pure Go, no SQL files, no external dependencies.

## Features

- ✅ Go-based migration files (no SQL)
- ✅ In-memory schema simulation (SchemaBuilder)
- ✅ Model-level control with `goosegorm:"managed:false"`
- ✅ Automatic migration generation from model changes
- ✅ Version tracking in database
- ✅ Rollback support

## Installation

Install the GooseGORM CLI tool:

```bash
go install github.com/pankajredekar/goosegorm/cmd/goosegorm@latest
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

## Configuration (goosegorm.yml)

```yaml
database_url: postgres://user:pass@localhost:5432/myapp?sslmode=disable
models_dir: ./models
migrations_dir: ./migrations
package_name: migrations
migration_table: _goosegorm_migrations
ignore_models: []
```

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
    if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
        sim.AlterTable("users").AddColumn("email", "string")
        return nil
    }
    return db.Migrator().AddColumn(&User{}, "Email")
}

func (m AddEmailToUsers) Down(db *gorm.DB) error {
    if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
        sim.AlterTable("users").DropColumn("email")
        return nil
    }
    return db.Migrator().DropColumn(&User{}, "Email")
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

## License

MIT
