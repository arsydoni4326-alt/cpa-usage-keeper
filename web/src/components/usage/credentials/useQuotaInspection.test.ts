import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'
import { shouldContinueQuotaInspectionPolling } from './useQuotaInspection'

const quotaInspectionSource = readFileSync(new URL('./useQuotaInspection.ts', import.meta.url), 'utf8').replace(/\r\n/g, '\n')

describe('useQuotaInspection polling', () => {
  it('polls only while an inspection round is actively running', () => {
    expect(shouldContinueQuotaInspectionPolling({ running: true, completed: false })).toBe(true)
    expect(shouldContinueQuotaInspectionPolling({ running: true, completed: true })).toBe(false)
    expect(shouldContinueQuotaInspectionPolling({ running: false, completed: false })).toBe(false)
    expect(shouldContinueQuotaInspectionPolling(null)).toBe(false)
  })

  it('does not schedule polling from the initial enabled status load', () => {
    const start = quotaInspectionSource.indexOf('const loadInitialInspectionStatus = async () => {')
    const end = quotaInspectionSource.indexOf('void loadInitialInspectionStatus()')

    expect(start).toBeGreaterThanOrEqual(0)
    expect(end).toBeGreaterThan(start)

    const initialLoadBlock = quotaInspectionSource.slice(start, end)
    expect(initialLoadBlock).not.toContain('setTimeout')
    expect(initialLoadBlock).not.toContain('pollQuotaInspectionStatus')
  })
})
