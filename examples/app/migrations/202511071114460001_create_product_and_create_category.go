package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type CreateProductAndCreateCategory struct{}

func (m CreateProductAndCreateCategory) Version() string { return "202511071114460001" }

func (m CreateProductAndCreateCategory) Name() string { return "create_product_and_create_category" }

func (m CreateProductAndCreateCategory) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.CreateTable("product").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("name", "string", false, false, false).
			AddColumnWithOptions("sku", "string", false, false, true).
			AddColumnWithOptions("price", "float", false, false, false)
		
		sim.CreateTable("category").
			AddColumnWithOptions("id", "bigint", false, true, false).
			AddColumnWithOptions("name", "string", false, false, false).
			AddColumnWithOptions("slug", "string", false, false, true)
		
		return nil
	}

	// Real DB mode
	type Product struct {
		Id uint `gorm:"primaryKey;not null"`
		Name string `gorm:"not null"`
		Sku string `gorm:"uniqueIndex;not null"`
		Price float64 `gorm:"not null"`
	}
	if err := db.Table("product").AutoMigrate(&Product{}); err != nil {
		return err
	}
	type Category struct {
		Id uint `gorm:"primaryKey;not null"`
		Name string `gorm:"not null"`
		Slug string `gorm:"uniqueIndex;not null"`
	}
	if err := db.Table("category").AutoMigrate(&Category{}); err != nil {
		return err
	}
	return nil
}

func (m CreateProductAndCreateCategory) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode - reverse operations
		sim.DropTable("category")
		sim.DropTable("product")
		return nil
	}

	// Real DB mode - reverse operations
	if err := db.Migrator().DropTable("category"); err != nil {
		return err
	}
	if err := db.Migrator().DropTable("product"); err != nil {
		return err
	}
	return nil
}

func init() {
	goosegorm.RegisterMigration(CreateProductAndCreateCategory{})
}
