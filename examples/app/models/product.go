package models

type Product struct {
	ID    uint   `gorm:"primaryKey"`
	Name  string `gorm:"not null"`
	SKU   string `gorm:"uniqueIndex:idx_sku;not null"`
	Price float64
}
