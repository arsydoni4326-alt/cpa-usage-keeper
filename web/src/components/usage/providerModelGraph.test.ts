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
	it('creates provider nodes and shared model nodes with stable order', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const providerIds = graph.nodes.filter((n) => n.data.type === 'provider').map((n) => n.id)
		const modelIds = graph.nodes.filter((n) => n.data.type === 'model').map((n) => n.id)

		expect(providerIds).toEqual([
			'provider:08 Open Router',
			'provider:14 GPT-Load Grok',
			'provider:09 ZenMux',
		])
		expect(modelIds).toEqual(['model:gpt-5.5', 'model:deepseek-v4-flash', 'model:grok-4.70-1M'])
	})

	it('merges duplicate provider entries and shared model labels', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const modelNode = graph.nodes.find((n) => n.id === 'model:gpt-5.5')
		expect(modelNode).toBeDefined()
		expect(modelNode?.data.type).toBe('model')
		if (modelNode?.data.type !== 'model') return
		expect(modelNode.data.providerCount).toBe(3)
		expect(modelNode.data.providers).toEqual([
			'provider:08 Open Router',
			'provider:14 GPT-Load Grok',
			'provider:09 ZenMux',
		])
	})

	it('creates one edge per unique provider-model connection and keeps disabled metadata', () => {
		const graph = buildProviderModelGraph(baseResponse)
		expect(graph.edges).toHaveLength(5)
		expect(graph.edges[0]).toMatchObject({
			id: 'provider:08 Open Router__model:gpt-5.5',
			source: 'provider:08 Open Router',
			target: 'model:gpt-5.5',
		})
		expect(graph.edges.at(-1)).toMatchObject({
			id: 'provider:09 ZenMux__model:gpt-5.5',
		})

		const providerNode = graph.nodes.find((n) => n.id === 'provider:14 GPT-Load Grok')
		expect(providerNode?.data.type).toBe('provider')
		if (providerNode?.data.type !== 'provider') return
		expect(providerNode.data.disabled).toBe(true)
		expect(providerNode.data.modelCount).toBe(2)
	})

	it('assigns deterministic two-column layout positions', () => {
		const graph = buildProviderModelGraph(baseResponse)
		const openRouter = graph.nodes.find((n) => n.id === 'provider:08 Open Router')
		const zenmux = graph.nodes.find((n) => n.id === 'provider:09 ZenMux')
		const sharedModel = graph.nodes.find((n) => n.id === 'model:gpt-5.5')
		const grok = graph.nodes.find((n) => n.id === 'model:grok-4.70-1M')

		expect(openRouter?.position).toEqual({ x: 0, y: 0 })
		expect(zenmux?.position).toEqual({ x: 0, y: 192 })
		expect(sharedModel?.position).toEqual({ x: 420, y: 0 })
		expect(grok?.position).toEqual({ x: 420, y: 104 })
	})
})
