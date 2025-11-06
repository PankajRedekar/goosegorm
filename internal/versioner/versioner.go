package versioner

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// MigrationRecord represents a migration record in the database
type MigrationRecord struct {
	Version   string    `gorm:"primaryKey;column:version"`
	Name      string    `gorm:"column:name"`
	AppliedAt time.Time `gorm:"column:applied_at"`
}

// TableName returns the table name for the migration record
func (MigrationRecord) TableName() string {
	return "_goosegorm_migrations"
}

// Versioner manages migration version tracking
type Versioner struct {
	db    *gorm.DB
	table string
}

// NewVersioner creates a new versioner
func NewVersioner(db *gorm.DB, tableName string) *Versioner {
	return &Versioner{
		db:    db,
		table: tableName,
	}
}

// Initialize creates the migration tracking table
func (v *Versioner) Initialize() error {
	if err := v.db.Exec(fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version VARCHAR(255) PRIMARY KEY,
			name VARCHAR(255),
			applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`, v.table)).Error; err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}
	return nil
}

// GetAppliedVersions returns all applied migration versions
func (v *Versioner) GetAppliedVersions() ([]string, error) {
	var records []MigrationRecord
	if err := v.db.Table(v.table).Order("version ASC").Find(&records).Error; err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}

	versions := make([]string, len(records))
	for i, r := range records {
		versions[i] = r.Version
	}
	return versions, nil
}

// IsApplied checks if a migration version is already applied
func (v *Versioner) IsApplied(version string) (bool, error) {
	var count int64
	if err := v.db.Table(v.table).Where("version = ?", version).Count(&count).Error; err != nil {
		return false, fmt.Errorf("failed to check migration status: %w", err)
	}
	return count > 0, nil
}

// RecordApplied records a migration as applied
func (v *Versioner) RecordApplied(version, name string) error {
	record := MigrationRecord{
		Version:   version,
		Name:      name,
		AppliedAt: time.Now(),
	}
	if err := v.db.Table(v.table).Create(&record).Error; err != nil {
		return fmt.Errorf("failed to record migration: %w", err)
	}
	return nil
}

// RemoveApplied removes a migration record (for rollback)
func (v *Versioner) RemoveApplied(version string) error {
	if err := v.db.Table(v.table).Where("version = ?", version).Delete(&MigrationRecord{}).Error; err != nil {
		return fmt.Errorf("failed to remove migration record: %w", err)
	}
	return nil
}

// GetLatestVersion returns the latest applied migration version
func (v *Versioner) GetLatestVersion() (string, error) {
	var record MigrationRecord
	if err := v.db.Table(v.table).Order("version DESC").First(&record).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return "", nil
		}
		return "", fmt.Errorf("failed to get latest version: %w", err)
	}
	return record.Version, nil
}

// GetAppliedCount returns the number of applied migrations
func (v *Versioner) GetAppliedCount() (int64, error) {
	var count int64
	if err := v.db.Table(v.table).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("failed to count applied migrations: %w", err)
	}
	return count, nil
}
