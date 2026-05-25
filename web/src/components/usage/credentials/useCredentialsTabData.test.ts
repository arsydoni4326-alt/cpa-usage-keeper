import { describe, expect, it } from 'vitest'
import { quotaRefreshDisplayError } from './useCredentialsTabData'
import { CREDENTIAL_PAGES_REFRESH_INTERVAL_MS } from './useCredentialPages'
import { buildQuotaCacheAuthIndexesKey, QUOTA_CACHE_REFRESH_INTERVAL_MS } from './useQuotaCache'

describe('Credentials polling intervals', () => {
  it('keeps list data on a 5 minute refresh interval', () => {
    expect(CREDENTIAL_PAGES_REFRESH_INTERVAL_MS).toBe(5 * 60 * 1000)
  })

  it('keeps quota cache on a 1 minute refresh interval', () => {
    expect(QUOTA_CACHE_REFRESH_INTERVAL_MS).toBe(60 * 1000)
  })
})

describe('buildQuotaCacheAuthIndexesKey', () => {
  it('keeps equal auth index lists stable across array references', () => {
    expect(buildQuotaCacheAuthIndexesKey(['auth-1', 'auth-2'])).toBe(buildQuotaCacheAuthIndexesKey(['auth-1', 'auth-2']))
  })

  it('changes when auth index contents or order changes', () => {
    expect(buildQuotaCacheAuthIndexesKey(['auth-1', 'auth-2'])).not.toBe(buildQuotaCacheAuthIndexesKey(['auth-2', 'auth-1']))
  })
})

describe('quotaRefreshDisplayError', () => {
  it('turns refresh rejection codes into friendly messages', () => {
    expect(quotaRefreshDisplayError('duplicate')).toBe('Quota refresh is already running for this credential.')
    expect(quotaRefreshDisplayError('not_auth_file')).toBe('Quota refresh only supports local auth files.')
    expect(quotaRefreshDisplayError('unsupported')).toBe('Quota refresh is not supported for this credential type.')
  })

  it('keeps backend friendly refresh failures displayable', () => {
    expect(quotaRefreshDisplayError('Quota refresh timed out. Please try again later.')).toBe('Quota refresh timed out. Please try again later.')
  })
})
