import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { GraphCanvas, type GraphEdge, type GraphNode, type InternalGraphNode } from 'reagraph'
import { fetchProviderModelGNN } from '@/lib/api'
import type { ProviderModelGraphResponse } from '@/lib/types'
import {
	buildProviderModelGraph,
	type ProviderGraphEdge,
	type ProviderGraphNode,
	type ProviderModelGraphGraph,
} from './providerModelGraph'
import styles from './ProviderModelReagraphPanel.module.scss'

// Keep the provider hue tinting identical to ProviderModelGNNPanel so both views
// agree visually: hue derived from kind_hash, alpha-shifted in dark theme.
function providerFillFromKindHash(kindHash: number | undefined, disabled: boolean, dark: boolean): string {
	if (kindHash === undefined) return disabled ? (dark ? '#3f3f46' : '#d4d4d8') : dark ? '#5b21b6' : '#ede9fe'
	const hue = Math.round(kindHash * 360) % 360
	if (disabled) return dark ? `hsla(${hue}, 18%, 26%, 1)` : `hsla(${hue}, 22%, 84%, 1)`
	return dark ? `hsla(${hue}, 62%, 40%, 1)` : `hsla(${hue}, 78%, 78%, 1)`
}

function modelFill(isShared: boolean | undefined, disabled: boolean, dark: boolean): string {
	if (isShared) return dark ? '#0e7490' : '#22d3ee'
	return disabled ? (dark ? '#27272a' : '#e4e4e7') : dark ? '#155e75' : '#a5f3fc'
}

// Node size: providers larger by model count, models larger when shared across providers.
function providerNodeSize(modelCount: number): number {
	return Math.min(8 + modelCount * 1.2, 32)
}

function modelNodeSize(providerCount: number): number {
	return providerCount > 1 ? Math.min(6 + providerCount * 1.5, 18) : 6
}

interface TooltipState {
	x: number
	y: number
	title: string
	kind: string
	extra?: string
}

interface ReagraphNodeData extends Record<string, unknown> {
	kindLabel: string
	featuresText?: string
	embeddingText?: string
	subtitle?: string
}

function describeFeatures(features?: Record<string, number>): string | undefined {
	if (!features) return undefined
	const entries = Object.entries(features)
	if (entries.length === 0) return undefined
	return entries
		.map(([k, v]) => `${k}=${Number.isInteger(v) ? v : v.toFixed(3)}`)
		.join(' · ')
}

function describeEmbedding(embedding?: number[]): string | undefined {
	return embedding && embedding.length > 0 ? `[${embedding.map(v => v.toFixed(3)).join(', ')}]` : undefined
}

// Canvas-only renderer: receives a prebuilt GNN graph so callers that already
// fetched /api/provider-model-gnn (e.g. ProviderModelGNNPanel) can swap views
// without a second request or a duplicated header/summary block.
export function ProviderModelReagraphCanvas({ graph }: { graph: ProviderModelGraphGraph }) {
	const { t } = useTranslation()
	const [tooltip, setTooltip] = useState<TooltipState | null>(null)
	const containerRef = useRef<HTMLDivElement | null>(null)
	const [isDark, setIsDark] = useState(false)

	// Track theme so WebGL fills match the surrounding CSS. We watch the
	// data-theme attribute on <html>, falling back to no theme (light default).
	useEffect(() => {
		const root = document.documentElement
		const sync = () => setIsDark(root.dataset.theme === 'dark')
		sync()
		const observer = new MutationObserver(sync)
		observer.observe(root, { attributes: true, attributeFilter: ['data-theme'] })
		return () => observer.disconnect()
	}, [])

	const { nodes, edges } = useMemo(() => {
		const mappedNodes: GraphNode[] = graph.nodes.map((node: ProviderGraphNode) => {
			const featuresText = describeFeatures(node.data.features)
			const embeddingText = describeEmbedding(node.data.embedding)
			if (node.data.type === 'provider') {
				const label = typeof node.data.label === 'string' ? node.data.label : node.id.replace(/^provider:/, '')
				const dataPayload: ReagraphNodeData = {
					kindLabel: t('usage_stats.provider_model_graph.tooltip_models', { count: node.data.modelCount }),
					featuresText,
					embeddingText,
					subtitle: node.data.kind,
				}
				return {
					id: node.id,
					label,
					fill: providerFillFromKindHash(node.data.kindHash, node.data.disabled, isDark),
					size: providerNodeSize(node.data.modelCount),
					subLabel: node.data.kind,
					data: dataPayload,
				}
			}
			const label = typeof node.data.label === 'string' ? node.data.label : node.id.replace(/^model:/, '')
			const dataPayload: ReagraphNodeData = {
				kindLabel:
					node.data.providerCount > 1
						? `${node.data.providerCount} ${t('usage_stats.provider_model_graph.shared_badge', 'shared')}`
						: '',
				featuresText,
				embeddingText,
			}
			return {
				id: node.id,
				label,
				fill: modelFill(node.data.isShared, node.data.disabled, isDark),
				size: modelNodeSize(node.data.providerCount),
				subLabel: node.data.isShared ? t('usage_stats.provider_model_graph.shared_badge') : undefined,
				data: dataPayload,
			}
		})
		const mappedEdges: GraphEdge[] = graph.edges.map((edge: ProviderGraphEdge) => ({
			id: edge.id,
			source: edge.source,
			target: edge.target,
			interpolation: 'curved',
			arrowPlacement: 'none',
		}))
		return { nodes: mappedNodes, edges: mappedEdges }
	}, [graph, isDark, t])

	const handleNodePointerOver = useCallback(
		(node: InternalGraphNode) => {
			const n = graph.nodes.find(x => x.id === node.id)
			const payload: ReagraphNodeData = (node.data as unknown as ReagraphNodeData | undefined) ?? { kindLabel: '' }
			setTooltip({
				x: 16,
				y: 16,
				title: node.label ?? node.id,
				kind:
					n?.data.type === 'provider'
						? t('usage_stats.provider_model_graph.tooltip_models', { count: n?.data.modelCount ?? 0 })
						: t('usage_stats.provider_model_graph.shared_badge'),
				extra: [payload.featuresText, payload.embeddingText].filter(Boolean).join(' | '),
			})
		},
		[graph, t],
	)

	const handleNodePointerOut = useCallback(() => setTooltip(null), [])

	return (
		<div className={styles.canvas} ref={containerRef}>
			<GraphCanvas
				nodes={nodes}
				edges={edges}
				layoutType="forceDirected2d"
				draggable
				animated
				labelType="auto"
				sizingType="attribute"
				sizingAttribute="size"
				onNodePointerOver={handleNodePointerOver}
				onNodePointerOut={handleNodePointerOut}
			/>
			{tooltip && (
				<div className={styles.tooltip} style={{ left: tooltip.x, top: tooltip.y }}>
					<div className={styles.tooltipTitle}>{tooltip.title}</div>
					{tooltip.kind && <div className={styles.tooltipBody}>{tooltip.kind}</div>}
					{tooltip.extra && <div className={styles.tooltipBody}>{tooltip.extra}</div>}
				</div>
			)}
		</div>
	)
}

// Standalone panel: fetches the GNN response itself and renders the full
// header/summary/state chrome, then delegates the canvas to the shared view.
export function ProviderModelReagraphPanel() {
	const { t } = useTranslation()
	const [data, setData] = useState<ProviderModelGraphResponse | null>(null)
	const [error, setError] = useState<string | null>(null)
	const [loading, setLoading] = useState(true)

	useEffect(() => {
		const controller = new AbortController()
		fetchProviderModelGNN(controller.signal)
			.then(res => {
				if (!controller.signal.aborted) setData(res)
			})
			.catch(err => {
				if (controller.signal.aborted) return
				setError(err instanceof Error ? err.message : String(err))
			})
			.finally(() => {
				if (!controller.signal.aborted) setLoading(false)
			})
		return () => controller.abort()
	}, [])

	const graph = useMemo(() => (data ? buildProviderModelGraph(data) : null), [data])

	return (
		<div className={styles.panel}>
			<div className={styles.header}>
				<div>
					<h2 className={styles.title}>{t('usage_stats.provider_model_graph.title')}</h2>
					<p className={styles.subtitle}>{t('usage_stats.provider_model_graph.subtitle')}</p>
				</div>
				{!loading && !error && graph ? (
					<div className={styles.summary}>
						{t('usage_stats.provider_model_graph.summary', {
							providers: graph.meta?.providerCount ?? graph.providerCount,
							models: graph.meta?.edgeCount ?? graph.edgeCount,
							dims: graph.meta?.featureDim ?? 0,
						})}
					</div>
				) : null}
			</div>
			{loading ? (
				<div className={styles.state}>{t('usage_stats.provider_model_graph.loading')}</div>
			) : error ? (
				<div className={`${styles.state} ${styles.stateError}`}>
					{t('usage_stats.provider_model_graph.error', { message: error })}
				</div>
			) : !graph || graph.nodes.length === 0 ? (
				<div className={styles.state}>{t('usage_stats.provider_model_graph.empty')}</div>
			) : (
				<ProviderModelReagraphCanvas graph={graph} />
			)}
		</div>
	)
}
