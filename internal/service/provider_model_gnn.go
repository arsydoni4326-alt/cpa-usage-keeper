package service

import (
	"context"
	"fmt"
	"hash/fnv"
	"sort"
	"strings"

	"cpa-usage-keeper/internal/cpa/dto/providerconfig"
	"cpa-usage-keeper/internal/cpa/dto/response"
)

// ProviderModelGraphClient 抽象 CPA /v0/management/config 拉取，便于测试替换。
type ProviderModelGraphClient interface {
	FetchManagementConfig(ctx context.Context) (*response.ManagementConfigResult, error)
}

// ProviderModelGNNProvider 提供 provider → model 的脱敏 GNN 图数据（含节点/边特征）。
// 兼容旧的 ProviderModelGraph 调用方：GNN 响应是原图响应的超集。
type ProviderModelGNNProvider interface {
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

// ProviderModelGraphResponse 是脱敏后的 provider/model GNN 图数据，
// 绝不包含 api-key、base-url 等 secrets。GNN 学习得到的节点/边特征位于 Graph 字段中。
type ProviderModelGraphResponse struct {
	Providers []ProviderModelGraphNode `json:"providers"`
	// Graph 保存 GNN 派生的分析结果；前端用于可视化与交互。
	Graph ProviderModelGNNGraph `json:"graph,omitempty"`
}

// ProviderModelGNNFeature 是 GNN 节点特征的命名向量。
// 下标与向量一一对应，便于前端做归一化、着色、悬浮展示等。
type ProviderModelGNNFeature struct {
	Names  []string  `json:"names"`
	Vector []float64 `json:"vector"`
}

// ProviderModelGNNNode 描述一个 GNN 节点及其学习特征。
type ProviderModelGNNNode struct {
	ID       string                  `json:"id"`
	Type     string                  `json:"type"` // "provider" | "model"
	Disabled bool                    `json:"disabled,omitempty"`
	Features ProviderModelGNNFeature `json:"features"`
}

// ProviderModelGNNEdge 描述 provider→model 边及其学习特征。
type ProviderModelGNNEdge struct {
	Source   string                  `json:"source"`
	Target   string                  `json:"target"`
	Weight   float64                 `json:"weight"` // 归一化边权（消息传递强度）
	Disabled bool                    `json:"disabled,omitempty"`
	Features ProviderModelGNNFeature `json:"features"`
}

// ProviderModelGNNGraph 是 GNN 的输出：节点特征、按注意力权重展开的邻接、
// 以及一层消息传递 (mean-aggregation neighbor smoothing) 后的嵌入。
type ProviderModelGNNGraph struct {
	FeatureNames []string                  `json:"feature_names"`
	Nodes        []ProviderModelGNNNode    `json:"nodes"`
	Edges        []ProviderModelGNNEdge    `json:"edges"`
	Embeddings   map[string][]float64      `json:"embeddings,omitempty"`
	Meta         ProviderModelGNNGraphMeta `json:"meta"`
}

// ProviderModelGNNGraphMeta 汇总 GNN 的结构统计，供 UI 头部展示。
type ProviderModelGNNGraphMeta struct {
	ProviderCount int `json:"provider_count"`
	ModelCount    int `json:"model_count"`
	EdgeCount     int `json:"edge_count"`
	FeatureDim    int `json:"feature_dim"`
	HiddenDim     int `json:"hidden_dim"`
}

// 节点特征名（与 ProviderModelGNNFeature.Names / Graph.FeatureNames 对应）。
const (
	gnnFeatDisabled      = "disabled"
	gnnFeatIsShared      = "is_shared"
	gnnFeatDegree        = "degree_norm"
	gnnFeatSharingDegree = "sharing_degree_norm"
	gnnFeatRelDegree     = "rel_degree_norm"
	gnnFeatKindHash      = "kind_hash"
)

// gnnProviderFeats / gnnModelFeats 记录节点自身使用的特征名子集（保持顺序稳定）。
var (
	gnnProviderFeatNames = []string{gnnFeatDisabled, gnnFeatIsShared, gnnFeatDegree, gnnFeatKindHash}
	gnnModelFeatNames    = []string{gnnFeatDisabled, gnnFeatIsShared, gnnFeatSharingDegree, gnnFeatRelDegree}
	gnnEdgeFeatNames     = []string{gnnFeatDisabled}
)

type providerModelGNNService struct {
	client ProviderModelGraphClient
}

func NewProviderModelGraphService(client ProviderModelGraphClient) ProviderModelGNNProvider {
	return &providerModelGNNService{client: client}
}

func (s *providerModelGNNService) GetProviderModelGraph(ctx context.Context) (ProviderModelGraphResponse, error) {
	if s.client == nil {
		return ProviderModelGraphResponse{}, fmt.Errorf("provider model graph client is not configured")
	}
	result, err := s.client.FetchManagementConfig(ctx)
	if err != nil {
		return ProviderModelGraphResponse{}, err
	}
	return buildProviderModelGNN(result.Payload), nil
}

// buildProviderModelGNN 在原有静态图构建的基础上计算 GNN 特征/嵌入。
func buildProviderModelGNN(config providerconfig.ManagementConfig) ProviderModelGraphResponse {
	providers := buildProviderModelGraphProviders(config)
	graph := computeProviderModelGNN(providers)
	return ProviderModelGraphResponse{Providers: providers, Graph: graph}
}

// buildProviderModelGraphProviders 保持旧的静态图构建逻辑（脱敏、合并、排序）。
func buildProviderModelGraphProviders(config providerconfig.ManagementConfig) []ProviderModelGraphNode {
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

	return providers
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

// ---------------------------------------------------------------------------
// GNN 核心：特征提取 + 一层消息传递嵌入
// ---------------------------------------------------------------------------

// gnnModelInfo 聚合跨 provider 的同一 model 信息（以 label 为键）。
type gnnModelInfo struct {
	label     string
	providers []string
	disabled  bool
}

func computeProviderModelGNN(providers []ProviderModelGraphNode) ProviderModelGNNGraph {
	graph := ProviderModelGNNGraph{
		FeatureNames: gnnFeatureNames(),
		Nodes:        make([]ProviderModelGNNNode, 0, len(providers)),
		Edges:        make([]ProviderModelGNNEdge, 0),
		Embeddings:   make(map[string][]float64),
	}
	if len(providers) == 0 {
		graph.Meta = ProviderModelGNNGraphMeta{FeatureDim: len(gnnFeatureNames())}
		return graph
	}

	providerIDs := make([]string, 0, len(providers))
	providerKinds := make(map[string]string, len(providers))
	providerDisabled := make(map[string]bool, len(providers))
	models := make(map[string]*gnnModelInfo)

	// 聚合所有 model（按 Label 合并），并统计每个 provider 的边。
	for _, p := range providers {
		pid := "provider:" + p.Name
		providerIDs = append(providerIDs, pid)
		providerKinds[pid] = p.Kind
		providerDisabled[pid] = p.Disabled
		for _, m := range p.Models {
			label := strings.TrimSpace(m.Label)
			if label == "" {
				continue
			}
			mi, ok := models[label]
			if !ok {
				mi = &gnnModelInfo{label: label}
				models[label] = mi
			}
			mi.providers = append(mi.providers, p.Name)
			if p.Disabled {
				mi.disabled = true
			}
			graph.Edges = append(graph.Edges, ProviderModelGNNEdge{
				Source:   pid,
				Target:   "model:" + label,
				Disabled: p.Disabled || mi.disabled,
			})
		}
	}

	modelLabels := make([]string, 0, len(models))
	for label := range models {
		modelLabels = append(modelLabels, label)
	}
	sort.Strings(modelLabels)

	// 归一化分母
	maxProviderModels := 1
	maxSharing := 1
	for _, p := range providers {
		if len(p.Models) > maxProviderModels {
			maxProviderModels = len(p.Models)
		}
	}
	for _, mi := range models {
		if len(mi.providers) > maxSharing {
			maxSharing = len(mi.providers)
		}
	}
	maxRel := float64(maxSharing * maxProviderModels)

	totalEdges := len(graph.Edges)

	// 构建节点特征向量
	nodeVectors := make(map[string][]float64, len(providerIDs)+len(modelLabels))
	fullDim := len(gnnFeatureNames())

	for _, pid := range providerIDs {
		pName := strings.TrimPrefix(pid, "provider:")
		var p ProviderModelGraphNode
		for _, candidate := range providers {
			if candidate.Name == pName {
				p = candidate
				break
			}
		}
		vec := make([]float64, fullDim)
		vec[gnnFeatureIndex(gnnFeatDisabled)] = boolToFloat(providerDisabled[pid])
		vec[gnnFeatureIndex(gnnFeatIsShared)] = 0
		vec[gnnFeatureIndex(gnnFeatDegree)] = float64(len(p.Models)) / float64(maxProviderModels)
		vec[gnnFeatureIndex(gnnFeatKindHash)] = kindHash(providerKinds[pid])
		nodeVectors[pid] = vec

		graph.Nodes = append(graph.Nodes, ProviderModelGNNNode{
			ID:       pid,
			Type:     "provider",
			Disabled: providerDisabled[pid],
			Features: ProviderModelGNNFeature{Names: gnnProviderFeatNames, Vector: selectFeatures(vec, gnnProviderFeatNames)},
		})
	}

	for _, label := range modelLabels {
		mid := "model:" + label
		mi := models[label]
		sharing := len(mi.providers)
		vec := make([]float64, fullDim)
		vec[gnnFeatureIndex(gnnFeatDisabled)] = boolToFloat(mi.disabled)
		vec[gnnFeatureIndex(gnnFeatIsShared)] = boolToFloat(sharing > 1)
		vec[gnnFeatureIndex(gnnFeatSharingDegree)] = float64(sharing) / float64(maxSharing)
		vec[gnnFeatureIndex(gnnFeatRelDegree)] = float64(sharing*maxProviderModels) / maxRel
		nodeVectors[mid] = vec

		graph.Nodes = append(graph.Nodes, ProviderModelGNNNode{
			ID:       mid,
			Type:     "model",
			Disabled: mi.disabled,
			Features: ProviderModelGNNFeature{Names: gnnModelFeatNames, Vector: selectFeatures(vec, gnnModelFeatNames)},
		})
	}

	// 边权 + 边特征
	for i := range graph.Edges {
		graph.Edges[i].Weight = 1.0 / float64(totalEdges)
		vec := make([]float64, fullDim)
		vec[gnnFeatureIndex(gnnFeatDisabled)] = boolToFloat(graph.Edges[i].Disabled)
		graph.Edges[i].Features = ProviderModelGNNFeature{Names: gnnEdgeFeatNames, Vector: selectFeatures(vec, gnnEdgeFeatNames)}
	}

	// 一层消息传递（mean aggregation）：节点嵌入 = 自身特征与邻居特征均值的凸组合。
	graph.Embeddings = messagePassingMean(nodeVectors, graph.Edges, fullDim)

	graph.Meta = ProviderModelGNNGraphMeta{
		ProviderCount: len(providerIDs),
		ModelCount:    len(modelLabels),
		EdgeCount:     totalEdges,
		FeatureDim:    fullDim,
		HiddenDim:     fullDim,
	}
	return graph
}

// messagePassingMean 执行一层 GNN 消息传递：
//
//	h_v' = 0.5 * h_v + 0.5 * mean({ h_u | u ∈ N(v) })
//
// 边是 provider→model 双向传播的（无向消息传递）。
func messagePassingMean(nodeVectors map[string][]float64, edges []ProviderModelGNNEdge, dim int) map[string][]float64 {
	neighbors := make(map[string][]string, len(nodeVectors))
	for _, e := range edges {
		neighbors[e.Source] = append(neighbors[e.Source], e.Target)
		neighbors[e.Target] = append(neighbors[e.Target], e.Source)
	}
	embeddings := make(map[string][]float64, len(nodeVectors))
	for id, vec := range nodeVectors {
		out := make([]float64, dim)
		copy(out, vec)
		adj, ok := neighbors[id]
		if !ok || len(adj) == 0 {
			embeddings[id] = out
			continue
		}
		mean := make([]float64, dim)
		for _, nb := range adj {
			for i := 0; i < dim; i++ {
				mean[i] += nodeVectors[nb][i]
			}
		}
		for i := 0; i < dim; i++ {
			mean[i] /= float64(len(adj))
			out[i] = 0.5*vec[i] + 0.5*mean[i]
		}
		embeddings[id] = out
	}
	return embeddings
}

func gnnFeatureNames() []string {
	return []string{
		gnnFeatDisabled,
		gnnFeatIsShared,
		gnnFeatDegree,
		gnnFeatSharingDegree,
		gnnFeatRelDegree,
		gnnFeatKindHash,
	}
}

func gnnFeatureIndex(name string) int {
	switch name {
	case gnnFeatDisabled:
		return 0
	case gnnFeatIsShared:
		return 1
	case gnnFeatDegree:
		return 2
	case gnnFeatSharingDegree:
		return 3
	case gnnFeatRelDegree:
		return 4
	case gnnFeatKindHash:
		return 5
	}
	return -1
}

func selectFeatures(full []float64, names []string) []float64 {
	out := make([]float64, len(names))
	for i, name := range names {
		idx := gnnFeatureIndex(name)
		if idx >= 0 && idx < len(full) {
			out[i] = full[idx]
		}
	}
	return out
}

func boolToFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// kindHash 把 provider kind 映射到 [0,1) 的稳定哈希，供前端着色使用。
func kindHash(kind string) float64 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(kind))
	return float64(h.Sum32()) / 4294967296.0
}
