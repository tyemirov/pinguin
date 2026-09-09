import { expect, test } from '@playwright/test';
import { configureRuntime, resetNotifications, stubExternalAssets } from './utils';

for (const width of [390, 1280]) {
  for (const path of ['/index.html', '/tenants.html', '/event-log.html', '/smtp-relay.html']) {
    test(`the current shared candidate preserves ${path} at ${width}px`, async ({ page, request }) => {
      await resetNotifications(request);
      await stubExternalAssets(page);
      await configureRuntime(page, { authenticated: path !== '/index.html' });
      await page.setViewportSize({ width, height: 900 });
      await page.goto(path);
      await page.evaluate(() => (window as any).MPRUI.whenAutoOrchestrationReady());
      await expect(page.locator('mpr-header')).toHaveAttribute('auth-config', /"providers"/);
      await expect(page).toHaveURL(new RegExp(path.replace('.', '\\.') + '$'));
      const footer = page.locator('mpr-footer');
      const menu = footer.getByRole('button', { name: 'Built By Marco Polo Research Lab', exact: true });
      await expect(menu).toBeVisible();
      await menu.focus();
      await page.keyboard.press('Enter');
      await expect(menu).toHaveAttribute('aria-expanded', 'true');
      await expect(footer.getByRole('link', { name: 'Pinguin', exact: true })).toHaveAttribute('href', 'https://pinguin.mprlab.com/');
      await page.keyboard.press('Escape');
      await expect(menu).toBeFocused();
      const theme = await page.locator('body').getAttribute('data-theme');
      await footer.locator('[data-mpr-theme-toggle="control"]').click();
      await expect(page.locator('body')).not.toHaveAttribute('data-theme', theme!);
      if (path === '/index.html') {
        await expect(page.locator('[data-mpr-auth-action="google"] button')).toBeVisible();
      } else {
        await expect(page.locator('mpr-header [data-mpr-user="trigger"]')).toBeVisible();
      }
    });
  }
}
