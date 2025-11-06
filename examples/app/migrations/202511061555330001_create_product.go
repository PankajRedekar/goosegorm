package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
	"time"
)

type CreateProduct struct{}

func (m CreateProduct) Version() string { return "202511061555330001" }

func (m CreateProduct) Name() string { return "create_product" }

func (m CreateProduct) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.CreateTable("product").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("name", "string", false, false, false).
			AddColumnWithOptions("description", "string", false, false, false).
			AddColumnWithOptions("price", "float", false, false, false).
			AddColumnWithOptions("stock", "bigint", false, false, false).
			AddColumnWithOptions("sku", "string", false, false, true).
			AddColumnWithOptions("created_at", "timestamp", false, false, false).
			AddColumnWithOptions("updated_at", "timestamp", false, false, false).
			AddColumnWithOptions("deleted_at", "string", false, false, false)
		
		return nil
	}

	// Real DB mode
	type Product struct {
		Id uint `gorm:"primaryKey;not null"`
		Name string `gorm:"not null"`
		Description string `gorm:"not null"`
		Price float64 `gorm:"not null"`
		Stock int64 `gorm:"not null"`
		Sku string `gorm:"uniqueIndex;not null"`
		CreatedAt time.Time `gorm:"not null"`
		UpdatedAt time.Time `gorm:"not null"`
		DeletedAt string `gorm:"not null"`
	}
	if err := db.Table("product").AutoMigrate(&Product{}); err != nil {
		return err
	}
	return nil
}

func (m CreateProduct) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode - reverse operations
		sim.DropTable("product")
		return nil
	}

	// Real DB mode - reverse operations
	if err := db.Migrator().DropTable("product"); err != nil {
		return err
	}
	return nil
}

func init() {
	goosegorm.RegisterMigration(CreateProduct{})
}
