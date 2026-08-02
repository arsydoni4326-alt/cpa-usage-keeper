import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Background, Controls, MiniMap, Position, ReactFlow } from '@xyflow/react'
import '@xyflow/react/dist/style.css'

import { fetchProviderModelGraph } from '@/lib/api'
import type { ProviderModelGraphResponse } from '@/lib/types'

import styles from './ProviderModelGraphPanel.module.scss'
import { buildProviderModelGraph, type ProviderGraphNode } from './providerModelGraph'

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

export function ProviderModelGraphPanel() {
	const { t } = useTranslation()
	const [data, setData] = useState<ProviderModelGraphResponse | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState<string | null>(null)

	useEffect(() => {
		const controller = new AbortController()
		fetchProviderModelGraph(controller.signal)
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
			const baseStyle = isProvider
				? disabled
					? PROVIDER_DISABLED_STYLE
					: PROVIDER_STYLE
				: disabled
					? MODEL_DISABLED_STYLE
					: MODEL_STYLE

			// Tooltip with full raw name + alias for hover
			const title = isProvider
				? `${node.data.label} • ${node.data.kind} • ${node.data.modelCount} model(s)`
				: (() => {
						const d = node.data as {
							label: string
							name: string
							provider: string
							providerCount: number
						}
						return d.name && d.name !== d.label
							? `${d.label}  —  raw: ${d.name}  —  via ${d.provider}${d.providerCount > 1 ? `  (+${d.providerCount - 1} more)` : ''}`
							: `${d.label}  —  via ${d.provider}`
					})()

			return {
				...node,
				sourcePosition: Position.Right,
				targetPosition: Position.Left,
				style: baseStyle,
				data: {
					...node.data,
					label: <span title={title}>{node.data.label}</span>,
				},
			} as ProviderGraphNode
		})
	}, [graph])

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
	} else {
		body = (
			<div className={styles.canvas}>
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
						nodeColor={(node) =>
							(node as ProviderGraphNode).data.type === 'provider' ? '#60a5fa' : '#cbd5e1'
						}
					/>
				</ReactFlow>
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
				{!loading && !error && graph ? (
					<div className={styles.summary}>
						{t('usage_stats.provider_model_graph.summary', {
							providers: graph.providerCount,
							models: graph.edgeCount,
						})}
					</div>
				) : null}
			</div>
			{body}
		</div>
	)
}
