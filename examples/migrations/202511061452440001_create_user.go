package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
	"time"
)

type CreateUser struct{}

func (m CreateUser) Version() string { return "202511061452440001" }

func (m CreateUser) Name() string { return "create_user" }

func (m CreateUser) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.CreateTable("user").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("email", "string", false, false, true).
			AddColumnWithOptions("username", "string", false, false, true).
			AddColumnWithOptions("password", "string", false, false, false).
			AddColumnWithOptions("first_name", "string", false, false, false).
			AddColumnWithOptions("last_name", "string", false, false, false).
			AddColumnWithOptions("is_active", "bool", false, false, false).
			AddColumnWithOptions("created_at", "timestamp", false, false, false).
			AddColumnWithOptions("var", "string", false, false, false).
			AddColumnWithOptions("updated_at", "timestamp", false, false, false).
			AddColumnWithOptions("deleted_at", "string", false, false, false)
		
		return nil
	}

	// Real DB mode
	type User struct {
		Id uint `gorm:"primaryKey;not null"`
		Email string `gorm:"uniqueIndex;not null"`
		Username string `gorm:"uniqueIndex;not null"`
		Password string `gorm:"not null"`
		FirstName string `gorm:"not null"`
		LastName string `gorm:"not null"`
		IsActive bool `gorm:"not null"`
		CreatedAt time.Time `gorm:"not null"`
		Var string `gorm:"not null"`
		UpdatedAt time.Time `gorm:"not null"`
		DeletedAt string `gorm:"not null"`
	}
	if err := db.Table("user").AutoMigrate(&User{}); err != nil {
		return err
	}
	return nil
}

func (m CreateUser) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode - reverse operations
		sim.DropTable("user")
		return nil
	}

	// Real DB mode - reverse operations
	if err := db.Migrator().DropTable("user"); err != nil {
		return err
	}
	return nil
}

func init() {
	goosegorm.RegisterMigration(CreateUser{})
}
