package migration

import (
	"path/filepath"
	"testing"
	"time"

	"cpa-usage-keeper/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillGeminiCodexTokenFormatMigrationNormalizesEventsAndAggregates(t *testing.T) {
	db := openGeminiCodexTokenBackfillTestDatabase(t)
	seedGeminiCodexTokenBackfillRows(t, db)

	if err := backfillGeminiCodexTokenFormatMigration(db); err != nil {
		t.Fatalf("backfill Gemini Codex token format: %v", err)
	}
	assertGeminiCodexTokenBackfillRows(t, db)

	if err := backfillGeminiCodexTokenFormatMigration(db); err != nil {
		t.Fatalf("backfill Gemini Codex token format should be idempotent: %v", err)
	}
	assertGeminiCodexTokenBackfillRows(t, db)
}

func openGeminiCodexTokenBackfillTestDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "gemini-codex-token-backfill.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open migration database: %v", err)
	}
	t.Cleanup(func() {
		closeOpenedDatabase(t, db)
	})
	if err := db.AutoMigrate(
		&entities.UsageEvent{},
		&entities.UsageIdentity{},
		&entities.UsageOverviewHourlyStat{},
		&entities.UsageOverviewDailyStat{},
		&entities.UsageOverviewAggregationCheckpoint{},
	); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	return db
}

func seedGeminiCodexTokenBackfillRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.FixedZone("CST", 8*60*60))
	alias := ""
	if err := db.Create([]entities.UsageIdentity{
		{ID: 1, AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "gemini-auth", Type: "gemini", Provider: "Gemini", OutputTokens: 17, ReasoningTokens: 6, TotalTokens: 42, LastAggregatedUsageEventID: 2, CreatedAt: now, UpdatedAt: now},
		{ID: 2, AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "gemini-cli-auth", Type: "gemini-cli", Provider: "Gemini", OutputTokens: 2, ReasoningTokens: 1, TotalTokens: 8, LastAggregatedUsageEventID: 3, CreatedAt: now, UpdatedAt: now},
		{ID: 3, AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "openai-auth", Type: "openai", Provider: "OpenAI", OutputTokens: 7, ReasoningTokens: 3, TotalTokens: 21, LastAggregatedUsageEventID: 4, CreatedAt: now, UpdatedAt: now},
		{ID: 4, AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "openai-gemini-display", Type: "openai", Provider: "Gemini", OutputTokens: 4, ReasoningTokens: 2, TotalTokens: 16, LastAggregatedUsageEventID: 6, CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed usage identities: %v", err)
	}
	if err := db.Create([]entities.UsageEvent{
		{ID: 1, EventKey: "gemini-old", APIGroupKey: "api-key", Provider: "Gemini", AuthType: "apikey", AuthIndex: "gemini-auth", Model: "gemini-pro", ModelAlias: &alias, Timestamp: time.Date(2026, 6, 1, 10, 15, 0, 0, now.Location()), InputTokens: 11, OutputTokens: 7, ReasoningTokens: 3, CachedTokens: 5, TotalTokens: 21, CreatedAt: now},
		{ID: 2, EventKey: "gemini-already-codex", APIGroupKey: "api-key", Provider: "Gemini", AuthType: "apikey", AuthIndex: "gemini-auth", Model: "gemini-pro", ModelAlias: &alias, Timestamp: time.Date(2026, 6, 1, 10, 20, 0, 0, now.Location()), InputTokens: 11, OutputTokens: 10, ReasoningTokens: 3, CachedTokens: 0, TotalTokens: 21, CreatedAt: now},
		{ID: 3, EventKey: "gemini-cli-old", APIGroupKey: "oauth-key", Provider: "Gemini", AuthType: "oauth", AuthIndex: "gemini-cli-auth", Model: "gemini-pro", Timestamp: time.Date(2026, 6, 1, 11, 15, 0, 0, now.Location()), InputTokens: 5, OutputTokens: 2, ReasoningTokens: 1, CachedTokens: 1, TotalTokens: 8, CreatedAt: now},
		{ID: 4, EventKey: "openai-untouched", APIGroupKey: "api-key", Provider: "OpenAI", AuthType: "apikey", AuthIndex: "openai-auth", Model: "gpt-5", Timestamp: time.Date(2026, 6, 1, 10, 30, 0, 0, now.Location()), InputTokens: 11, OutputTokens: 7, ReasoningTokens: 3, TotalTokens: 21, CreatedAt: now},
		{ID: 5, EventKey: "gemini-unaggregated", APIGroupKey: "api-key", Provider: "Gemini", AuthType: "apikey", AuthIndex: "gemini-auth", Model: "gemini-pro", ModelAlias: &alias, Timestamp: time.Date(2026, 6, 1, 10, 40, 0, 0, now.Location()), InputTokens: 10, OutputTokens: 4, ReasoningTokens: 2, TotalTokens: 16, CreatedAt: now},
		{ID: 6, EventKey: "openai-display-gemini", APIGroupKey: "api-key", Provider: "Gemini", AuthType: "apikey", AuthIndex: "openai-gemini-display", Model: "gpt-5", Timestamp: time.Date(2026, 6, 1, 10, 45, 0, 0, now.Location()), InputTokens: 10, OutputTokens: 4, ReasoningTokens: 2, TotalTokens: 16, CreatedAt: now},
		{ID: 7, EventKey: "gemini-missing-total-already-codex", APIGroupKey: "api-key", Provider: "Gemini", AuthType: "apikey", AuthIndex: "gemini-auth", Model: "gemini-pro", ModelAlias: &alias, Timestamp: time.Date(2026, 6, 1, 10, 50, 0, 0, now.Location()), InputTokens: 11, OutputTokens: 10, ReasoningTokens: 3, TotalTokens: 0, CreatedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed usage events: %v", err)
	}
	if err := db.Create([]entities.UsageOverviewHourlyStat{
		{ID: 1, BucketStart: time.Date(2026, 6, 1, 10, 0, 0, 0, now.Location()), APIGroupKey: "api-key", Model: "gemini-pro", AuthIndex: "gemini-auth", ModelAlias: "", OutputTokens: 17, ReasoningTokens: 6, TotalTokens: 42, CreatedAt: now, UpdatedAt: now},
		{ID: 2, BucketStart: time.Date(2026, 6, 1, 11, 0, 0, 0, now.Location()), APIGroupKey: "oauth-key", Model: "gemini-pro", AuthIndex: "gemini-cli-auth", ModelAlias: "", OutputTokens: 2, ReasoningTokens: 1, TotalTokens: 8, CreatedAt: now, UpdatedAt: now},
		{ID: 3, BucketStart: time.Date(2026, 6, 1, 10, 0, 0, 0, now.Location()), APIGroupKey: "api-key", Model: "gpt-5", AuthIndex: "openai-auth", ModelAlias: "", OutputTokens: 7, ReasoningTokens: 3, TotalTokens: 21, CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed hourly stats: %v", err)
	}
	if err := db.Create([]entities.UsageOverviewDailyStat{
		{ID: 1, BucketStart: time.Date(2026, 6, 1, 0, 0, 0, 0, now.Location()), APIGroupKey: "api-key", Model: "gemini-pro", AuthIndex: "gemini-auth", ModelAlias: "", OutputTokens: 17, ReasoningTokens: 6, TotalTokens: 42, CreatedAt: now, UpdatedAt: now},
		{ID: 2, BucketStart: time.Date(2026, 6, 1, 0, 0, 0, 0, now.Location()), APIGroupKey: "oauth-key", Model: "gemini-pro", AuthIndex: "gemini-cli-auth", ModelAlias: "", OutputTokens: 2, ReasoningTokens: 1, TotalTokens: 8, CreatedAt: now, UpdatedAt: now},
		{ID: 3, BucketStart: time.Date(2026, 6, 1, 0, 0, 0, 0, now.Location()), APIGroupKey: "api-key", Model: "gpt-5", AuthIndex: "openai-auth", ModelAlias: "", OutputTokens: 7, ReasoningTokens: 3, TotalTokens: 21, CreatedAt: now, UpdatedAt: now},
	}).Error; err != nil {
		t.Fatalf("seed daily stats: %v", err)
	}
	if err := db.Create(&entities.UsageOverviewAggregationCheckpoint{ID: 1, Name: "overview", LastAggregatedUsageEventID: 3, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatalf("seed overview checkpoint: %v", err)
	}
}

func assertGeminiCodexTokenBackfillRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	assertGeminiBackfillEventTokens(t, db, "gemini-old", 10, 3, 21)
	assertGeminiBackfillEventTokens(t, db, "gemini-already-codex", 10, 3, 21)
	assertGeminiBackfillEventTokens(t, db, "gemini-cli-old", 3, 1, 8)
	assertGeminiBackfillEventTokens(t, db, "openai-untouched", 7, 3, 21)
	assertGeminiBackfillEventTokens(t, db, "gemini-unaggregated", 6, 2, 16)
	assertGeminiBackfillEventTokens(t, db, "openai-display-gemini", 4, 2, 16)
	assertGeminiBackfillEventTokens(t, db, "gemini-missing-total-already-codex", 10, 3, 21)

	assertGeminiBackfillAggregateTokens(t, db, "usage_overview_hourly_stats", 1, 20, 6, 42)
	assertGeminiBackfillAggregateTokens(t, db, "usage_overview_hourly_stats", 2, 3, 1, 8)
	assertGeminiBackfillAggregateTokens(t, db, "usage_overview_hourly_stats", 3, 7, 3, 21)
	assertGeminiBackfillAggregateTokens(t, db, "usage_overview_daily_stats", 1, 20, 6, 42)
	assertGeminiBackfillAggregateTokens(t, db, "usage_overview_daily_stats", 2, 3, 1, 8)
	assertGeminiBackfillAggregateTokens(t, db, "usage_overview_daily_stats", 3, 7, 3, 21)

	assertGeminiBackfillIdentityTokens(t, db, 1, 20, 6, 42)
	assertGeminiBackfillIdentityTokens(t, db, 2, 3, 1, 8)
	assertGeminiBackfillIdentityTokens(t, db, 3, 7, 3, 21)
	assertGeminiBackfillIdentityTokens(t, db, 4, 4, 2, 16)
}

func assertGeminiBackfillEventTokens(t *testing.T, db *gorm.DB, eventKey string, output, reasoning, total int64) {
	t.Helper()
	var row struct {
		OutputTokens    int64
		ReasoningTokens int64
		TotalTokens     int64
	}
	if err := db.Model(&entities.UsageEvent{}).Select("output_tokens", "reasoning_tokens", "total_tokens").Where("event_key = ?", eventKey).Scan(&row).Error; err != nil {
		t.Fatalf("load usage event %s: %v", eventKey, err)
	}
	if row.OutputTokens != output || row.ReasoningTokens != reasoning || row.TotalTokens != total {
		t.Fatalf("unexpected event %s tokens: %+v", eventKey, row)
	}
}

func assertGeminiBackfillAggregateTokens(t *testing.T, db *gorm.DB, table string, id, output, reasoning, total int64) {
	t.Helper()
	var row struct {
		OutputTokens    int64
		ReasoningTokens int64
		TotalTokens     int64
	}
	if err := db.Table(table).Select("output_tokens", "reasoning_tokens", "total_tokens").Where("id = ?", id).Scan(&row).Error; err != nil {
		t.Fatalf("load aggregate %s/%d: %v", table, id, err)
	}
	if row.OutputTokens != output || row.ReasoningTokens != reasoning || row.TotalTokens != total {
		t.Fatalf("unexpected aggregate %s/%d tokens: %+v", table, id, row)
	}
}

func assertGeminiBackfillIdentityTokens(t *testing.T, db *gorm.DB, id, output, reasoning, total int64) {
	t.Helper()
	var row struct {
		OutputTokens    int64
		ReasoningTokens int64
		TotalTokens     int64
	}
	if err := db.Model(&entities.UsageIdentity{}).Select("output_tokens", "reasoning_tokens", "total_tokens").Where("id = ?", id).Scan(&row).Error; err != nil {
		t.Fatalf("load identity %d: %v", id, err)
	}
	if row.OutputTokens != output || row.ReasoningTokens != reasoning || row.TotalTokens != total {
		t.Fatalf("unexpected identity %d tokens: %+v", id, row)
	}
}
