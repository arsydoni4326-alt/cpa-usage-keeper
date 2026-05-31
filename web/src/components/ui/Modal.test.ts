import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

const modalSource = readFileSync(new URL('./Modal.tsx', import.meta.url), 'utf8').replace(/\r\n/g, '\n');

describe('Modal scroll lock', () => {
  it('does not mutate body layout while the modal is open', () => {
    expect(modalSource).not.toContain('body.style.position');
    expect(modalSource).not.toContain('body.style.top');
    expect(modalSource).not.toContain('body.style.width');
    expect(modalSource).not.toContain('body.style.overflow');
    expect(modalSource).toContain('handleOverlayWheel');
    expect(modalSource).toContain('handleOverlayTouchMove');
    expect(modalSource).toContain("target.closest('.modal-body')");
    expect(modalSource).toContain('window.scrollTo({ top: scrollY, left: 0, behavior: \'auto\' });');
  });
});
