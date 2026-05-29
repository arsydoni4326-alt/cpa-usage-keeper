package helper

import (
	"strings"

	"cpa-usage-keeper/internal/entities"
)

// CPAAPIKeyDisplayLabel 返回 CPA API Key 的安全展示名：优先别名，其次使用已脱敏 key。
func CPAAPIKeyDisplayLabel(row entities.CPAAPIKey) string {
	label := row.DisplayKey
	if strings.TrimSpace(row.KeyAlias) != "" {
		label = strings.TrimSpace(row.KeyAlias)
	}
	return label
}
