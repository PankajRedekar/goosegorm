package migrations

import (
	"gorm.io/gorm"
	"github.com/pankajredekar/goosegorm"
)

type AddIndexIdxSkuToProduct struct{}

func (m AddIndexIdxSkuToProduct) Version() string { return "202511061555330002" }

func (m AddIndexIdxSkuToProduct) Name() string { return "add_index_idx_sku_to_product" }

func (m AddIndexIdxSkuToProduct) Up(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode
		sim.AlterTable("product").AddIndex("idx_sku")
		return nil
	}

	// Real DB mode
	// Create index idx_sku on product (sku)
	if err := db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_sku ON product (sku)").Error; err != nil {
		return err
	}
	return nil
}

func (m AddIndexIdxSkuToProduct) Down(db *gorm.DB) error {
	if sim, ok := any(db).(*goosegorm.SchemaBuilder); ok {
		// Simulation mode - reverse operations
		sim.AlterTable("product").DropIndex("idx_sku")
		return nil
	}

	// Real DB mode - reverse operations
	if err := db.Exec("DROP INDEX IF EXISTS idx_sku").Error; err != nil {
		return err
	}
	return nil
}

func init() {
	goosegorm.RegisterMigration(AddIndexIdxSkuToProduct{})
}
