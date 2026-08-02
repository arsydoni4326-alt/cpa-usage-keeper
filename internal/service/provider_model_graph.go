package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"cpa-usage-keeper/internal/cpa/dto/providerconfig"
	"cpa-usage-keeper/internal/cpa/dto/response"
)

// ProviderModelGraphClient 抽象 CPA /v0/management/config 拉取，便于测试替换。
type ProviderModelGraphClient interface {
	FetchManagementConfig(ctx context.Context) (*response.ManagementConfigResult, error)
}

// ProviderModelGraphProvider 提供 provider → model label 的脱敏图数据。
type ProviderModelGraphProvider interface {
	GetProviderModelGraph(ctx context.Context) (ProviderModelGraphResponse, error)
}

// ProviderModelGraphModel 是前端图节点的最小模型信息。
type ProviderModelGraphModel struct {
	Name  string `json:"name"`
	Alias string `json:"alias,omitempty"`
	Label string `json:"label"`
}

// ProviderModelGraphNode 聚合单一 provider 的全部模型别名。
type ProviderModelGraphNode struct {
	Name     string                    `json:"name"`
	Kind     string                    `json:"kind"`
	Disabled bool                      `json:"disabled,omitempty"`
	Models   []ProviderModelGraphModel `json:"models"`
}

// ProviderModelGraphResponse 是脱敏后的 provider/model 图数据，绝不包含 api-key、base-url 等 secrets。
type ProviderModelGraphResponse struct {
	Providers []ProviderModelGraphNode `json:"providers"`
}

type providerModelGraphService struct {
	client ProviderModelGraphClient
}

func NewProviderModelGraphService(client ProviderModelGraphClient) ProviderModelGraphProvider {
	return &providerModelGraphService{client: client}
}

func (s *providerModelGraphService) GetProviderModelGraph(ctx context.Context) (ProviderModelGraphResponse, error) {
	if s.client == nil {
		return ProviderModelGraphResponse{}, fmt.Errorf("provider model graph client is not configured")
	}
	result, err := s.client.FetchManagementConfig(ctx)
	if err != nil {
		return ProviderModelGraphResponse{}, err
	}
	return buildProviderModelGraph(result.Payload), nil
}

func buildProviderModelGraph(config providerconfig.ManagementConfig) ProviderModelGraphResponse {
	providers := make([]ProviderModelGraphNode, 0, len(config.OpenAICompatibility)+len(config.OAuthModelAlias)+1)

	if geminiModels := collectGeminiModels(config.GeminiAPIKey); len(geminiModels) > 0 {
		providers = append(providers, ProviderModelGraphNode{
			Name:   "Gemini",
			Kind:   "gemini-api-key",
			Models: geminiModels,
		})
	}

	for _, entry := range config.OpenAICompatibility {
		name := strings.TrimSpace(entry.Name)
		if name == "" || len(entry.Models) == 0 {
			continue
		}
		providers = append(providers, ProviderModelGraphNode{
			Name:     name,
			Kind:     "openai-compatibility",
			Disabled: entry.Disabled,
			Models:   dedupeGraphModels(entry.Models),
		})
	}

	oauthNames := make([]string, 0, len(config.OAuthModelAlias))
	for name := range config.OAuthModelAlias {
		oauthNames = append(oauthNames, name)
	}
	sort.Strings(oauthNames)
	for _, name := range oauthNames {
		models := config.OAuthModelAlias[name]
		if len(models) == 0 {
			continue
		}
		providers = append(providers, ProviderModelGraphNode{
			Name:   name,
			Kind:   "oauth-model-alias",
			Models: dedupeGraphModels(models),
		})
	}

	return ProviderModelGraphResponse{Providers: providers}
}

func collectGeminiModels(entries []providerconfig.ConfigGeminiEntry) []ProviderModelGraphModel {
	all := make([]providerconfig.ModelAliasEntry, 0)
	for _, entry := range entries {
		all = append(all, entry.Models...)
	}
	return dedupeGraphModels(all)
}

func dedupeGraphModels(entries []providerconfig.ModelAliasEntry) []ProviderModelGraphModel {
	seen := make(map[string]struct{}, len(entries))
	models := make([]ProviderModelGraphModel, 0, len(entries))
	for _, entry := range entries {
		label := entry.Label()
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		models = append(models, ProviderModelGraphModel{
			Name:  strings.TrimSpace(entry.Name),
			Alias: strings.TrimSpace(entry.Alias),
			Label: label,
		})
	}
	return models
}
