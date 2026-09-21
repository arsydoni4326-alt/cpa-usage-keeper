// Feature-local types for the Provider Model GNN domain (mirrors the backend
// response of GET /api/v1/provider-model-gnn, see internal/gnn/gnn.go).

export interface ProviderModelGraphModel {
  name: string
  alias?: string
  label: string
}

export interface ProviderModelGraphNode {
  name: string
  kind: string
  disabled?: boolean
  models: ProviderModelGraphModel[]
}

export interface ProviderModelGNNFeature {
  names: string[]
  vector: number[]
}

export interface ProviderModelGNNGraphNode {
  id: string
  type: 'provider' | 'model'
  disabled?: boolean
  features: ProviderModelGNNFeature
}

export interface ProviderModelGNNGraphEdge {
  source: string
  target: string
  weight: number
  disabled?: boolean
  features: ProviderModelGNNFeature
}

export interface ProviderModelGNNGraphMeta {
  provider_count: number
  model_count: number
  edge_count: number
  feature_dim: number
  hidden_dim: number
}

export interface ProviderModelGNNGraph {
  feature_names: string[]
  nodes: ProviderModelGNNGraphNode[]
  edges: ProviderModelGNNGraphEdge[]
  embeddings?: Record<string, number[]>
  meta: ProviderModelGNNGraphMeta
}

export interface ProviderModelGraphResponse {
  providers: ProviderModelGraphNode[]
  graph?: ProviderModelGNNGraph
}