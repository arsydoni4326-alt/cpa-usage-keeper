package test

import (
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/repository/migration"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const authSessionSourceMigrationVersion = "20260701_add_auth_session_source"

func Test20260701AuthSessionSourceMigrationAddsSourceToExistingAuthSessions(t *testing.T) {
	db := openAuthSessionSourceMigrationDatabase(t)
	defer closeAuthSessionSourceMigrationDatabase(t, db)

	if err := db.Exec(`CREATE TABLE schema_migrations (version TEXT PRIMARY KEY, applied_at DATETIME NOT NULL)`).Error; err != nil {
		t.Fatalf("create schema_migrations: %v", err)
	}
	seedSchemaMigrationsBefore(t, db, authSessionSourceMigrationVersion, "2026-06-29T00:00:00Z")
	if err := db.Exec(`CREATE TABLE auth_sessions (
		token_hash TEXT PRIMARY KEY,
		role TEXT NOT NULL,
		cpa_api_key_id INTEGER,
		expires_at DATETIME NOT NULL,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("create legacy auth_sessions: %v", err)
	}
	if err := db.Exec(
		"INSERT INTO auth_sessions (token_hash, role, cpa_api_key_id, expires_at, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		"legacy-token-hash",
		"admin",
		0,
		"2026-07-02T00:00:00Z",
		"2026-07-01T00:00:00Z",
		"2026-07-01T00:00:00Z",
	).Error; err != nil {
		t.Fatalf("seed legacy auth session: %v", err)
	}

	if err := migration.Run(db); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if !db.Migrator().HasColumn(&entities.AuthSession{}, "Source") {
		t.Fatal("expected auth_sessions.source column to exist after migration")
	}
	var source string
	if err := db.Table("auth_sessions").Select("source").Where("token_hash = ?", "legacy-token-hash").Scan(&source).Error; err != nil {
		t.Fatalf("load migrated source: %v", err)
	}
	if source != "standard" {
		t.Fatalf("expected legacy session source to backfill to standard, got %q", source)
	}
	var count int64
	if err := db.Table("schema_migrations").Where("version = ?", authSessionSourceMigrationVersion).Count(&count).Error; err != nil {
		t.Fatalf("count auth session source migration: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected migration %s to be recorded once, got %d", authSessionSourceMigrationVersion, count)
	}
}

func openAuthSessionSourceMigrationDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "auth-session-source.db")), &gorm.Config{NowFunc: func() time.Time {
		return time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	}})
	if err != nil {
		t.Fatalf("open sqlite database: %v", err)
	}
	return db
}

func closeAuthSessionSourceMigrationDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sqlite database: %v", err)
	}
}
