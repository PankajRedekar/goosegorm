package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddIndexIdxSkuToProductAndAddIndexIdxSlugToCategory struct{}

func (m AddIndexIdxSkuToProductAndAddIndexIdxSlugToCategory) Version() string { return "202511071053150002" }

func (m AddIndexIdxSkuToProductAndAddIndexIdxSlugToCategory) Name() string { return "add_index_idx_sku_to_product_and_add_index_idx_slug_to_category" }

func (m AddIndexIdxSkuToProductAndAddIndexIdxSlugToCategory) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.AlterTable("product").AddIndex("idx_sku")
		sim.AlterTable("category").AddIndex("idx_slug")
		return nil
	}

	// Real DB mode
	// Create index idx_sku on product (sku)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sku ON product (sku)").Error; err != nil {
		return err
	}
	// Create index idx_slug on category (slug)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_slug ON category (slug)").Error; err != nil {
		return err
	}
	return nil
}

func (m AddIndexIdxSkuToProductAndAddIndexIdxSlugToCategory) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode - reverse operations
		sim.AlterTable("category").DropIndex("idx_slug")
		sim.AlterTable("product").DropIndex("idx_sku")
		return nil
	}

	// Real DB mode - reverse operations
	if err := db.Exec("DROP INDEX IF EXISTS idx_slug").Error; err != nil {
		return err
	}
	if err := db.Exec("DROP INDEX IF EXISTS idx_sku").Error; err != nil {
		return err
	}
	return nil
}

func init() {
	goosegorm.RegisterMigration(AddIndexIdxSkuToProductAndAddIndexIdxSlugToCategory{})
}
