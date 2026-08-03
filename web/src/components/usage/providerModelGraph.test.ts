import { describe, expect, it } from 'vitest'

import type { ProviderModelGraphResponse } from '@/lib/types'

import { buildProviderModelGraph } from './providerModelGraph'

// Feature names mirror the backend GNN (fixed ordering, indices 0-5).
const FEATURE_NAMES = [
	'disabled',
	'is_shared',
	'degree_norm',
	'sharing_degree_norm',
	'rel_degree_norm',
	'kind_hash',
]

const baseResponse: ProviderModelGraphResponse = {
	providers: [
		{
			name: '08 Open Router',
			kind: 'openai-compatibility',
			models: [
				{ name: 'openai/gpt-5.5', alias: 'gpt-5.5', label: 'gpt-5.5' },
				{ name: 'deepseek-ai/deepseek-v4-flash', alias: 'deepseek-v4-flash', label: 'deepseek-v4-flash' },
			],
		},
		{
			name: '14 GPT-Load Grok',
			kind: 'openai-compatibility',
			disabled: true,
			models: [
				{ name: 'openai/gpt-5.5', alias: 'gpt-5.5', label: 'gpt-5.5' },
				{ name: 'grok-4.70-1M', alias: 'grok-4.70-1M', label: 'grok-4.70-1M' },
			],
		},
		{
			name: '14 GPT-Load Grok',
			kind: 'openai-compatibility',
			models: [
				{ name: 'gpt-5.5', alias: '', label: '' },
			],
		},
		{
			name: 'Moonshot AI',
			kind: 'openai-compatibility',
			models: [],
		},
		{
			name: '   ',
			kind: 'openai-compatibility',
			models: [{ name: 'ignored', alias: 'ignored', label: 'ignored' }],
		},
		{
			name: '09 ZenMux',
			kind: 'openai-compatibility',
			models: [{ name: 'openai/gpt-5.5', alias: 'gpt-5.5', label: 'gpt-5.5' }],
		},
	],
	graph: {
		feature_names: FEATURE_NAMES,
		nodes: [
			{
				id: 'provider:08 Open Router',
				type: 'provider',
				features: { names: FEATURE_NAMES, vector: [0, 0, 0.4, 0, 0, 0.42] },
			},
			{
				id: 'provider:14 GPT-Load Grok',
				type: 'provider',
				disabled: true,
				features: { names: FEATURE_NAMES, vector: [1, 0, 0.4, 0, 0, 0.42] },
			},
			{
				id: 'provider:09 ZenMux',
				type: 'provider',
				features: { names: FEATURE_NAMES, vector: [0, 0, 0.2, 0, 0, 0.42] },
			},
			{
				id: 'model:gpt-5.5',
				type: 'model',
				disabled: true,
				features: { names: FEATURE_NAMES, vector: [1, 1, 0.6, 1, 1, 0] },
			},
			{
				id: 'model:deepseek-v4-flash',
				type: 'model',
				features: { names: FEATURE_NAMES, vector: [0, 0, 0.2, 0, 0.33, 0] },
			},
			{
				id: 'model:grok-4.70-1M',
				type: 'model',
				disabled: true,
				features: { names: FEATURE_NAMES, vector: [1, 0, 0.2, 0, 0.33, 0] },
			},
		],
		edges: [
			{
				source: 'provider:08 Open Router',
				target: 'model:gpt-5.5',
				weight: 0.2,
				features: { names: FEATURE_NAMES, vector: [0, 0, 0, 0, 0, 0] },
			},
			{
				source: 'provider:08 Open Router',
				target: 'model:deepseek-v4-flash',
				weight: 0.2,
				features: { names: FEATURE_NAMES, vector: [0, 0, 0, 0, 0, 0] },
			},
			{
				source: 'provider:14 GPT-Load Grok',
				target: 'model:gpt-5.5',
				weight: 0.2,
				disabled: true,
				features: { names: FEATURE_NAMES, vector: [1, 0, 0, 0, 0, 0] },
			},
			{
				source: 'provider:14 GPT-Load Grok',
				target: 'model:grok-4.70-1M',
				weight: 0.2,
				disabled: true,
				features: { names: FEATURE_NAMES, vector: [1, 0, 0, 0, 0, 0] },
			},
			{
				source: 'provider:09 ZenMux',
				target: 'model:gpt-5.5',
				weight: 0.2,
				features: { names: FEATURE_NAMES, vector: [0, 0, 0, 0, 0, 0] },
			},
		],
		embeddings: {
			'provider:08 Open Router': [0.1, 0.2, 0.3, 0, 0, 0.25],
			'provider:14 GPT-Load Grok': [0.5, 0.1, 0.1, 0, 0, 0.25],
			'provider:09 ZenMux': [0.05, 0.05, 0.1, 0, 0, 0.25],
			'model:gpt-5.5': [0.55, 0.55, 0.4, 0.5, 0.5, 0.2],
			'model:deepseek-v4-flash': [0.05, 0.05, 0.2, 0, 0.2, 0.1],
			'model:grok-4.70-1M': [0.5, 0.05, 0.15, 0, 0.15, 0.1],
		},
		meta: {
			provider_count: 3,
			model_count: 3,
			edge_count: 5,
			feature_dim: 6,
			hidden_dim: 6,
		},
	},
}

describe('buildProviderModelGraph', () => {
	it('merges model nodes per shared alias (or name/label if missing) across all providers', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const modelNodes = graph.nodes.filter((n) => n.data.type === 'model')
		const modelIds = modelNodes.map((n) => n.id)

		// Should only emit 3 unique model nodes: gpt-5.5, deepseek-v4-flash, grok-4.70-1M
		expect(modelIds).toEqual([
			'model:gpt-5.5',
			'model:deepseek-v4-flash',
			'model:grok-4.70-1M',
		])

		// Each model's data should include merged lists of providers/names/aliases
		const gpt = modelNodes.find((n) => n.id === 'model:gpt-5.5')
		expect(gpt?.data.type === 'model').toBeTruthy()
		if (gpt?.data.type !== 'model') return
		expect(gpt.data.providerCount).toBe(3)
		expect(gpt.data.providers).toEqual([
			'08 Open Router',
			'14 GPT-Load Grok',
			'09 ZenMux',
		])
		expect(gpt.data.names).toContain('openai/gpt-5.5')
		// The second entry with name 'gpt-5.5' + alias 'gpt-5.5' dedupes into the same key
		expect(gpt.data.aliases).toContain('gpt-5.5')
	})

	it('creates one edge per (provider, merged-model) pair and keeps disabled metadata', () => {
		const graph = buildProviderModelGraph(baseResponse)
		expect(graph.edges).toHaveLength(5)

		// The provider with disabled=true should propagate that to its model nodes
		const gptNode = graph.nodes.find((n) => n.id === 'model:gpt-5.5')
		expect(gptNode?.data.type === 'model').toBeTruthy()
		if (gptNode?.data.type !== 'model') return
		expect(gptNode.data.disabled).toBe(true)
	})

	it('assigns stable, reasonable layout positions (compact two column spread)', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const providerNodes = graph.nodes.filter((n) => n.data.type === 'provider')
		const modelNodes = graph.nodes.filter((n) => n.data.type === 'model')

		// Providers stack in left column and models are laid out in right column(s)
		expect(providerNodes.every((n) => n.position.x === 0)).toBe(true)
		// With 3 merged models and MAX_MODEL_COLS=4, all fit in the first column
		expect(modelNodes.every((n) => n.position.x === 400)).toBe(true)

		// No node should be negative or NaN, and vertical position should be reasonable
		for (const n of graph.nodes) {
			expect(Number.isFinite(n.position.x) && Number.isFinite(n.position.y)).toBe(true)
			expect(n.position.x).toBeGreaterThanOrEqual(0)
			expect(n.position.y).toBeGreaterThanOrEqual(0)
		}
		// Model nodes are completely covered by the provider and model row/column
		expect(graph.totalHeight).toBeGreaterThan(modelNodes.length * 8)
	})

	it('returns an empty graph when no providers have models', () => {
		const graph = buildProviderModelGraph({ providers: [] })
		expect(graph.nodes).toHaveLength(0)
		expect(graph.edges).toHaveLength(0)
		expect(graph.providerCount).toBe(0)
		expect(graph.modelCount).toBe(0)
		expect(graph.edgeCount).toBe(0)
		expect(graph.totalHeight).toBeGreaterThanOrEqual(0)
	})

	it('joins GNN features, embeddings, and meta onto layout nodes', () => {
		const graph = buildProviderModelGraph(baseResponse)

		// Meta summary should mirror the GNN meta block.
		expect(graph.meta).toEqual({
			providerCount: 3,
			modelCount: 3,
			edgeCount: 5,
			featureDim: 6,
			hiddenDim: 6,
		})

		// Provider node picks up kind_hash + embedding + named features.
		const router = graph.nodes.find((n) => n.id === 'provider:08 Open Router')
		expect(router?.data.type).toBe('provider')
		if (router?.data.type !== 'provider') return
		expect(router.data.kindHash).toBeCloseTo(0.42, 5)
		expect(router.data.embedding).toEqual([0.1, 0.2, 0.3, 0, 0, 0.25])
		expect(router.data.features).toMatchObject({
			disabled: 0,
			is_shared: 0,
			degree_norm: 0.4,
		})

		// Disabled provider carries disabled feature = 1.
		const grokProvider = graph.nodes.find((n) => n.id === 'provider:14 GPT-Load Grok')
		expect(grokProvider?.data.type).toBe('provider')
		if (grokProvider?.data.type !== 'provider') return
		expect(grokProvider.data.features?.disabled).toBe(1)

		// Shared model exposes is_shared + feature vector + embedding.
		const gpt = graph.nodes.find((n) => n.id === 'model:gpt-5.5')
		expect(gpt?.data.type).toBe('model')
		if (gpt?.data.type !== 'model') return
		expect(gpt.data.isShared).toBe(true)
		expect(gpt.data.features?.is_shared).toBe(1)
		expect(gpt.data.features?.sharing_degree_norm).toBe(1)
		expect(gpt.data.embedding).toHaveLength(6)

		// Non-shared model should not be flagged as shared.
		const deepseek = graph.nodes.find((n) => n.id === 'model:deepseek-v4-flash')
		expect(deepseek?.data.type).toBe('model')
		if (deepseek?.data.type !== 'model') return
		expect(deepseek.data.isShared).toBe(false)

		// Edge data carries GNN weights via the response (front-end just uses ids).
		expect(graph.edges.every((e) => e.source.startsWith('provider:'))).toBe(true)
		expect(graph.edges.every((e) => e.target.startsWith('model:'))).toBe(true)
	})

	it('remains robust when the GNN block is missing entirely', () => {
		const stripped: ProviderModelGraphResponse = {
			providers: baseResponse.providers,
		}
		const graph = buildProviderModelGraph(stripped)
		expect(graph.meta).toBeUndefined()
		const gpt = graph.nodes.find((n) => n.id === 'model:gpt-5.5')
		expect(gpt?.data.type).toBe('model')
		if (gpt?.data.type !== 'model') return
		expect(gpt.data.features).toBeUndefined()
		expect(gpt.data.embedding).toBeUndefined()
		expect(gpt.data.isShared).toBeUndefined()
	})
})
