package migration

import (
	"errors"
	"strings"
	"time"

	"cpa-usage-keeper/internal/entities"
	"cpa-usage-keeper/internal/timeutil"
	"gorm.io/gorm"
)

const geminiCodexBackfillOverviewCheckpointName = "overview"
const geminiCodexBackfillEventBatchSize = 500

type geminiCodexBackfillIdentityKey struct {
	authType entities.UsageIdentityAuthType
	identity string
}

func backfillGeminiCodexTokenFormatMigration(tx *gorm.DB) error {
	for _, model := range []any{
		&entities.UsageEvent{},
		&entities.UsageIdentity{},
		&entities.UsageOverviewHourlyStat{},
		&entities.UsageOverviewDailyStat{},
		&entities.UsageOverviewAggregationCheckpoint{},
	} {
		if !tx.Migrator().HasTable(model) {
			return nil
		}
	}

	identityByKey, err := loadGeminiCodexBackfillIdentityLookup(tx)
	if err != nil {
		return err
	}
	overviewLastAggregatedID, err := loadGeminiCodexBackfillOverviewCursor(tx)
	if err != nil {
		return err
	}

	lastEventID := int64(0)
	for {
		events := make([]entities.UsageEvent, 0, geminiCodexBackfillEventBatchSize)
		if err := tx.Where("id > ? AND reasoning_tokens > ?", lastEventID, 0).
			Order("id asc").
			Limit(geminiCodexBackfillEventBatchSize).
			Find(&events).Error; err != nil {
			return err
		}
		if len(events) == 0 {
			return nil
		}
		for _, event := range events {
			key, hasKey := geminiCodexBackfillIdentityKeyForEvent(event)
			identity, hasIdentity := identityByKey[key]
			if !isGeminiCodexBackfillFamilyEvent(event, identity, hasKey && hasIdentity) {
				continue
			}

			outputDelta, totalDelta := geminiCodexBackfillTokenDeltas(event)
			if outputDelta == 0 && totalDelta == 0 {
				continue
			}
			if err := updateGeminiCodexBackfillUsageEvent(tx, event, outputDelta, totalDelta); err != nil {
				return err
			}
			if event.ID <= overviewLastAggregatedID {
				if err := updateGeminiCodexBackfillOverviewStats(tx, event, outputDelta, totalDelta); err != nil {
					return err
				}
			}
			if hasKey && hasIdentity && event.ID <= identity.LastAggregatedUsageEventID {
				identity.OutputTokens += outputDelta
				identity.TotalTokens += totalDelta
				if err := tx.Model(&entities.UsageIdentity{}).
					Where("id = ?", identity.ID).
					Updates(map[string]any{
						"output_tokens": identity.OutputTokens,
						"total_tokens":  identity.TotalTokens,
					}).Error; err != nil {
					return err
				}
				identityByKey[key] = identity
			}
		}
		lastEventID = events[len(events)-1].ID
	}
}

func loadGeminiCodexBackfillIdentityLookup(tx *gorm.DB) (map[geminiCodexBackfillIdentityKey]entities.UsageIdentity, error) {
	var identities []entities.UsageIdentity
	if err := tx.Find(&identities).Error; err != nil {
		return nil, err
	}
	identityByKey := make(map[geminiCodexBackfillIdentityKey]entities.UsageIdentity, len(identities))
	for _, identity := range identities {
		key := geminiCodexBackfillIdentityKey{authType: identity.AuthType, identity: strings.TrimSpace(identity.Identity)}
		if key.identity == "" {
			continue
		}
		identityByKey[key] = identity
	}
	return identityByKey, nil
}

func loadGeminiCodexBackfillOverviewCursor(tx *gorm.DB) (int64, error) {
	var checkpoint entities.UsageOverviewAggregationCheckpoint
	err := tx.Where("name = ?", geminiCodexBackfillOverviewCheckpointName).First(&checkpoint).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return checkpoint.LastAggregatedUsageEventID, nil
}

func geminiCodexBackfillIdentityKeyForEvent(event entities.UsageEvent) (geminiCodexBackfillIdentityKey, bool) {
	authIndex := strings.TrimSpace(event.AuthIndex)
	if authIndex == "" {
		return geminiCodexBackfillIdentityKey{}, false
	}
	switch strings.ToLower(strings.TrimSpace(event.AuthType)) {
	case "oauth":
		return geminiCodexBackfillIdentityKey{authType: entities.UsageIdentityAuthTypeAuthFile, identity: authIndex}, true
	case "apikey", "api_key":
		return geminiCodexBackfillIdentityKey{authType: entities.UsageIdentityAuthTypeAIProvider, identity: authIndex}, true
	default:
		return geminiCodexBackfillIdentityKey{}, false
	}
}

func isGeminiCodexBackfillFamilyEvent(event entities.UsageEvent, identity entities.UsageIdentity, hasIdentity bool) bool {
	if hasIdentity {
		return isGeminiCodexBackfillFamilyType(identity.Type)
	}
	return isGeminiCodexBackfillFamilyType(event.Provider)
}

func isGeminiCodexBackfillFamilyType(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "gemini", "vertex", "gemini-cli", "gemini-cli-code-assist", "antigravity", "aistudio", "ai-studio":
		return true
	default:
		return false
	}
}

func geminiCodexBackfillTokenDeltas(event entities.UsageEvent) (int64, int64) {
	if event.ReasoningTokens <= 0 {
		return 0, 0
	}
	if event.TotalTokens > 0 {
		if event.InputTokens+event.OutputTokens == event.TotalTokens {
			return 0, 0
		}
		if event.InputTokens+event.OutputTokens+event.ReasoningTokens != event.TotalTokens {
			return 0, 0
		}
		return event.ReasoningTokens, 0
	}
	// total 缺失时无法可靠区分 old Gemini output 和已 Codex 化 output；只补 total，避免二次折入 reasoning。
	newTotalTokens := event.InputTokens + event.OutputTokens
	return 0, newTotalTokens - event.TotalTokens
}

func updateGeminiCodexBackfillUsageEvent(tx *gorm.DB, event entities.UsageEvent, outputDelta, totalDelta int64) error {
	return tx.Model(&entities.UsageEvent{}).
		Where("id = ?", event.ID).
		Updates(map[string]any{
			"output_tokens": event.OutputTokens + outputDelta,
			"total_tokens":  event.TotalTokens + totalDelta,
		}).Error
}

func updateGeminiCodexBackfillOverviewStats(tx *gorm.DB, event entities.UsageEvent, outputDelta, totalDelta int64) error {
	key := geminiCodexBackfillOverviewKeyForEvent(event)
	if err := updateGeminiCodexBackfillHourlyStats(tx, key, outputDelta, totalDelta); err != nil {
		return err
	}
	return updateGeminiCodexBackfillDailyStats(tx, key, outputDelta, totalDelta)
}

type geminiCodexBackfillOverviewKey struct {
	HourBucketStart time.Time
	DayBucketStart  time.Time
	APIGroupKey     string
	Model           string
	AuthIndex       string
	ModelAlias      string
}

func geminiCodexBackfillOverviewKeyForEvent(event entities.UsageEvent) geminiCodexBackfillOverviewKey {
	timestamp := timeutil.NormalizeStorageTime(event.Timestamp)
	dayBucket := time.Date(timestamp.Year(), timestamp.Month(), timestamp.Day(), 0, 0, 0, 0, timestamp.Location())
	modelAlias := ""
	if event.ModelAlias != nil {
		modelAlias = normalizeGeminiCodexBackfillOptionalDimension(*event.ModelAlias)
	}
	return geminiCodexBackfillOverviewKey{
		HourBucketStart: timestamp.Truncate(time.Hour),
		DayBucketStart:  dayBucket,
		APIGroupKey:     normalizeGeminiCodexBackfillDimension(event.APIGroupKey),
		Model:           normalizeGeminiCodexBackfillDimension(event.Model),
		AuthIndex:       normalizeGeminiCodexBackfillOptionalDimension(event.AuthIndex),
		ModelAlias:      modelAlias,
	}
}

func normalizeGeminiCodexBackfillDimension(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	return value
}

func normalizeGeminiCodexBackfillOptionalDimension(value string) string {
	return strings.TrimSpace(value)
}

func updateGeminiCodexBackfillHourlyStats(tx *gorm.DB, key geminiCodexBackfillOverviewKey, outputDelta, totalDelta int64) error {
	var row entities.UsageOverviewHourlyStat
	err := tx.Where("bucket_start = ? AND api_group_key = ? AND model = ? AND auth_index = ? AND model_alias = ?",
		timeutil.FormatStorageTime(key.HourBucketStart), key.APIGroupKey, key.Model, key.AuthIndex, key.ModelAlias).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Model(&entities.UsageOverviewHourlyStat{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"output_tokens": row.OutputTokens + outputDelta,
			"total_tokens":  row.TotalTokens + totalDelta,
		}).Error
}

func updateGeminiCodexBackfillDailyStats(tx *gorm.DB, key geminiCodexBackfillOverviewKey, outputDelta, totalDelta int64) error {
	var row entities.UsageOverviewDailyStat
	err := tx.Where("bucket_start = ? AND api_group_key = ? AND model = ? AND auth_index = ? AND model_alias = ?",
		timeutil.FormatStorageTime(key.DayBucketStart), key.APIGroupKey, key.Model, key.AuthIndex, key.ModelAlias).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Model(&entities.UsageOverviewDailyStat{}).
		Where("id = ?", row.ID).
		Updates(map[string]any{
			"output_tokens": row.OutputTokens + outputDelta,
			"total_tokens":  row.TotalTokens + totalDelta,
		}).Error
}
