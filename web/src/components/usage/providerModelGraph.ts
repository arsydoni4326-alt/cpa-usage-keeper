import type { Edge, Node } from '@xyflow/react'

import type { ProviderModelGraphResponse } from '@/lib/types'

interface GraphModelDatum extends Record<string, unknown> {
	type: 'model'
	label: string
	providerCount: number
	providers: string[]
}

interface GraphProviderDatum extends Record<string, unknown> {
	type: 'provider'
	label: string
	kind: string
	disabled: boolean
	modelCount: number
}

export type ProviderGraphModelNode = Node<GraphModelDatum>
export type ProviderGraphProviderNode = Node<GraphProviderDatum>
export type ProviderGraphNode = ProviderGraphModelNode | ProviderGraphProviderNode
export type ProviderGraphEdge = Edge

export interface ProviderModelGraphGraph {
	nodes: ProviderGraphNode[]
	edges: ProviderGraphEdge[]
}

// Simple two-column layout: providers on the left, models on the right.
// @xyflow/react has no built-in auto-layout; computing static positions here
// keeps the dependency footprint small while avoiding overlapping nodes.
const PROVIDER_X = 0
const PROVIDER_GAP_Y = 96
const MODEL_X = 420
const MODEL_GAP_Y = 52

export function buildProviderModelGraph(response: ProviderModelGraphResponse): ProviderModelGraphGraph {
	const edges: ProviderGraphEdge[] = []
	const providerNodes = new Map<string, ProviderGraphProviderNode>()
	const modelNodes = new Map<string, ProviderGraphModelNode>()
	const edgeIds = new Set<string>()
	const orderedModels: ProviderGraphModelNode[] = []
	const orderedProviders: ProviderGraphProviderNode[] = []

	for (const provider of response.providers ?? []) {
		if (!provider) continue
		const name = (provider.name ?? '').trim()
		if (!name) continue

		// Collect usable model labels first so providers without any are skipped.
		const labels: string[] = []
		for (const model of provider.models ?? []) {
			if (!model) continue
			const label = (model.label || model.alias || model.name || '').trim()
			if (!label) continue
			labels.push(label)
		}
		if (labels.length === 0) continue

		const providerId = `provider:${name}`
		if (!providerNodes.has(providerId)) {
			const node: ProviderGraphProviderNode = {
				id: providerId,
				position: { x: PROVIDER_X, y: 0 },
				data: {
					type: 'provider',
					label: name,
					kind: provider.kind,
					disabled: !!provider.disabled,
					modelCount: labels.length,
				},
			}
			providerNodes.set(providerId, node)
			orderedProviders.push(node)
		}

		for (const label of labels) {
			const modelId = `model:${label}`
			let node = modelNodes.get(modelId)
			if (!node) {
				node = {
					id: modelId,
					position: { x: MODEL_X, y: 0 },
					data: {
						type: 'model',
						label,
						providerCount: 0,
						providers: [],
					},
				}
				modelNodes.set(modelId, node)
				orderedModels.push(node)
			}
			if (!node.data.providers.includes(providerId)) {
				node.data.providers.push(providerId)
				node.data.providerCount = node.data.providers.length
			}

			const edgeId = `${providerId}__${modelId}`
			if (edgeIds.has(edgeId)) {
				continue
			}
			edgeIds.add(edgeId)
			edges.push({
				id: edgeId,
				source: providerId,
				target: modelId,
				type: 'smoothstep',
			})
		}
	}

	orderedProviders.forEach((node, index) => {
		node.position = { x: PROVIDER_X, y: index * PROVIDER_GAP_Y }
	})
	orderedModels.forEach((node, index) => {
		node.position = { x: MODEL_X, y: index * MODEL_GAP_Y }
	})

	return { nodes: [...orderedProviders, ...orderedModels], edges }
}
