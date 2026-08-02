import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const usagePageSource = readFileSync(new URL('../UsagePage.tsx', import.meta.url), 'utf8').replace(/\r\n/g, '\n');

describe('UsagePage CPAMC embed behavior', () => {
  it('detects CPAMC embed mode for per-tab session fallback', () => {
    expect(usagePageSource).toContain("import { isCPAMCEmbed } from '@/embed/cpamcEmbed';");
    expect(usagePageSource).toMatch(/const isEmbeddedInCPAMC = isCPAMCEmbed\(\);/);
  });

  it('does not render the Back to CPA link', () => {
    expect(usagePageSource).not.toContain('cpaManagementURL');
  });
});
