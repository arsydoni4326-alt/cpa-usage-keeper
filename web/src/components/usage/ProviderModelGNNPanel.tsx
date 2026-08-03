import { useEffect, useMemo, useState, lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'
import {
	Background,
	Controls,
	MiniMap,
	Position,
	ReactFlow,
	ReactFlowProvider,
	useNodesInitialized,
	useReactFlow,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'

import { fetchProviderModelGNN } from '@/lib/api'
import type { ProviderModelGraphResponse } from '@/lib/types'

import styles from './ProviderModelGNNPanel.module.scss'
import { buildProviderModelGraph, type ProviderGraphNode } from './providerModelGraph'

// Lazy-load the Reagraph (WebGL) canvas so the xyflow grid stays the fast default
// and the heavier three.js bundle only downloads when the user opts in.
const ProviderModelReagraphCanvas = lazy(async () => {
	const mod = await import('./ProviderModelReagraphPanel')
	return { default: mod.ProviderModelReagraphCanvas }
})

const PROVIDER_STYLE: React.CSSProperties = {
	background: '#dbeafe',
	color: '#1e3a8a',
	border: '1px solid #60a5fa',
	borderRadius: 8,
	padding: '8px 12px',
	fontWeight: 600,
	fontSize: 12,
	maxWidth: 280,
	width: 240,
	textAlign: 'left',
	whiteSpace: 'nowrap',
	overflow: 'hidden',
	textOverflow: 'ellipsis',
}

const PROVIDER_DISABLED_STYLE: React.CSSProperties = {
	...PROVIDER_STYLE,
	background: '#fee2e2',
	color: '#7f1d1d',
	border: '1px solid #f87171',
	opacity: 0.75,
}

const MODEL_STYLE: React.CSSProperties = {
	background: '#ffffff',
	color: '#111827',
	border: '1px solid #d0d7de',
	borderRadius: 6,
	padding: '6px 10px',
	fontSize: 11,
	fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
	width: 360,
	textAlign: 'left',
	whiteSpace: 'nowrap',
	overflow: 'hidden',
	textOverflow: 'ellipsis',
}

const MODEL_DISABLED_STYLE: React.CSSProperties = {
	...MODEL_STYLE,
	background: '#fef3c7',
	border: '1px solid #f59e0b',
	color: '#78350f',
	opacity: 0.85,
}

const MODEL_SHARED_BADGE_STYLE: React.CSSProperties = {
	display: 'inline-block',
	marginLeft: 6,
	padding: '0 5px',
	borderRadius: 999,
	fontSize: 9,
	fontWeight: 700,
	lineHeight: '14px',
	verticalAlign: 'middle',
	background: '#ede9fe',
	color: '#5b21b6',
	border: '1px solid #c4b5fd',
}

// providerStyleByKindHash tints provider nodes by the GNN kind_hash feature so
// each provider kind gets a stable color without a hard-coded palette lookup.
// We only vary hue while keeping saturation/lightness aligned with the default
// provider style so disabled styling stays legible.
function providerStyleByKindHash(kindHash: number | undefined, disabled: boolean): React.CSSProperties {
	if (disabled) return PROVIDER_DISABLED_STYLE
	if (typeof kindHash !== 'number' || !Number.isFinite(kindHash)) return PROVIDER_STYLE
	const hue = Math.round(kindHash * 360) % 360
	return {
		...PROVIDER_STYLE,
		background: `hsl(${hue}, 84%, 92%)`,
		color: `hsl(${hue}, 60%, 24%)`,
		border: `1px solid hsl(${hue}, 65%, 55%)`,
	}
}

// formatFeatureValue renders a single feature value, collapsing near-integers
// (0/1 flags) while keeping normalized features readable.
function formatFeatureValue(value: number): string {
	if (Number.isInteger(value)) return String(value)
	if (value === 0 || value === 1) return String(value)
	return value.toFixed(3)
}

// describeFeatures renders the GNN feature vector as `name=value` pairs joined
// by spaces; when only a few names are present the raw ordering is preserved.
function describeFeatures(features: Record<string, number> | undefined): string | null {
	if (!features) return null
	const entries = Object.entries(features)
	if (entries.length === 0) return null
	return entries.map(([name, value]) => `${name}=${formatFeatureValue(value)}`).join(' ')
}

// describeEmbedding renders the node embedding vector with fixed precision,
// wrapped in brackets so it reads as a tuple in tooltips.
function describeEmbedding(embedding: number[] | undefined): string | null {
	if (!embedding || embedding.length === 0) return null
	return `[${embedding.map((v) => v.toFixed(3)).join(', ')}]`
}

export function ProviderModelGNNPanel() {
	const { t } = useTranslation()
	const [data, setData] = useState<ProviderModelGraphResponse | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState<string | null>(null)
	const [renderer, setRenderer] = useState<'flow' | 'reagraph'>('flow')

	useEffect(() => {
		const controller = new AbortController()
		fetchProviderModelGNN(controller.signal)
			.then(setData)
			.catch((err: unknown) => {
				if ((err as Error)?.name === 'AbortError') return
				setError(err instanceof Error ? err.message : String(err))
			})
			.finally(() => setLoading(false))
		return () => controller.abort()
	}, [])

	const graph = useMemo(() => (data ? buildProviderModelGraph(data) : null), [data])

	const styledNodes = useMemo(() => {
		if (!graph) return []
		return graph.nodes.map((node) => {
			const isProvider = node.data.type === 'provider'
			const disabled = isProvider
				? node.data.disabled
				: (node as ProviderGraphNode & { data: { disabled?: boolean } }).data.disabled ?? false
			// kindHash only exists on provider data; on model data the Record index
			// signature types it as unknown, so narrow explicitly before use.
			const kindHash = typeof node.data.kindHash === 'number' ? node.data.kindHash : undefined
			const baseStyle = isProvider
				? providerStyleByKindHash(kindHash, disabled)
				: disabled
					? MODEL_DISABLED_STYLE
					: MODEL_STYLE

			const featureLine = describeFeatures(node.data.features)
			const embeddingLine = describeEmbedding(node.data.embedding)

			// Tooltip with full name/alias/provider summary plus GNN state.
			const title = isProvider
				? (() => {
						const pieces: string[] = [
							`${node.data.label} • ${node.data.kind} • ${t('usage_stats.provider_model_graph.tooltip_models', { count: node.data.modelCount })}`,
						]
						if (featureLine) pieces.push(`${t('usage_stats.provider_model_graph.tooltip_features')}: ${featureLine}`)
						if (embeddingLine) pieces.push(`${t('usage_stats.provider_model_graph.tooltip_embedding')}: ${embeddingLine}`)
						return pieces.filter(Boolean).join('\n')
					})()
				: (() => {
						const d = node.data as {
							label: string
							names?: string[]
							aliases?: string[]
							providers?: string[]
							providerCount?: number
						}
						const aliasNames = Array.isArray(d.aliases) ? d.aliases : []
						const rawNames = Array.isArray(d.names) ? d.names : []
						const providerNames = Array.isArray(d.providers) ? d.providers : []
						const pieces: string[] = [d.label]
						if (aliasNames.length > 0) pieces.push(`alias: ${aliasNames.join(', ')}`)
						if (rawNames.some((n) => n !== d.label)) {
							pieces.push(`raw: ${rawNames.filter((n) => n && n !== d.label).join(', ')}`)
						}
						if (providerNames.length > 0) {
							pieces.push(`via: ${providerNames.join(', ')}`)
						}
						if (featureLine) pieces.push(`${t('usage_stats.provider_model_graph.tooltip_features')}: ${featureLine}`)
						if (embeddingLine) pieces.push(`${t('usage_stats.provider_model_graph.tooltip_embedding')}: ${embeddingLine}`)
						return pieces.filter(Boolean).join('\n')
					})()

			const isShared = !isProvider && (node.data as { isShared?: boolean }).isShared === true
			const label = isShared ? (
				<span title={title}>
					{node.data.label}
					<span style={MODEL_SHARED_BADGE_STYLE}>{t('usage_stats.provider_model_graph.shared_badge')}</span>
				</span>
			) : (
				<span title={title}>{node.data.label}</span>
			)

			return {
				...node,
				sourcePosition: Position.Right,
				targetPosition: Position.Left,
				style: baseStyle,
				data: {
					...node.data,
					label,
				},
			} as ProviderGraphNode
		})
	}, [graph, t])

	let body
	if (loading) {
		body = <div className={styles.state}>{t('usage_stats.provider_model_graph.loading')}</div>
	} else if (error) {
		body = (
			<div className={`${styles.state} ${styles.stateError}`}>
				{t('usage_stats.provider_model_graph.error', { message: error })}
			</div>
		)
	} else if (!graph || graph.nodes.length === 0) {
		body = <div className={styles.state}>{t('usage_stats.provider_model_graph.empty')}</div>
	} else if (renderer === 'reagraph') {
		body = (
			<Suspense fallback={<div className={styles.state}>{t('usage_stats.provider_model_graph.loading')}</div>}>
				<ProviderModelReagraphCanvas graph={graph} />
			</Suspense>
		)
	} else {
		body = (
			<div className={styles.canvas}>
				<ReactFlowProvider>
					<ReactFlow
						nodes={styledNodes}
						edges={graph.edges}
						fitView
						minZoom={0.02}
						maxZoom={1.6}
						fitViewOptions={{ padding: 0.1, minZoom: 0.05, maxZoom: 0.9 }}
						nodesDraggable
						nodesConnectable={false}
						elementsSelectable
						defaultEdgeOptions={{ type: 'smoothstep' }}
					>
						<Background gap={16} />
						<Controls />
						<MiniMap
							pannable
							zoomable
							nodeColor={(node) => {
								const datum = (node as ProviderGraphNode).data
								if (datum.type !== 'provider') return '#cbd5e1'
								if (typeof datum.kindHash === 'number' && Number.isFinite(datum.kindHash)) {
									return `hsl(${Math.round(datum.kindHash * 360) % 360}, 65%, 55%)`
								}
								return '#60a5fa'
							}}
						/>
					</ReactFlow>
					<FitViewWhenReady />
				</ReactFlowProvider>
			</div>
		)
	}

	return (
		<div className={styles.panel}>
			<div className={styles.header}>
				<div>
					<h3 className={styles.title}>{t('usage_stats.provider_model_graph.title')}</h3>
					<p className={styles.subtitle}>{t('usage_stats.provider_model_graph.subtitle')}</p>
				</div>
				<div className={styles.headerRight}>
					{!loading && !error && graph ? (
						<div className={styles.summary}>
							{t('usage_stats.provider_model_graph.summary', {
								providers: graph.meta?.providerCount ?? graph.providerCount,
								models: graph.meta?.edgeCount ?? graph.edgeCount,
								dims: graph.meta?.featureDim ?? 0,
							})}
						</div>
					) : null}
					<div className={styles.toggle} role="tablist" aria-label={t('usage_stats.provider_model_graph.renderer_label')}>
						<button
							type="button"
							role="tab"
							aria-selected={renderer === 'flow'}
							className={`${styles.toggleButton} ${renderer === 'flow' ? styles.toggleButtonActive : ''}`}
							onClick={() => setRenderer('flow')}
						>
							{t('usage_stats.provider_model_graph.renderer_classic')}
						</button>
						<button
							type="button"
							role="tab"
							aria-selected={renderer === 'reagraph'}
							className={`${styles.toggleButton} ${renderer === 'reagraph' ? styles.toggleButtonActive : ''}`}
							onClick={() => setRenderer('reagraph')}
						>
							{t('usage_stats.provider_model_graph.renderer_reagraph')}
						</button>
					</div>
				</div>
			</div>
			{body}
		</div>
	)
}

// Refits the graph when node dimensions have been measured. React Flow's
// initial `fitView` can run before node size data is available, especially
// with dynamic content or hidden parents, so we call fitView again after
// `useNodesInitialized` fires.
function FitViewWhenReady() {
	const initialized = useNodesInitialized()
	const { fitView } = useReactFlow()

	useEffect(() => {
		if (!initialized) return
		fitView({ padding: 0.1, minZoom: 0.05, maxZoom: 0.9 })
	}, [initialized, fitView])

	return null
}
