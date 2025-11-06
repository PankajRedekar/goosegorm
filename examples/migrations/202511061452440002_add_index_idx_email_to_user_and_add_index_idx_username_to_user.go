package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddIndexIdxEmailToUserAndAddIndexIdxUsernameToUser struct{}

func (m AddIndexIdxEmailToUserAndAddIndexIdxUsernameToUser) Version() string { return "202511061452440002" }

func (m AddIndexIdxEmailToUserAndAddIndexIdxUsernameToUser) Name() string { return "add_index_idx_email_to_user_and_add_index_idx_username_to_user" }

func (m AddIndexIdxEmailToUserAndAddIndexIdxUsernameToUser) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.AlterTable("user").AddIndex("idx_email")
		sim.AlterTable("user").AddIndex("idx_username")
		return nil
	}

	// Real DB mode
	// Create index idx_email on user (email)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_email ON user (email)").Error; err != nil {
		return err
	}
	// Create index idx_username on user (username)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_username ON user (username)").Error; err != nil {
		return err
	}
	return nil
}

func (m AddIndexIdxEmailToUserAndAddIndexIdxUsernameToUser) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode - reverse operations
		sim.AlterTable("user").DropIndex("idx_username")
		sim.AlterTable("user").DropIndex("idx_email")
		return nil
	}

	// Real DB mode - reverse operations
	if err := db.Exec("DROP INDEX IF EXISTS idx_username").Error; err != nil {
		return err
	}
	if err := db.Exec("DROP INDEX IF EXISTS idx_email").Error; err != nil {
		return err
	}
	return nil
}

func init() {
	goosegorm.RegisterMigration(AddIndexIdxEmailToUserAndAddIndexIdxUsernameToUser{})
}
