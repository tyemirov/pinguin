import { expect, type Page } from '@playwright/test';
import { createHash } from 'node:crypto';
import candidate from './shared-ui-candidate.json';

let assetPromise: Promise<Map<string, Buffer>>;
async function downloadCandidate() {
  const assets = new Map<string, Buffer>();
  for (const [name, digest] of Object.entries(candidate.assets)) {
    const response = await fetch(`https://raw.githubusercontent.com/MarcoPoloResearchLab/mpr-ui/${candidate.revision}/${name}`, { signal: AbortSignal.timeout(15000) });
    expect(response.ok).toBe(true);
    const bytes = Buffer.from(await response.arrayBuffer());
    expect(createHash('sha256').update(bytes).digest('hex')).toBe(digest);
    assets.set(name, bytes);
  }
  return assets;
}

export async function useSharedCandidate(page: Page) {
  assetPromise ??= downloadCandidate();
  for (const [name, body] of await assetPromise) {
    await page.route(url => url.pathname.endsWith(`/${name}`), route => route.fulfill({
      body, contentType: name.endsWith('.css') ? 'text/css' : 'application/javascript',
    }));
  }
}
