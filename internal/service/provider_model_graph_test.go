package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"
	"testing"

	"cpa-usage-keeper/internal/cpa/dto/providerconfig"
	"cpa-usage-keeper/internal/cpa/dto/response"
)

type fakeProviderModelGraphClient struct {
	result *response.ManagementConfigResult
	err    error
}

func (f *fakeProviderModelGraphClient) FetchManagementConfig(ctx context.Context) (*response.ManagementConfigResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.result, nil
}

func graphResult(config providerconfig.ManagementConfig) *response.ManagementConfigResult {
	return &response.ManagementConfigResult{StatusCode: 200, Payload: config}
}

func TestGetProviderModelGraphRequiresConfiguredClient(t *testing.T) {
	provider := NewProviderModelGraphService(nil)
	if _, err := provider.GetProviderModelGraph(context.Background()); err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("expected not-configured error, got %v", err)
	}
}

func TestGetProviderModelGraphPassesThroughFetchErrors(t *testing.T) {
	boom := errors.New("cpa unreachable")
	provider := NewProviderModelGraphService(&fakeProviderModelGraphClient{err: boom})
	if _, err := provider.GetProviderModelGraph(context.Background()); !errors.Is(err, boom) {
		t.Fatalf("expected passthrough error, got %v", err)
	}
}

func TestGetProviderModelGraphMergesGeminiEntriesIntoSingleNode(t *testing.T) {
	provider := NewProviderModelGraphService(&fakeProviderModelGraphClient{result: graphResult(providerconfig.ManagementConfig{
		GeminiAPIKey: []providerconfig.ConfigGeminiEntry{
			{Models: []providerconfig.ModelAliasEntry{
				{Name: "gemini-2.5-pro", Alias: "gemini-pro"},
				{Name: "gemini-2.5-flash", Alias: ""},
			}},
			{Models: []providerconfig.ModelAliasEntry{
				{Name: "gemini-3-flash-preview", Alias: "gemini-flash"},
			}},
		},
	})})

	res, err := provider.GetProviderModelGraph(context.Background())
	if err != nil {
		t.Fatalf("GetProviderModelGraph returned error: %v", err)
	}
	if len(res.Providers) != 1 {
		t.Fatalf("expected a single gemini provider node, got %+v", res.Providers)
	}
	node := res.Providers[0]
	if node.Name != "Gemini" || node.Kind != "gemini-api-key" {
		t.Fatalf("unexpected gemini node identity: %+v", node)
	}
	if node.Disabled {
		t.Fatalf("gemini node must not report disabled: %+v", node)
	}
	if len(node.Models) != 3 {
		t.Fatalf("expected 3 merged gemini models, got %+v", node.Models)
	}
	labels := make([]string, 0, len(node.Models))
	for _, model := range node.Models {
		labels = append(labels, model.Label)
	}
	want := []string{"gemini-pro", "gemini-2.5-flash", "gemini-flash"}
	for i, label := range want {
		if labels[i] != label {
			t.Fatalf("unexpected gemini labels: got %v want %v", labels, want)
		}
	}
	if node.Models[0].Name != "gemini-2.5-pro" || node.Models[0].Alias != "gemini-pro" {
		t.Fatalf("expected alias-first model to keep name/alias, got %+v", node.Models[0])
	}
	if node.Models[1].Alias != "" {
		t.Fatalf("expected fallback-to-name model to keep empty alias, got %+v", node.Models[1])
	}
}

func TestGetProviderModelGraphMapsOpenAICompatibilityEntries(t *testing.T) {
	provider := NewProviderModelGraphService(&fakeProviderModelGraphClient{result: graphResult(providerconfig.ManagementConfig{
		OpenAICompatibility: []providerconfig.ConfigOpenAICompatibilityEntry{
			{Name: "Prod Pool", Disabled: true, Models: []providerconfig.ModelAliasEntry{{Name: "gpt-4o", Alias: "fast-gpt"}}},
			{Name: "   ", Models: []providerconfig.ModelAliasEntry{{Name: "gpt-4o-mini"}}},
			{Name: "NoModels"},
			{Name: "Backup Pool", Models: []providerconfig.ModelAliasEntry{{Name: "claude-sonnet-4"}}},
		},
	})})

	res, err := provider.GetProviderModelGraph(context.Background())
	if err != nil {
		t.Fatalf("GetProviderModelGraph returned error: %v", err)
	}
	if len(res.Providers) != 2 {
		t.Fatalf("expected blank-name and empty-model entries to be skipped, got %+v", res.Providers)
	}
	first := res.Providers[0]
	if first.Name != "Prod Pool" || first.Kind != "openai-compatibility" || !first.Disabled {
		t.Fatalf("unexpected first openai-compat node: %+v", first)
	}
	if len(first.Models) != 1 || first.Models[0].Label != "fast-gpt" || first.Models[0].Name != "gpt-4o" {
		t.Fatalf("unexpected first openai-compat models: %+v", first.Models)
	}
	second := res.Providers[1]
	if second.Name != "Backup Pool" || second.Disabled {
		t.Fatalf("unexpected second openai-compat node: %+v", second)
	}
	if len(second.Models) != 1 || second.Models[0].Label != "claude-sonnet-4" || second.Models[0].Alias != "" {
		t.Fatalf("expected label fallback to name when alias empty, got %+v", second.Models)
	}
}

func TestGetProviderModelGraphSortsOAuthProvidersAndSkipsEmpty(t *testing.T) {
	provider := NewProviderModelGraphService(&fakeProviderModelGraphClient{result: graphResult(providerconfig.ManagementConfig{
		OAuthModelAlias: map[string][]providerconfig.ModelAliasEntry{
			"qwen":   {{Name: "qwen3-coder-plus"}},
			"kiro":   {},
			"antigravity": {
				{Name: "gemini-2.5-pro", Alias: "ag-pro"},
				{Name: "claude-opus-4-5"},
			},
		},
	})})

	res, err := provider.GetProviderModelGraph(context.Background())
	if err != nil {
		t.Fatalf("GetProviderModelGraph returned error: %v", err)
	}
	if len(res.Providers) != 2 {
		t.Fatalf("expected empty oauth provider to be skipped, got %+v", res.Providers)
	}
	names := []string{res.Providers[0].Name, res.Providers[1].Name}
	if !sort.StringsAreSorted(names) {
		t.Fatalf("expected oauth providers sorted alphabetically, got %v", names)
	}
	for _, node := range res.Providers {
		if node.Kind != "oauth-model-alias" {
			t.Fatalf("expected oauth-model-alias kind, got %+v", node)
		}
	}
	if names[0] != "antigravity" || names[1] != "qwen" {
		t.Fatalf("unexpected oauth ordering: %v", names)
	}
	if len(res.Providers[0].Models) != 2 || res.Providers[0].Models[0].Label != "ag-pro" {
		t.Fatalf("unexpected antigravity models: %+v", res.Providers[0].Models)
	}
}

func TestGetProviderModelGraphDedupesModelsByLabelWithinNode(t *testing.T) {
	provider := NewProviderModelGraphService(&fakeProviderModelGraphClient{result: graphResult(providerconfig.ManagementConfig{
		OpenAICompatibility: []providerconfig.ConfigOpenAICompatibilityEntry{
			{Name: "Pool", Models: []providerconfig.ModelAliasEntry{
				{Name: "gpt-4o", Alias: "shared"},
				{Name: "gpt-4o-2024-08-06", Alias: "shared"},
				{Name: "dup-name"},
				{Name: "dup-name"},
				{Name: "  ", Alias: "  "},
			}},
		},
	})})

	res, err := provider.GetProviderModelGraph(context.Background())
	if err != nil {
		t.Fatalf("GetProviderModelGraph returned error: %v", err)
	}
	if len(res.Providers) != 1 {
		t.Fatalf("expected single provider, got %+v", res.Providers)
	}
	models := res.Providers[0].Models
	if len(models) != 2 {
		t.Fatalf("expected duplicate labels and blank entries removed, got %+v", models)
	}
	if models[0].Label != "shared" || models[0].Name != "gpt-4o" {
		t.Fatalf("expected first-seen entry to win the dedupe, got %+v", models[0])
	}
	if models[1].Label != "dup-name" {
		t.Fatalf("expected name-only dedupe entry, got %+v", models[1])
	}
}

func TestGetProviderModelGraphIncludesGNNEmbeddings(t *testing.T) {
	provider := NewProviderModelGraphService(&fakeProviderModelGraphClient{result: graphResult(providerconfig.ManagementConfig{
		OpenAICompatibility: []providerconfig.ConfigOpenAICompatibilityEntry{
			{Name: "A", Models: []providerconfig.ModelAliasEntry{{Name: "m1"}, {Name: "m2"}}},
			{Name: "B", Models: []providerconfig.ModelAliasEntry{{Name: "m1"}}},
		},
	})})

	res, err := provider.GetProviderModelGraph(context.Background())
	if err != nil {
		t.Fatalf("GetProviderModelGraph returned error: %v", err)
	}
	if res.Graph.Meta.ProviderCount != 2 || res.Graph.Meta.ModelCount != 2 || res.Graph.Meta.EdgeCount != 3 {
		t.Fatalf("unexpected GNN meta: %+v", res.Graph.Meta)
	}
	if res.Graph.Meta.FeatureDim != 6 || res.Graph.Meta.HiddenDim != 6 {
		t.Fatalf("unexpected GNN feature dims: %+v", res.Graph.Meta)
	}
	if len(res.Graph.Nodes) != 4 {
		t.Fatalf("expected 4 GNN nodes, got %+v", res.Graph.Nodes)
	}
	if len(res.Graph.Edges) != 3 {
		t.Fatalf("expected 3 GNN edges, got %+v", res.Graph.Edges)
	}
	for _, edge := range res.Graph.Edges {
		if edge.Weight != 1.0/3.0 {
			t.Fatalf("expected normalized edge weight 1/3, got %+v", edge)
		}
		if len(edge.Features.Names) != len(edge.Features.Vector) {
			t.Fatalf("edge features names/vector length mismatch: %+v", edge.Features)
		}
	}
	for _, node := range res.Graph.Nodes {
		if len(node.Features.Names) != len(node.Features.Vector) {
			t.Fatalf("node features names/vector length mismatch: %+v", node.Features)
		}
		emb, ok := res.Graph.Embeddings[node.ID]
		if !ok {
			t.Fatalf("missing GNN embedding for node %s", node.ID)
		}
		if len(emb) != res.Graph.Meta.HiddenDim {
			t.Fatalf("embedding dim mismatch for node %s: %d", node.ID, len(emb))
		}
	}
	// Shared model m1 should have is_shared=1 and higher sharing degree than m2.
	var m1, m2 *ProviderModelGNNNode
	for i, node := range res.Graph.Nodes {
		switch node.ID {
		case "model:m1":
			m1 = &res.Graph.Nodes[i]
		case "model:m2":
			m2 = &res.Graph.Nodes[i]
		}
	}
	if m1 == nil || m2 == nil {
		t.Fatalf("expected model nodes m1 and m2, got %+v", res.Graph.Nodes)
	}
	if m1.Features.Vector[1] != 1 { // is_shared
		t.Fatalf("expected m1 is_shared=1, got %+v", m1.Features)
	}
	if m2.Features.Vector[1] != 0 { // is_shared
		t.Fatalf("expected m2 is_shared=0, got %+v", m2.Features)
	}
	if m1.Features.Vector[2] <= m2.Features.Vector[2] { // sharing_degree_norm
		t.Fatalf("expected m1 sharing degree > m2, got m1=%+v m2=%+v", m1.Features, m2.Features)
	}
}

func TestGetProviderModelGraphSerializesOnlySanitizedFields(t *testing.T) {
	provider := NewProviderModelGraphService(&fakeProviderModelGraphClient{result: graphResult(providerconfig.ManagementConfig{
		OpenAICompatibility: []providerconfig.ConfigOpenAICompatibilityEntry{
			{Name: "Pool", Models: []providerconfig.ModelAliasEntry{
				{Name: "gpt-4o", Alias: "fast"},
				{Name: "gpt-4o-mini"},
			}},
		},
	})})

	res, err := provider.GetProviderModelGraph(context.Background())
	if err != nil {
		t.Fatalf("GetProviderModelGraph returned error: %v", err)
	}
	raw, err := json.Marshal(res)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	providers, ok := decoded["providers"].([]any)
	if !ok || len(providers) != 1 {
		t.Fatalf("expected providers array, got %s", raw)
	}
	node, ok := providers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected provider node object, got %s", raw)
	}
	if node["name"] != "Pool" || node["kind"] != "openai-compatibility" {
		t.Fatalf("unexpected node payload: %s", raw)
	}
	if _, exists := node["disabled"]; exists {
		t.Fatalf("disabled must be omitted when false: %s", raw)
	}
	models, ok := node["models"].([]any)
	if !ok || len(models) != 2 {
		t.Fatalf("expected models array, got %s", raw)
	}
	first, _ := models[0].(map[string]any)
	if first["label"] != "fast" || first["name"] != "gpt-4o" || first["alias"] != "fast" {
		t.Fatalf("unexpected aliased model payload: %s", raw)
	}
	second, _ := models[1].(map[string]any)
	if second["label"] != "gpt-4o-mini" {
		t.Fatalf("expected label fallback to name, got %s", raw)
	}
	if _, exists := second["alias"]; exists {
		t.Fatalf("alias must be omitted when empty: %s", raw)
	}
	body := string(raw)
	for _, secret := range []string{"api-key", "base-url", "headers", "priority"} {
		if strings.Contains(body, secret) {
			t.Fatalf("response must never contain secret field %q: %s", secret, body)
		}
	}
	// GNN graph payload must accompany the sanitized providers.
	graph, ok := decoded["graph"].(map[string]any)
	if !ok {
		t.Fatalf("expected GNN graph in response, got %s", raw)
	}
	nodes, ok := graph["nodes"].([]any)
	if !ok || len(nodes) == 0 {
		t.Fatalf("expected GNN nodes array, got %s", raw)
	}
	for _, n := range nodes {
		nodeMap, ok := n.(map[string]any)
		if !ok {
			t.Fatalf("expected GNN node object, got %s", raw)
		}
		features, ok := nodeMap["features"].(map[string]any)
		if !ok || len(features["names"].([]any)) == 0 || len(features["vector"].([]any)) == 0 {
			t.Fatalf("expected GNN node features with names/vector, got %s", raw)
		}
	}
	edges, ok := graph["edges"].([]any)
	if !ok || len(edges) == 0 {
		t.Fatalf("expected GNN edges array, got %s", raw)
	}
	for _, e := range edges {
		edgeMap, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("expected GNN edge object, got %s", raw)
		}
		if edgeMap["weight"].(float64) <= 0 {
			t.Fatalf("expected positive edge weight, got %s", raw)
		}
	}
	embeddings, ok := graph["embeddings"].(map[string]any)
	if !ok || len(embeddings) == 0 {
		t.Fatalf("expected GNN embeddings map, got %s", raw)
	}
	meta, ok := graph["meta"].(map[string]any)
	if !ok || meta["provider_count"].(float64) == 0 || meta["model_count"].(float64) == 0 {
		t.Fatalf("expected GNN meta counts, got %s", raw)
	}
}
