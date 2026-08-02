import type { ReactNode } from 'react'
import type { Edge, Node } from '@xyflow/react'

import type { ProviderModelGraphResponse } from '@/lib/types'

// label is a string when built; the panel may swap it for a styled React node.
interface GraphModelDatum extends Record<string, unknown> {
	type: 'model'
	label: ReactNode
	name: string
	provider: string
	providerCount: number
	disabled: boolean
}

interface GraphProviderDatum extends Record<string, unknown> {
	type: 'provider'
	label: ReactNode
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
	providerCount: number
	edgeCount: number
	totalHeight: number
}

// Two-column layout: providers on the left, their models stacked beside each provider.
// Provider row height is sized by how many models it owns, so rows never overlap and
// each provider's models sit in its own band. Models shared across providers are
// rendered once per (provider, model) pair so edges link to a clean horizontal row —
// keeps the graph readable even with 100+ models.
const PROVIDER_X = 0
const MODEL_X = 460
const HEADER_HEIGHT = 8 // top padding
const PROVIDER_HEADER = 22 // vertical offset of provider node inside its band
const MODEL_ROW_H = 56
const PROVIDER_MIN_H = 64
const PROVIDER_GAP = 22

interface PreparedProvider {
	name: string
	kind: string
	disabled: boolean
	models: { label: string; name: string }[]
}

function collectProviders(response: ProviderModelGraphResponse): PreparedProvider[] {
	const result: PreparedProvider[] = []
	const seen = new Map<string, PreparedProvider>()

	for (const raw of response.providers ?? []) {
		if (!raw) continue
		const name = (raw.name ?? '').trim()
		if (!name) continue

		const seenModels = new Set<string>()
		const models: { label: string; name: string }[] = []
		for (const rawModel of raw.models ?? []) {
			if (!rawModel) continue
			const label = (rawModel.label || rawModel.alias || rawModel.name || '').trim()
			if (!label || seenModels.has(label)) continue
			seenModels.add(label)
			models.push({ label, name: (rawModel.name ?? '').trim() })
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
		const merged = new Set(existing.models.map((m) => m.label))
		for (const model of models) {
			if (merged.has(model.label)) continue
			merged.add(model.label)
			existing.models.push(model)
		}
		if (raw.disabled) existing.disabled = true
	}

	return result
}

export function buildProviderModelGraph(response: ProviderModelGraphResponse): ProviderModelGraphGraph {
	const providers = collectProviders(response)
	const nodes: ProviderGraphNode[] = []
	const edges: ProviderGraphEdge[] = []
	// Count providers per model label for the shared-models badge.
	const providerCountByLabel = new Map<string, number>()

	for (const provider of providers) {
		for (const model of provider.models) {
			providerCountByLabel.set(model.label, (providerCountByLabel.get(model.label) ?? 0) + 1)
		}
	}

	let y = HEADER_HEIGHT
	for (const provider of providers) {
		const providerId = `provider:${provider.name}`
		const modelCount = provider.models.length
		const bandHeight = Math.max(PROVIDER_MIN_H, modelCount * MODEL_ROW_H)

		const providerY = y + Math.min(PROVIDER_HEADER, bandHeight / 2 - 12)
		nodes.push({
			id: providerId,
			position: { x: PROVIDER_X, y: providerY },
			data: {
				type: 'provider',
				label: provider.name,
				kind: provider.kind,
				disabled: provider.disabled,
				modelCount,
			},
		})

		let modelY = y
		for (const model of provider.models) {
			const modelId = `model:${provider.name}::${model.label}`
			nodes.push({
				id: modelId,
				position: { x: MODEL_X, y: modelY },
				data: {
					type: 'model',
					label: model.label,
					name: model.name,
					provider: provider.name,
					providerCount: providerCountByLabel.get(model.label) ?? 1,
					disabled: provider.disabled,
				},
			})
			edges.push({
				id: `edge:${providerId}__${model.label}`,
				source: providerId,
				target: modelId,
				type: 'smoothstep',
			})
			modelY += MODEL_ROW_H
		}

		y += bandHeight + PROVIDER_GAP
	}

	return {
		nodes,
		edges,
		providerCount: providers.length,
		edgeCount: edges.length,
		totalHeight: Math.max(0, y - PROVIDER_GAP),
	}
}
