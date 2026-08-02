package providerconfig

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ModelAliasEntry 是 /v0/management/config 中 provider 模型条目的最小视图，只保留 name 与 alias。
// label() 遵循 alias 优先规则：alias 为空时回退到 name。
type ModelAliasEntry struct {
	Name  string
	Alias string
}

func (m *ModelAliasEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode model alias entry: %w", err)
	}
	m.Name = firstString(raw, "name", "model", "id")
	m.Alias = firstString(raw, "alias")
	return nil
}

// Label 返回展示名：优先 alias，alias 为空时回退 name。
func (m ModelAliasEntry) Label() string {
	if trimmed := strings.TrimSpace(m.Alias); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(m.Name)
}

// ConfigGeminiEntry 是 gemini-api-key 配置的模型视图，忽略 api-key 与 excluded-models。
type ConfigGeminiEntry struct {
	Models []ModelAliasEntry
}

func (c *ConfigGeminiEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode gemini config entry: %w", err)
	}
	models, err := decodeModelAliasEntries(raw["models"])
	if err != nil {
		return err
	}
	c.Models = models
	return nil
}

// ConfigOpenAICompatibilityEntry 是 openai-compatibility 配置的模型视图，保留 name 与 disabled。
type ConfigOpenAICompatibilityEntry struct {
	Name     string
	Disabled bool
	Models   []ModelAliasEntry
}

func (c *ConfigOpenAICompatibilityEntry) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode openai compatibility config entry: %w", err)
	}
	c.Name = firstString(raw, "name", "id")
	if disabled := firstBool(raw, "disabled"); disabled != nil {
		c.Disabled = *disabled
	}
	models, err := decodeModelAliasEntries(raw["models"])
	if err != nil {
		return err
	}
	c.Models = models
	return nil
}

// ConfigOAuthProvider 是 oauth-model-alias 映射中单个 provider 的 模型别名列表，保留 fork 标记。
type ConfigOAuthProvider struct {
	Name   string
	Models []ModelAliasEntry
}

func decodeModelAliasEntries(value any) ([]ModelAliasEntry, error) {
	rawEntries, ok := value.([]any)
	if !ok {
		return nil, nil
	}
	entries := make([]ModelAliasEntry, 0, len(rawEntries))
	for _, rawEntry := range rawEntries {
		rawMap, ok := rawEntry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unsupported model alias entry type %T", rawEntry)
		}
		entry := ModelAliasEntry{
			Name:  firstString(rawMap, "name", "model", "id"),
			Alias: firstString(rawMap, "alias"),
		}
		if entry.Name == "" && entry.Alias == "" {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// ManagementConfig 是 /v0/management/config 响应的归一化视图，只保留 provider 与模型别名关系。
// 任何 secrets（api-key、base-url、headers 等）都不会被解析到这个结构中。
type ManagementConfig struct {
	GeminiAPIKey        []ConfigGeminiEntry
	OpenAICompatibility []ConfigOpenAICompatibilityEntry
	OAuthModelAlias     map[string][]ModelAliasEntry
}

func decodeConfigGeminiEntries(value any) ([]ConfigGeminiEntry, error) {
	rawEntries, ok := value.([]any)
	if !ok {
		return nil, nil
	}
	entries := make([]ConfigGeminiEntry, 0, len(rawEntries))
	for _, rawEntry := range rawEntries {
		rawMap, ok := rawEntry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unsupported gemini config entry type %T", rawEntry)
		}
		models, err := decodeModelAliasEntries(rawMap["models"])
		if err != nil {
			return nil, err
		}
		if len(models) == 0 {
			continue
		}
		entries = append(entries, ConfigGeminiEntry{Models: models})
	}
	return entries, nil
}

func decodeConfigOpenAICompatibilityEntries(value any) ([]ConfigOpenAICompatibilityEntry, error) {
	rawEntries, ok := value.([]any)
	if !ok {
		return nil, nil
	}
	entries := make([]ConfigOpenAICompatibilityEntry, 0, len(rawEntries))
	for _, rawEntry := range rawEntries {
		rawMap, ok := rawEntry.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("unsupported openai compatibility entry type %T", rawEntry)
		}
		models, err := decodeModelAliasEntries(rawMap["models"])
		if err != nil {
			return nil, err
		}
		if len(models) == 0 {
			continue
		}
		entry := ConfigOpenAICompatibilityEntry{
			Name:   firstString(rawMap, "name", "id"),
			Models: models,
		}
		if disabled := firstBool(rawMap, "disabled"); disabled != nil {
			entry.Disabled = *disabled
		}
		if strings.TrimSpace(entry.Name) == "" {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func decodeOAuthModelAlias(value any) (map[string][]ModelAliasEntry, error) {
	rawMap, ok := value.(map[string]any)
	if !ok {
		return nil, nil
	}
	result := make(map[string][]ModelAliasEntry, len(rawMap))
	for provider, rawValue := range rawMap {
		models, err := decodeModelAliasEntries(rawValue)
		if err != nil {
			return nil, err
		}
		if len(models) == 0 {
			continue
		}
		result[provider] = models
	}
	return result, nil
}

func (c *ManagementConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("decode management config: %w", err)
	}
	gemini, err := decodeConfigGeminiEntries(raw["gemini-api-key"])
	if err != nil {
		return err
	}
	openaiCompat, err := decodeConfigOpenAICompatibilityEntries(raw["openai-compatibility"])
	if err != nil {
		return err
	}
	oauthAlias, err := decodeOAuthModelAlias(raw["oauth-model-alias"])
	if err != nil {
		return err
	}
	c.GeminiAPIKey = gemini
	c.OpenAICompatibility = openaiCompat
	c.OAuthModelAlias = oauthAlias
	return nil
}
