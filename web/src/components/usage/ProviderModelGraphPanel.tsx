import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ReactFlow, Background, Controls, MiniMap } from '@xyflow/react'
import '@xyflow/react/dist/style.css'

import { fetchProviderModelGraph } from '@/lib/api'
import type { ProviderModelGraphResponse } from '@/lib/types'

import { buildProviderModelGraph } from './providerModelGraph'
import styles from './ProviderModelGraphPanel.module.scss'

export function ProviderModelGraphPanel() {
	const { t } = useTranslation()
	const abortRef = useRef<AbortController | null>(null)
	const [data, setData] = useState<ProviderModelGraphResponse | null>(null)
	const [loading, setLoading] = useState(true)
	const [error, setError] = useState<string | null>(null)

	useEffect(() => {
		const controller = new AbortController()
		abortRef.current = controller
		fetchProviderModelGraph(controller.signal)
			.then((payload) => {
				setData(payload)
			})
			.catch((err: unknown) => {
				if ((err as Error)?.name === 'AbortError') return
				setError(err instanceof Error ? err.message : String(err))
			})
			.finally(() => {
				setLoading(false)
			})
		return () => {
			controller.abort()
		}
	}, [])

	const graph = useMemo(() => (data ? buildProviderModelGraph(data) : null), [data])

	const summary = useMemo(() => {
		if (!graph) return { providers: 0, models: 0, shared: 0 }
		const providers = graph.nodes.filter((node) => node.data.type === 'provider').length
		const models = graph.nodes.filter((node) => node.data.type === 'model').length
		const shared = graph.nodes.filter(
			(node) => node.data.type === 'model' && node.data.providerCount > 1,
		).length
		return { providers, models, shared }
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
					nodes={graph.nodes}
					edges={graph.edges}
					fitView
					nodesDraggable={false}
					nodesConnectable={false}
					elementsSelectable={false}
					defaultEdgeOptions={{ type: 'smoothstep' }}
				>
					<Background gap={16} />
					<Controls />
					<MiniMap
						pannable
						zoomable
						nodeColor={(node) => (node.data?.type === 'provider' ? '#94a3b8' : '#38bdf8')}
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
				{!loading && !error && summary.providers > 0 ? (
					<div className={styles.summary}>
						{t('usage_stats.provider_model_graph.summary', {
							providers: summary.providers,
							models: summary.models,
							shared: summary.shared,
						})}
					</div>
				) : null}
			</div>
			{body}
		</div>
	)
}
