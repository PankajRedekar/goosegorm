package models

import (
	"time"

	"gorm.io/gorm"
)

// Category represents a product category
type Category struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"not null;uniqueIndex;size:100" json:"name"`
	Slug      string         `gorm:"uniqueIndex;size:100" json:"slug"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName returns the table name for Category
func (Category) TableName() string {
	return "categories"
}
