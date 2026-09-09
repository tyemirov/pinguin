import { expect, test } from '@playwright/test';
import { configureRuntime, resetNotifications, stubExternalAssets } from './utils';

for (const width of [390, 1280]) {
  test(`protected reads and tenant creation recover once at ${width}px`, async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
    await configureRuntime(page, { authenticated: true });
    await page.setViewportSize({ width, height: 900 });
    let reads = 0;
    let sessions = 0;
    const submissions: Array<{ body: string | null; key: string | null }> = [];
    page.on('request', request => {
      if (new URL(request.url()).pathname === '/auth/session') sessions += 1;
    });
    await page.route('**/api/tenants', async route => {
      if (route.request().method() === 'GET') {
        reads += 1;
        if (reads === 1) {
          await route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"session_required"}' });
          return;
        }
      }
      if (route.request().method() === 'POST') {
        submissions.push({ body: route.request().postData(), key: await route.request().headerValue('idempotency-key') });
        if (submissions.length === 1) {
          await route.fulfill({ status: 401, contentType: 'application/json', body: '{"error":"session_required"}' });
          return;
        }
      }
      await route.continue();
    });
    await page.goto('/tenants.html');
    await expect(page.getByTestId('tenant-card')).toHaveCount(2);
    expect(reads).toBe(2);
    expect(sessions).toBe(2);
    await page.getByRole('button', { name: 'Create tenant' }).click();
    const dialog = page.getByRole('dialog', { name: 'Create tenant' });
    await dialog.getByLabel('Display name').fill('Recovered tenant');
    await dialog.getByLabel('Support email').fill('support@recovery.example');
    await dialog.getByLabel('SMTP host').fill('smtp.recovery.example');
    await dialog.getByLabel('SMTP port').fill('587');
    await dialog.getByLabel('SMTP username').fill('recovery-user');
    await dialog.getByLabel('SMTP password').fill('fixture-password');
    await dialog.getByLabel('From address').fill('notify@recovery.example');
    await dialog.getByRole('button', { name: 'Create tenant' }).click();
    const keyDialog = page.getByRole('dialog', { name: 'Copy the new API key' });
    await expect(keyDialog).toBeVisible();
    expect(submissions).toHaveLength(2);
    expect(submissions[1]).toEqual(submissions[0]);
    expect(submissions[0].key).toBeTruthy();
    expect(sessions).toBe(3);
    await keyDialog.getByRole('button', { name: 'Close' }).click();
    await expect(page.getByTestId('tenant-card').filter({ hasText: 'Recovered tenant' })).toHaveCount(1);
    await page.reload();
    await expect(page.getByTestId('tenant-card')).toHaveCount(3);
    await expect(page.getByTestId('tenant-card').filter({ hasText: 'Recovered tenant' })).toHaveCount(1);
    await page.locator('mpr-header [data-mpr-user="trigger"]').click();
    await page.locator('mpr-header [data-mpr-user="logout"]').click();
    await expect(page).toHaveURL(new URL('/', page.url()).href);
    await page.reload();
    await expect(page.locator('[data-mpr-auth-action="google"] button')).toBeVisible();
  });
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
