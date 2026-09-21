// Feature-local API client for the Provider Model GNN domain. Kept inside the
// feature folder (like web/src/features/ranking/api.ts) so the whole feature
// is independently mergeable from upstream.
import { apiFetch, apiPath, ApiError } from '@/lib/api'
import type { ProviderModelGraphResponse } from './types'

export async function fetchProviderModelGNN(signal?: AbortSignal): Promise<ProviderModelGraphResponse> {
  const response = await apiFetch(apiPath('/provider-model-gnn'), { signal, cache: 'no-store' })
  if (!response.ok) {
    let message = `Failed to load provider model GNN: ${response.status}`
    try {
      const payload = await response.json() as { error?: string }
      if (payload.error) message = payload.error
    } catch {
      // ignore invalid error payloads
    }
    throw new ApiError(message, response.status)
  }
  return response.json()
}