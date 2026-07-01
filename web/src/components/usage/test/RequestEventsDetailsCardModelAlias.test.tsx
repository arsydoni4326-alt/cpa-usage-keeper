import React from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { RequestEventsDetailsCard } from '../RequestEventsDetailsCard';
import type { UsageEvent } from '@/lib/types';

const events: UsageEvent[] = [
  {
    id: '101',
    timestamp: '2026-04-23T02:00:00.000Z',
    api_key: 'Production Key',
    model: 'claude-sonnet',
    model_alias: 'sonnet-business',
    source: 'Provider A',
    source_raw: 'source-a',
    source_type: 'openai',
    auth_index: '1',
    failed: false,
    latency_ms: 120,
    ttft_ms: 45,
    speed_tps: 30,
    tokens: {
      input_tokens: 100,
      output_tokens: 60,
      reasoning_tokens: 20,
      cached_tokens: 20,
      cache_read_tokens: 20,
      cache_creation_tokens: 0,
      total_tokens: 200,
    },
    cost_usd: 0.1234,
    cost_available: true,
    pricing_style: 'claude',
  },
];

const renderCard = (props: Partial<React.ComponentProps<typeof RequestEventsDetailsCard>> = {}) =>
  renderToStaticMarkup(
    <RequestEventsDetailsCard
      events={events}
      loading={false}
      page={1}
      pageSize={20}
      pageSizeOptions={[20, 50, 100, 500, 1000]}
      totalCount={1}
      totalPages={1}
      modelOptions={['claude-sonnet']}
      sourceOptions={[{ value: 'source-a', label: 'Provider A' }]}
      modelFilter="__all__"
      sourceFilter="__all__"
      resultFilter="__all__"
      onPageChange={() => undefined}
      onPageSizeChange={() => undefined}
      onModelFilterChange={() => undefined}
      onSourceFilterChange={() => undefined}
      onResultFilterChange={() => undefined}
      {...props}
    />,
  );

describe('RequestEventsDetailsCard model alias column', () => {
  it('shows model alias after model by default', () => {
    const html = renderCard();
    const modelHeaderIndex = html.indexOf('>Model</th>');
    const modelAliasHeaderIndex = html.indexOf('>Model Alias</th>');
    const effortHeaderIndex = html.indexOf('title="Reasoning Effort">Effort</th>');

    expect(modelHeaderIndex).toBeGreaterThanOrEqual(0);
    expect(modelAliasHeaderIndex).toBeGreaterThanOrEqual(0);
    expect(effortHeaderIndex).toBeGreaterThanOrEqual(0);
    expect(modelHeaderIndex).toBeLessThan(modelAliasHeaderIndex);
    expect(modelAliasHeaderIndex).toBeLessThan(effortHeaderIndex);
    expect(html).toMatch(/<td class="[^"]*modelCell[^"]*" title="sonnet-business">sonnet-business<\/td>/);
  });

  it('renders a dash when model alias is missing', () => {
    const html = renderCard({
      events: [{ ...events[0], model_alias: '' }],
    });

    expect(html).toMatch(/claude-sonnet<\/td><td class="[^"]*modelCell[^"]*" title="-">-<\/td>/);
  });
});
