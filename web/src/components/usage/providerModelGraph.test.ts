import { describe, expect, it } from 'vitest'

import type { ProviderModelGraphResponse } from '@/lib/types'

import { buildProviderModelGraph } from './providerModelGraph'

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
})
