package api

import (
	"strconv"
	"strings"

	"cpa-usage-keeper/internal/models"
	"cpa-usage-keeper/internal/redact"
)

type usageSourceResolver struct {
	authIdentities     map[string]models.UsageIdentity
	providerIdentities map[string]models.UsageIdentity
}

// newUsageSourceResolver 把活跃 usage identity 建成内存索引，供 Credentials 和事件展示快速解析 source。
func newUsageSourceResolver(identities []models.UsageIdentity) usageSourceResolver {
	authIdentities := make(map[string]models.UsageIdentity, len(identities))
	providerIdentities := make(map[string]models.UsageIdentity, len(identities))
	for _, identity := range identities {
		// resolver 索引只收录活跃身份，避免 deleted identity 影响 Credentials 和事件展示解析。
		if identity.IsDeleted {
			continue
		}
		key := strings.TrimSpace(identity.Identity)
		if key == "" {
			continue
		}
		switch identity.AuthType {
		case models.UsageIdentityAuthTypeAuthFile:
			authIdentities[key] = identity
		case models.UsageIdentityAuthTypeAIProvider:
			providerIdentities[key] = identity
		}
	}

	return usageSourceResolver{
		authIdentities:     authIdentities,
		providerIdentities: providerIdentities,
	}
}

type usageSourceResolution struct {
	DisplayName string
	SourceType  string
	SourceKey   string
}

// usageSourceResolutionFromIdentity 从 provider identity 生成前端展示名、类型和稳定 source_key。
func usageSourceResolutionFromIdentity(item models.UsageIdentity, fallbackIdentity string) usageSourceResolution {
	identityType := safeAIProviderDisplayValue(item.Type, fallbackIdentity, "")
	displayName := firstNonEmptyString(
		safeAIProviderDisplayValue(item.Name, fallbackIdentity, ""),
		safeAIProviderDisplayValue(item.Provider, fallbackIdentity, ""),
		identityType,
		redact.APIKeyDisplayName(fallbackIdentity),
	)
	sourceKey := "provider:" + uintToString(item.ID)
	if item.ID == 0 {
		sourceKey = "provider:" + redact.APIKeyDisplayName(fallbackIdentity)
	}
	return usageSourceResolution{
		DisplayName: displayName,
		SourceType:  identityType,
		SourceKey:   sourceKey,
	}
}

// resolve 只在命中活跃 identity 时返回解析结果，Credentials 不展示无效身份的 fallback 数据。
func (r usageSourceResolver) resolve(rawSource string, authIndex string) (usageSourceResolution, bool) {
	// 优先用 API key source 匹配 AI provider identity，确保 provider 展示名和 source_key 稳定。
	normalizedSource := strings.TrimSpace(rawSource)
	if normalizedSource != "" {
		if item, ok := r.providerIdentities[normalizedSource]; ok {
			return usageSourceResolutionFromIdentity(item, normalizedSource), true
		}
	}

	// provider source 没有命中时，再用 oauth/auth file 的 auth_index 解析账号身份。
	normalizedAuthIndex := strings.TrimSpace(authIndex)
	if normalizedAuthIndex != "" {
		if identity, ok := r.authIdentities[normalizedAuthIndex]; ok {
			displayName := firstNonEmptyString(identity.Name, normalizedAuthIndex)
			return usageSourceResolution{
				DisplayName: displayName,
				SourceType:  firstNonEmptyString(identity.Type, identity.Provider),
				SourceKey:   "auth:" + normalizedAuthIndex,
			}, true
		}
	}

	return usageSourceResolution{}, false
}

// uintToString 统一把数据库 ID 转成 source_key 使用的字符串片段。
func uintToString(value uint) string {
	return strconv.FormatUint(uint64(value), 10)
}

// firstNonEmptyString 按优先级返回第一个非空展示字段。
func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}
