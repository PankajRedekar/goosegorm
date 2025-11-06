package versioner

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	return db
}

func TestNewVersioner(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")
	if ver == nil {
		t.Fatal("NewVersioner returned nil")
	}
	if ver.table != "_test_migrations" {
		t.Errorf("Expected table name '_test_migrations', got '%s'", ver.table)
	}
}

func TestInitialize(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")

	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Check if table exists by trying to query it
	var count int64
	if err := db.Table("_test_migrations").Count(&count).Error; err != nil {
		t.Fatalf("Table should exist: %v", err)
	}
}

func TestRecordApplied(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	err := ver.RecordApplied("20250101000000", "test_migration")
	if err != nil {
		t.Fatalf("RecordApplied failed: %v", err)
	}

	// Verify record exists
	applied, err := ver.IsApplied("20250101000000")
	if err != nil {
		t.Fatalf("IsApplied failed: %v", err)
	}
	if !applied {
		t.Error("Migration should be marked as applied")
	}
}

func TestIsApplied(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Should not be applied initially
	applied, err := ver.IsApplied("20250101000000")
	if err != nil {
		t.Fatalf("IsApplied failed: %v", err)
	}
	if applied {
		t.Error("Migration should not be applied initially")
	}

	// Record it
	if err := ver.RecordApplied("20250101000000", "test"); err != nil {
		t.Fatalf("RecordApplied failed: %v", err)
	}

	// Should now be applied
	applied, err = ver.IsApplied("20250101000000")
	if err != nil {
		t.Fatalf("IsApplied failed: %v", err)
	}
	if !applied {
		t.Error("Migration should be applied now")
	}
}

func TestGetAppliedVersions(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Record multiple migrations
	versions := []string{"20250101000000", "20250102000000", "20250103000000"}
	for _, v := range versions {
		if err := ver.RecordApplied(v, "test_"+v); err != nil {
			t.Fatalf("RecordApplied failed: %v", err)
		}
	}

	applied, err := ver.GetAppliedVersions()
	if err != nil {
		t.Fatalf("GetAppliedVersions failed: %v", err)
	}

	if len(applied) != len(versions) {
		t.Errorf("Expected %d applied versions, got %d", len(versions), len(applied))
	}

	// Check order (should be ascending)
	for i, v := range applied {
		if v != versions[i] {
			t.Errorf("Expected version '%s' at index %d, got '%s'", versions[i], i, v)
		}
	}
}

func TestRemoveApplied(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Record migration
	if err := ver.RecordApplied("20250101000000", "test"); err != nil {
		t.Fatalf("RecordApplied failed: %v", err)
	}

	// Remove it
	if err := ver.RemoveApplied("20250101000000"); err != nil {
		t.Fatalf("RemoveApplied failed: %v", err)
	}

	// Should not be applied anymore
	applied, err := ver.IsApplied("20250101000000")
	if err != nil {
		t.Fatalf("IsApplied failed: %v", err)
	}
	if applied {
		t.Error("Migration should not be applied after removal")
	}
}

func TestGetLatestVersion(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	// Should return empty string when no migrations
	latest, err := ver.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion failed: %v", err)
	}
	if latest != "" {
		t.Errorf("Expected empty string, got '%s'", latest)
	}

	// Record migrations
	versions := []string{"20250101000000", "20250103000000", "20250102000000"}
	for _, v := range versions {
		if err := ver.RecordApplied(v, "test_"+v); err != nil {
			t.Fatalf("RecordApplied failed: %v", err)
		}
	}

	latest, err = ver.GetLatestVersion()
	if err != nil {
		t.Fatalf("GetLatestVersion failed: %v", err)
	}
	expected := "20250103000000" // Latest by version
	if latest != expected {
		t.Errorf("Expected latest version '%s', got '%s'", expected, latest)
	}
}

func TestGetAppliedCount(t *testing.T) {
	db := setupTestDB(t)
	ver := NewVersioner(db, "_test_migrations")
	if err := ver.Initialize(); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	count, err := ver.GetAppliedCount()
	if err != nil {
		t.Fatalf("GetAppliedCount failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count 0, got %d", count)
	}

	// Record some migrations
	for i := 0; i < 3; i++ {
		version := "2025010100000" + string(rune('0'+i))
		if err := ver.RecordApplied(version, "test"); err != nil {
			t.Fatalf("RecordApplied failed: %v", err)
		}
	}

	count, err = ver.GetAppliedCount()
	if err != nil {
		t.Fatalf("GetAppliedCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Expected count 3, got %d", count)
	}
}
