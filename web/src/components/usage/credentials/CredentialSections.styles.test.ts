import { readFileSync } from 'node:fs'
import { describe, expect, it } from 'vitest'

const credentialStyles = readFileSync(new URL('./CredentialSections.module.scss', import.meta.url), 'utf8')
const authFileSectionSource = readFileSync(new URL('./AuthFileCredentialsSection.tsx', import.meta.url), 'utf8')
const aiProviderSectionSource = readFileSync(new URL('./AiProviderCredentialsSection.tsx', import.meta.url), 'utf8')

describe('Credential section styles', () => {
  it('keeps Auth Files metric and quota columns at requested widths', () => {
    expect(credentialStyles).toMatch(/\.credentialMetricGroup\s*\{[\s\S]*?grid-template-columns:\s*120px repeat\(3, 95px\);/)
    expect(credentialStyles).toMatch(/\.credentialRow\s*\{[\s\S]*?grid-template-columns:\s*minmax\(170px, 250px\) minmax\(405px, max-content\) minmax\(220px, 1fr\);/)
    expect(credentialStyles).toMatch(/\.credentialQuotaRow\s*\{[\s\S]*?grid-template-columns:\s*minmax\(170px, 250px\) minmax\(405px, max-content\) 170px;/)
    expect(credentialStyles).toMatch(/\.credentialQuotaSidePanel\s*\{[\s\S]*?max-width:\s*170px;/)
    expect(credentialStyles).toMatch(/\.credentialQuotaSideWithAction\s*\{[\s\S]*?gap:\s*14px;/)
    expect(credentialStyles).toMatch(/\.credentialQuotaBars\s*\{[\s\S]*?gap:\s*12px;/)
  })

  it('applies quota-only layout classes only to Auth Files rows', () => {
    expect(authFileSectionSource).toContain('rowClassName={styles.credentialQuotaRow}')
    expect(authFileSectionSource).toContain('sideClassName={styles.credentialQuotaSidePanel}')
    expect(aiProviderSectionSource).not.toContain('credentialQuotaRow')
    expect(aiProviderSectionSource).not.toContain('credentialQuotaSidePanel')
    expect(aiProviderSectionSource).not.toContain('credentialQuotaSideWithAction')
  })

  it('keeps plan and remaining-day badges readable in dark mode', () => {
    expect(credentialStyles).toMatch(/\[data-theme='dark'\][\s\S]*\.credentialPlanBadgeTeam[\s\S]*?color:\s*#bbf7d0;/)
    expect(credentialStyles).toMatch(/\[data-theme='dark'\][\s\S]*\.credentialPlanBadgePlus[\s\S]*?color:\s*#bfdbfe;/)
    expect(credentialStyles).toMatch(/\[data-theme='dark'\][\s\S]*\.credentialPlanBadgePro[\s\S]*?color:\s*#fde68a;/)
    expect(credentialStyles).toMatch(/\[data-theme='dark'\][\s\S]*\.credentialRemainingDaysBadge[\s\S]*?color:\s*#bbf7d0;/)
  })
})
