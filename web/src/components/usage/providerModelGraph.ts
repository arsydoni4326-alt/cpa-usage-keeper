import type { ReactNode } from 'react'
import type { Edge, Node } from '@xyflow/react'

import type {
	ProviderModelGNNGraph,
	ProviderModelGraphResponse,
} from '@/lib/types'

// label is a string when built; the panel may swap it for a styled React node.
interface GraphModelDatum extends Record<string, unknown> {
	type: 'model'
	label: ReactNode
	names: string[]
	aliases: string[]
	providers: string[]
	providerCount: number
	disabled: boolean
	// GNN-derived state, joined from response.graph by node id.
	features?: Record<string, number>
	embedding?: number[]
	isShared?: boolean
}

interface GraphProviderDatum extends Record<string, unknown> {
	type: 'provider'
	label: ReactNode
	kind: string
	disabled: boolean
	modelCount: number
	models: string[]
	// GNN-derived state, joined from response.graph by node id.
	features?: Record<string, number>
	embedding?: number[]
	kindHash?: number
}

export type ProviderGraphModelNode = Node<GraphModelDatum>
export type ProviderGraphProviderNode = Node<GraphProviderDatum>
export type ProviderGraphNode = ProviderGraphModelNode | ProviderGraphProviderNode
export type ProviderGraphEdge = Edge

export interface ProviderModelGraphGraph {
	nodes: ProviderGraphNode[]
	edges: ProviderGraphEdge[]
	providerCount: number
	modelCount: number
	edgeCount: number
	totalHeight: number
	totalWidth: number
	meta?: {
		providerCount: number
		modelCount: number
		edgeCount: number
		featureDim: number
		hiddenDim: number
	}
}

// New layout config for balanced/stretched two-column grid
const PROVIDER_X = 0
const MODEL_X = 400
const HEADER_HEIGHT = 24
const PROVIDER_ROW_H = 54
const PROVIDER_GAP = 24
const MODEL_ROW_H = 42
const MAX_MODEL_COLS = 4
const MODEL_COL_GAP = 380

interface PreparedProvider {
	name: string
	kind: string
	disabled: boolean
	models: { label: string; name: string; alias?: string }[]
}

function collectProviders(response: ProviderModelGraphResponse): PreparedProvider[] {
	const result: PreparedProvider[] = []
	const seen = new Map<string, PreparedProvider>()

	for (const raw of response.providers ?? []) {
		if (!raw) continue
		const name = (raw.name ?? '').trim()
		if (!name) continue

		const seenDoneModels = new Set<string>()
		const models: { label: string; name: string; alias?: string }[] = []
		for (const rawModel of raw.models ?? []) {
			if (!rawModel) continue
			// Always prefer .alias as distinctive key, fallback to name, then label
			const mergeKey = (rawModel.alias || rawModel.name || rawModel.label || '').trim()
			if (!mergeKey || seenDoneModels.has(mergeKey)) continue
			seenDoneModels.add(mergeKey)
			models.push({
				label: (rawModel.label ?? '').trim(),
				name: (rawModel.name ?? '').trim(),
				alias: (rawModel.alias ?? '').trim() || undefined,
			})
		}
		if (models.length === 0) continue

		// Merge duplicates of the same provider name (e.g. repeated config blocks).
		let existing = seen.get(name)
		if (!existing) {
			existing = {
				name,
				kind: raw.kind,
				disabled: !!raw.disabled,
				models: [],
			}
			seen.set(name, existing)
			result.push(existing)
		}
		const mergedLabels = new Set(existing.models.map(m => m.alias ?? m.name ?? m.label))
		for (const model of models) {
			const modelKey = model.alias ?? model.name ?? model.label
			if (mergedLabels.has(modelKey)) continue
			mergedLabels.add(modelKey)
			existing.models.push(model)
		}
		if (raw.disabled) existing.disabled = true
	}
	return result
}

// The merged model info storage
interface MergedModelDatum {
	label: string
	names: Set<string>
	aliases: Set<string>
	providers: Set<string>
	disabled: boolean
}

// featureVectorToMap pairs a feature vector with the graph-wide feature_names
// so the UI can render named values (e.g. tooltips) without knowing index
// ordering up front.
function featureVectorToMap(
	names: string[] | undefined,
	vector: number[] | undefined,
): Record<string, number> | undefined {
	if (!names || !vector || names.length === 0 || vector.length === 0) return undefined
	const out: Record<string, number> = {}
	for (let i = 0; i < names.length && i < vector.length; i++) {
		out[names[i]] = vector[i]
	}
	return out
}

export function buildProviderModelGraph(response: ProviderModelGraphResponse): ProviderModelGraphGraph {
	const providers = collectProviders(response)
	const modelFieldMap = new Map<string, MergedModelDatum>()

	// Merge models by alias|name|label so a given label only ever has a single node
	for (const provider of providers) {
		for (const m of provider.models) {
			const key = m.alias ?? m.name ?? m.label
			if (!key) continue
			if (!modelFieldMap.has(key)) {
				modelFieldMap.set(key, {
					label: key,
					names: new Set<string>(),
					aliases: new Set<string>(),
					providers: new Set<string>(),
					disabled: provider.disabled,
				})
			}
			const merged = modelFieldMap.get(key)!
			if (m.name) merged.names.add(m.name)
			if (m.alias) merged.aliases.add(m.alias)
			merged.providers.add(provider.name)
			if (provider.disabled) merged.disabled = true
		}
	}

	// Index GNN state by node id so layout nodes can be decorated in one pass.
	const gnn: ProviderModelGNNGraph | undefined = response.graph
	const gnnFeatureNames = gnn?.feature_names
	const gnnNodeById = new Map<string, NonNullable<ProviderModelGNNGraph['nodes']>[number]>()
	for (const node of gnn?.nodes ?? []) {
		if (node?.id) gnnNodeById.set(node.id, node)
	}

	// Place provider nodes in fixed column, evenly spaced by PROVIDER_ROW_H
	const providerNodes: ProviderGraphNode[] = []
	let provY = HEADER_HEIGHT
	for (const provider of providers) {
		const providerId = `provider:${provider.name}`
		const modelList = provider.models.map(m => m.alias ?? m.name ?? m.label)
		const gnnNode = gnnNodeById.get(providerId)
		const features = featureVectorToMap(gnnFeatureNames, gnnNode?.features?.vector)
		providerNodes.push({
			id: providerId,
			position: { x: PROVIDER_X, y: provY },
			data: {
				type: 'provider',
				label: provider.name,
				kind: provider.kind,
				disabled: provider.disabled,
				modelCount: modelList.length,
				models: modelList,
				features,
				embedding: gnn?.embeddings?.[providerId],
				kindHash: features?.kind_hash,
			},
		})
		provY += PROVIDER_ROW_H + PROVIDER_GAP
	}

	// Prepare model nodes in fixed column(s), stacked vertically. Small graphs stay in a
	// single column next to the providers; only when the model count exceeds
	// MAX_MODEL_COLS do we split into multiple columns to cap vertical height.
	const mergedModelList = Array.from(modelFieldMap.values())
	const modelCount = mergedModelList.length

	// Distribute models across up to MAX_MODEL_COLS columns, column-major (fill down then right).
	// Only split into multiple columns when the model count exceeds MAX_MODEL_COLS;
	// smaller graphs stay in a single column next to the providers (x=MODEL_X).
	const numModelCols = modelCount > MAX_MODEL_COLS ? MAX_MODEL_COLS : 1
	const rowsPerCol = Math.ceil(modelCount / numModelCols) || 1

	const modelNodes: ProviderGraphNode[] = []
	for (let i = 0; i < modelCount; i++) {
		const merged = mergedModelList[i]
		const col = numModelCols === 1 ? 0 : Math.floor(i / rowsPerCol)
		const row = numModelCols === 1 ? i : i % rowsPerCol
		const modelX = MODEL_X + col * MODEL_COL_GAP
		const modelY = HEADER_HEIGHT + row * MODEL_ROW_H

		const modelId = `model:${merged.label}`
		const gnnNode = gnnNodeById.get(modelId)
		const features = featureVectorToMap(gnnFeatureNames, gnnNode?.features?.vector)
		modelNodes.push({
			id: modelId,
			position: { x: modelX, y: modelY },
			data: {
				type: 'model',
				label: merged.label,
				names: Array.from(merged.names),
				aliases: Array.from(merged.aliases),
				providers: Array.from(merged.providers),
				providerCount: merged.providers.size,
				disabled: merged.disabled,
				features,
				embedding: gnn?.embeddings?.[modelId],
				isShared: features ? features.is_shared === 1 : undefined,
			},
		})
	}

	// Edges: provider-model
	const edges: ProviderGraphEdge[] = []
	for (const provider of providers) {
		const providerId = `provider:${provider.name}`
		for (const m of provider.models) {
			const modelKey = m.alias ?? m.name ?? m.label
			if (!modelKey) continue
			const modelId = `model:${modelKey}`
			edges.push({
				id: `edge:${providerId}__${modelKey}`,
				source: providerId,
				target: modelId,
				type: 'smoothstep',
			})
		}
	}

	// Layout summary: cover the furthest node in each axis
	const providersBottom = HEADER_HEIGHT + providers.length * (PROVIDER_ROW_H + PROVIDER_GAP)
	const modelsBottom = HEADER_HEIGHT + rowsPerCol * MODEL_ROW_H
	const totalHeight = Math.max(providersBottom, modelsBottom)
	const totalWidth = MODEL_X + (numModelCols - 1) * MODEL_COL_GAP + 420

	return {
		nodes: [...providerNodes, ...modelNodes],
		edges,
		providerCount: providers.length,
		modelCount: mergedModelList.length,
		edgeCount: edges.length,
		totalHeight,
		totalWidth,
		meta: gnn?.meta
			? {
					providerCount: gnn.meta.provider_count,
					modelCount: gnn.meta.model_count,
					edgeCount: gnn.meta.edge_count,
					featureDim: gnn.meta.feature_dim,
					hiddenDim: gnn.meta.hidden_dim,
				}
			: undefined,
	}
}
