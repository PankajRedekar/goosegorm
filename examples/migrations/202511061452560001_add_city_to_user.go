package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddCityToUser struct{}

func (m AddCityToUser) Version() string { return "202511061452560001" }

func (m AddCityToUser) Name() string { return "add_city_to_user" }

func (m AddCityToUser) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.AlterTable("user").AddColumnWithOptions("city", "string", false, false, false)
		return nil
	}

	// Real DB mode
	type UserCity struct {
		City string `gorm:"not null"`
	}
	if err := db.Table("user").Migrator().AddColumn(&UserCity{}, "City"); err != nil {
		return err
	}
	return nil
}

func (m AddCityToUser) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode - reverse operations
		sim.AlterTable("user").DropColumn("city")
		return nil
	}

	// Real DB mode - reverse operations
	if err := db.Migrator().DropColumn("user", "City"); err != nil {
		return err
	}
	return nil
}

func init() {
	goosegorm.RegisterMigration(AddCityToUser{})
}
