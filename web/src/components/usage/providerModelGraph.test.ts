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
			models: [{ name: 'gpt-5.5', alias: '', label: '' }],
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
	it('creates provider nodes and one model node per provider-model pair', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const providerIds = graph.nodes.filter((n) => n.data.type === 'provider').map((n) => n.id)
		const modelIds = graph.nodes.filter((n) => n.data.type === 'model').map((n) => n.id)

		expect(providerIds).toEqual([
			'provider:08 Open Router',
			'provider:14 GPT-Load Grok',
			'provider:09 ZenMux',
		])
		expect(modelIds).toEqual([
			'model:08 Open Router::gpt-5.5',
			'model:08 Open Router::deepseek-v4-flash',
			'model:14 GPT-Load Grok::gpt-5.5',
			'model:14 GPT-Load Grok::grok-4.70-1M',
			'model:09 ZenMux::gpt-5.5',
		])
		expect(graph.providerCount).toBe(3)
		expect(graph.edgeCount).toBe(5)
	})

	it('merges duplicate provider blocks and counts providers sharing a label', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const shared = graph.nodes.filter(
			(n) => n.data.type === 'model' && n.data.label === 'gpt-5.5',
		)
		expect(shared).toHaveLength(3)
		for (const node of shared) {
			if (node.data.type !== 'model') continue
			expect(node.data.providerCount).toBe(3)
		}

		const unique = graph.nodes.find((n) => n.id === 'model:14 GPT-Load Grok::grok-4.70-1M')
		expect(unique?.data.type).toBe('model')
		if (unique?.data.type !== 'model') return
		expect(unique.data.providerCount).toBe(1)
		expect(unique.data.name).toBe('grok-4.70-1M')
	})

	it('creates one edge per provider-model pair and keeps disabled metadata', () => {
		const graph = buildProviderModelGraph(baseResponse)
		expect(graph.edges).toHaveLength(5)
		expect(graph.edges[0]).toMatchObject({
			id: 'edge:provider:08 Open Router__gpt-5.5',
			source: 'provider:08 Open Router',
			target: 'model:08 Open Router::gpt-5.5',
		})
		expect(graph.edges.at(-1)).toMatchObject({
			id: 'edge:provider:09 ZenMux__gpt-5.5',
			source: 'provider:09 ZenMux',
			target: 'model:09 ZenMux::gpt-5.5',
		})

		const providerNode = graph.nodes.find((n) => n.id === 'provider:14 GPT-Load Grok')
		expect(providerNode?.data.type).toBe('provider')
		if (providerNode?.data.type !== 'provider') return
		expect(providerNode.data.disabled).toBe(true)
		expect(providerNode.data.modelCount).toBe(2)
	})

	it('assigns deterministic per-provider band layout positions', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const openRouter = graph.nodes.find((n) => n.id === 'provider:08 Open Router')
		const grokProvider = graph.nodes.find((n) => n.id === 'provider:14 GPT-Load Grok')
		const zenmux = graph.nodes.find((n) => n.id === 'provider:09 ZenMux')
		const firstModel = graph.nodes.find((n) => n.id === 'model:08 Open Router::gpt-5.5')
		const secondModel = graph.nodes.find((n) => n.id === 'model:08 Open Router::deepseek-v4-flash')
		const grokModel = graph.nodes.find((n) => n.id === 'model:14 GPT-Load Grok::gpt-5.5')

		// Band 1 '08 Open Router' (2 models): y=8, height=112, next band at 142.
		expect(openRouter?.position).toEqual({ x: 0, y: 30 })
		expect(firstModel?.position).toEqual({ x: 460, y: 8 })
		expect(secondModel?.position).toEqual({ x: 460, y: 64 })
		// Band 2 '14 GPT-Load Grok' (2 models): y=142, height=112, next band at 276.
		expect(grokProvider?.position).toEqual({ x: 0, y: 164 })
		expect(grokModel?.position).toEqual({ x: 460, y: 142 })
		// Band 3 '09 ZenMux' (1 model): y=276, height clamped to min 64.
		expect(zenmux?.position).toEqual({ x: 0, y: 296 })
		expect(graph.totalHeight).toBe(340)
	})

	it('returns an empty graph when no providers have models', () => {
		const graph = buildProviderModelGraph({ providers: [] })
		expect(graph.nodes).toHaveLength(0)
		expect(graph.edges).toHaveLength(0)
		expect(graph.providerCount).toBe(0)
		expect(graph.edgeCount).toBe(0)
		expect(graph.totalHeight).toBe(0)
	})

	// Regression guard: React Flow's fitView silently collapses to an invisible
	// viewport when any node position is non-finite. Every node and edge anchor
	// must be a plain finite number so the canvas always has real geometry.
	it('guarantees every node position is finite and non-negative', () => {
		const graph = buildProviderModelGraph(baseResponse)
		expect(graph.nodes.length).toBeGreaterThan(0)
		for (const node of graph.nodes) {
			expect(Number.isFinite(node.position.x), `${node.id} x`).toBe(true)
			expect(Number.isFinite(node.position.y), `${node.id} y`).toBe(true)
			expect(node.position.x).toBeGreaterThanOrEqual(0)
			expect(node.position.y).toBeGreaterThanOrEqual(0)
		}
		for (const edge of graph.edges) {
			expect(typeof edge.source).toBe('string')
			expect(typeof edge.target).toBe('string')
			expect(graph.nodes.some((n) => n.id === edge.source)).toBe(true)
			expect(graph.nodes.some((n) => n.id === edge.target)).toBe(true)
		}
		expect(Number.isFinite(graph.totalHeight)).toBe(true)
		expect(graph.totalHeight).toBeGreaterThan(0)
	})
})
