package test

import (
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/repository/migration"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const modelPriceMultiplierMigrationVersion = "20260702_model_price_multiplier"

func Test20260702ModelPriceMultiplierMigrationAddsDefaultToExistingPricing(t *testing.T) {
	db := openModelPriceMultiplierMigrationDatabase(t)
	defer closeModelPriceMultiplierMigrationDatabase(t, db)

	if err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME NOT NULL)`).Error; err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	seedSchemaMigrationsBefore(t, db, modelPriceMultiplierMigrationVersion, "2026-07-01T00:00:00Z")
	if err := db.Exec(`CREATE TABLE model_price_settings (
		id integer PRIMARY KEY,
		model text,
		pricing_style text NOT NULL DEFAULT 'openai',
		prompt_price_per1_m real,
		completion_price_per1_m real,
		cache_price_per1_m real,
		cache_creation_price_per1_m real NOT NULL DEFAULT 0,
		created_at datetime,
		updated_at datetime
	)`).Error; err != nil {
		t.Fatalf("create legacy model_price_settings table: %v", err)
	}
	if err := db.Exec(`INSERT INTO model_price_settings (
		id,
		model,
		pricing_style,
		prompt_price_per1_m,
		completion_price_per1_m,
		cache_price_per1_m,
		cache_creation_price_per1_m
	) VALUES (?, ?, ?, ?, ?, ?, ?)`, int64(1), "claude-sonnet", "claude", 3.0, 15.0, 0.3, 3.75).Error; err != nil {
		t.Fatalf("seed legacy model price setting: %v", err)
	}

	if err := migration.Run(db); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !db.Migrator().HasColumn("model_price_settings", "price_multiplier") {
		t.Fatal("expected model_price_settings.price_multiplier column to exist after migration")
	}
	var multiplier float64
	if err := db.Table("model_price_settings").Select("price_multiplier").Where("model = ?", "claude-sonnet").Scan(&multiplier).Error; err != nil {
		t.Fatalf("load migrated price multiplier: %v", err)
	}
	if multiplier != 1 {
		t.Fatalf("expected legacy price multiplier to default to 1, got %v", multiplier)
	}
	var count int64
	if err := db.Table("schema_migrations").Where("version = ?", modelPriceMultiplierMigrationVersion).Count(&count).Error; err != nil {
		t.Fatalf("count model price multiplier migration: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected migration %s to be recorded once, got %d", modelPriceMultiplierMigrationVersion, count)
	}
}

func openModelPriceMultiplierMigrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "model-price-multiplier.db")), &gorm.Config{NowFunc: func() time.Time {
		return time.Date(2026, 7, 2, 0, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	return db
}

func closeModelPriceMultiplierMigrationDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sqlite database: %v", err)
	}
}
