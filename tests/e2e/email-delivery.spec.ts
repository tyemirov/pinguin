import { expect, test, type Page } from '@playwright/test';
import { configureRuntime, resetNotifications, stubExternalAssets } from './utils';

function delivery(id = 'ses-original', subject = 'Catalog review') {
  const sent = new Date(Date.now() - 60_000).toISOString();
  return {
    id, from: 'sales@tenant.example', subject, sent_at: sent, updated_at: sent,
    expires_at: new Date(Date.now() + 86_340_000).toISOString(),
    recipients: [{ address: 'client@outside.example', status: 'delivered', updated_at: sent, response: '250 Accepted' }],
    forwarded_message_ids: [],
  };
}
function payload(messages: unknown[]) {
  return { messages, window_start: new Date(Date.now() - 86_400_000).toISOString(), as_of: new Date().toISOString() };
}
async function setup(page: Page) {
  await stubExternalAssets(page);
  await configureRuntime(page, { authenticated: true });
}

test.describe('Email delivery', () => {
  test.beforeEach(async ({ request }) => resetNotifications(request));

  for (const trigger of ['poll', 'online', 'focus', 'pageshow', 'visibilitychange']) {
    test(`recovers tenant discovery after a transient failure on ${trigger}`, async ({ page }) => {
      await setup(page);
      await page.clock.install();
      let tenantsAvailable = false;
      await page.route('**/api/tenants', route => tenantsAvailable
        ? route.continue()
        : route.fulfill({ status: 503, json: { error: { message: 'Temporarily unavailable' } } }));
      await page.route('**/api/tenants/*/email-deliveries?*', route => route.fulfill({ json: payload([delivery()]) }));
      await page.goto('/email-delivery.html');
      await expect(page.getByRole('alert')).toHaveText('Unable to load tenants.');
      await expect(page.getByLabel('Tenant', { exact: true })).toBeDisabled();
      tenantsAvailable = true;
      if (trigger === 'poll') await page.clock.fastForward(15_000);
      else await page.evaluate(eventName => {
        const target = eventName === 'visibilitychange' ? document : window;
        target.dispatchEvent(new Event(eventName));
      }, trigger);
      await expect(page.getByText('Catalog review', { exact: true })).toBeVisible();
      await expect(page.getByLabel('Tenant', { exact: true })).toBeEnabled();
      await expect(page.getByRole('alert')).toHaveCount(0);
    });
  }

  test('shows generic recipient outcomes and a forwarding chain on a narrow screen', async ({ page }, testInfo) => {
    await setup(page);
    const original = delivery();
    original.recipients.push({ address: 'shared@tenant.example', status: 'delivered', updated_at: original.sent_at, response: '250 Forwarding accepted' });
    original.forwarded_message_ids = ['ses-forward'];
    const forwarded = { ...delivery('ses-forward'), from: 'shared@tenant.example', parent_message_id: original.id,
      recipients: [{ address: 'owner@destination.example', status: 'unconfirmed', updated_at: original.sent_at, response: '' }] };
    await page.route('**/api/tenants/*/email-deliveries?*', route => route.fulfill({ json: payload([original, forwarded]) }));
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto('/email-delivery.html');
    await expect(page.getByRole('heading', { name: 'Email delivery', exact: true })).toBeVisible();
    const originalRow = page.getByTestId('email-delivery-record').first();
    await expect(originalRow.locator('summary').first()).toContainText('2 Delivered');
    await expect(originalRow.locator('.delivery-response').first()).not.toBeVisible();
    expect((await originalRow.boundingBox())!.height).toBeLessThan(96);
    await originalRow.locator('summary').first().focus();
    await page.keyboard.press('Enter');
    await expect(page.getByText('client@outside.example', { exact: true })).toBeVisible();
    await expect(page.getByText('Delivered', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('1 Unconfirmed', { exact: true })).toBeVisible();
    await expect(page.getByRole('link', { name: 'ses-forward', exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: /refresh|save/i })).toHaveCount(0);
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.screenshot({ path: testInfo.outputPath('email-delivery-mobile.png'), fullPage: true });
    await originalRow.locator('summary').first().click();
    await page.setViewportSize({ width: 1280, height: 1100 });
    expect((await originalRow.boundingBox())!.height).toBeLessThan(72);
    expect((await page.locator('.delivery-toolbar').boundingBox())!.height).toBeLessThan(64);
    await page.getByRole('heading', { name: 'Email delivery', exact: true }).click();
    await page.getByTestId('page-work-surface').screenshot({ path: testInfo.outputPath('email-delivery-compact.png') });
    await page.screenshot({ path: testInfo.outputPath('email-delivery-desktop.png'), fullPage: true });
  });

  test('shows recipient-specific failure and delay diagnostics without losing an earlier acceptance', async ({ page }) => {
    await setup(page);
    const message = delivery();
    message.recipients = [
      { address: 'bounced@outside.example', status: 'bounced', updated_at: message.sent_at, response: '550 No such mailbox' },
      { address: 'slow@outside.example', status: 'delayed', updated_at: message.sent_at, response: '421 Try again later' },
      { address: 'complaint@outside.example', status: 'complaint', updated_at: message.sent_at, response: 'Abuse report', delivered_at: message.sent_at },
    ];
    await page.route('**/api/tenants/*/email-deliveries?*', route => route.fulfill({ json: payload([message]) }));
    await page.goto('/email-delivery.html');
    await expect(page.getByText('1 Bounced', { exact: true })).toBeVisible();
    await expect(page.getByText('1 Delayed', { exact: true })).toBeVisible();
    await expect(page.getByText('1 Complaint', { exact: true })).toBeVisible();
    await expect(page.getByText('550 No such mailbox', { exact: true })).not.toBeVisible();
    await page.getByTestId('email-delivery-record').locator('summary').first().click();
    await expect(page.getByText('Bounced', { exact: true })).toBeVisible();
    await expect(page.getByText('550 No such mailbox', { exact: true })).toBeVisible();
    await expect(page.getByText('Delayed', { exact: true })).toBeVisible();
    await expect(page.getByText('421 Try again later', { exact: true })).toBeVisible();
    await expect(page.getByText('Complaint', { exact: true })).toBeVisible();
    await expect(page.locator('.delivery-recipients li').filter({ hasText: 'complaint@outside.example' }).getByText('Accepted earlier:', { exact: false })).toBeVisible();
  });

  test('distinguishes empty history from unavailable and malformed provider responses', async ({ page }) => {
    await setup(page);
    let state = 'empty';
    await page.route('**/api/tenants/*/email-deliveries?*', route => state === 'empty'
      ? route.fulfill({ json: payload([]) })
      : state === 'unavailable' ? route.fulfill({ status: 503, json: { error: { code: 'delivery.unavailable', message: 'unavailable' } } })
        : route.fulfill({ json: payload([{ id: 'broken' }]) }));
    await page.goto('/email-delivery.html');
    await expect(page.getByText('No matching emails in the last 24 hours.')).toBeVisible();
    state = 'unavailable';
    await page.evaluate(() => window.dispatchEvent(new Event('online')));
    await expect(page.getByRole('alert')).toContainText('Delivery history is unavailable');
    await expect(page.getByText('No matching emails in the last 24 hours.')).toHaveCount(0);
    state = 'malformed';
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect(page.getByRole('alert')).toContainText('Unable to load delivery history');
    state = 'empty';
    await page.evaluate(() => window.dispatchEvent(new Event('online')));
    await expect(page.getByText('No matching emails in the last 24 hours.')).toBeVisible();
    await expect(page.getByRole('alert')).toHaveCount(0);
  });

  test('clears old tenant data and ignores a delayed response after tenant changes', async ({ page }) => {
    await setup(page);
    let release: (() => void) | undefined;
    let holdRequests = false;
    await page.route('**/api/tenants/*/email-deliveries?*', async route => {
      if (route.request().url().includes('tenant-devserver')) {
        if (holdRequests) await new Promise<void>(resolve => { release = resolve; });
        await route.fulfill({ json: payload([delivery('first', 'First tenant private email')]) });
      } else await route.fulfill({ json: payload([delivery('second', 'Second tenant email')]) });
    });
    await page.goto('/email-delivery.html');
    await expect(page.getByText('First tenant private email', { exact: true })).toBeVisible();
    holdRequests = true;
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect.poll(() => Boolean(release)).toBe(true);
    await page.getByLabel('Tenant', { exact: true }).selectOption('tenant-archive');
    await expect(page.getByText('First tenant private email', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Second tenant email', { exact: true })).toBeVisible();
    release?.();
    await expect(page.getByText('First tenant private email', { exact: true })).toHaveCount(0);
  });

  test('automatically polls, searches, reconnects and expires visible records', async ({ page }) => {
    await setup(page);
    const fixture = delivery();
    fixture.expires_at = new Date(Date.now() + 10_000).toISOString();
    const queries: string[] = [];
    await page.route('**/api/tenants/*/email-deliveries?*', route => {
      queries.push(new URL(route.request().url()).searchParams.get('q') || '');
      return route.fulfill({ json: payload([fixture]) });
    });
    await page.clock.install();
    await page.goto('/email-delivery.html');
    await expect(page.getByText('Catalog review', { exact: true })).toBeVisible();
    await page.getByRole('searchbox', { name: 'Search emails' }).fill('owner@destination.example');
    await page.clock.fastForward(400);
    await expect.poll(() => queries.includes('owner@destination.example')).toBe(true);
    const count = queries.length;
    await page.clock.fastForward(15_000);
    await expect.poll(() => queries.length).toBeGreaterThan(count);
    await expect(page.getByText('Catalog review', { exact: true })).toHaveCount(0);
    const afterPoll = queries.length;
    await page.evaluate(() => window.dispatchEvent(new Event('online')));
    await expect.poll(() => queries.length).toBeGreaterThan(afterPoll);
  });

  test('keeps loaded pages current and clears mail immediately when the account changes', async ({ page }) => {
    await setup(page);
    let accountChanged = false;
    let secondPageCalls = 0;
    await page.route('**/api/tenants/*/email-deliveries?*', route => {
      const cursor = new URL(route.request().url()).searchParams.get('cursor');
      if (cursor) secondPageCalls += 1;
      return route.fulfill({ json: { ...payload(accountChanged ? [] : [delivery(cursor ? 'older' : 'newer', cursor ? 'Older email' : 'Newer email')]),
        ...(cursor || accountChanged ? {} : { next_cursor: 'next-test-page' }) } });
    });
    await page.goto('/email-delivery.html');
    await expect(page.getByText('Newer email', { exact: true })).toBeVisible();
    await page.getByRole('button', { name: 'More emails' }).click();
    await expect(page.getByText('Older email', { exact: true })).toBeVisible();
    await page.evaluate(() => window.dispatchEvent(new Event('focus')));
    await expect.poll(() => secondPageCalls).toBe(2);
    await expect(page.getByText('Older email', { exact: true })).toBeVisible();
    accountChanged = true;
    await page.evaluate(() => window.Alpine.store('auth').setProfile({ user_email: 'other@example.com', user_id: 'other-user' }));
    await expect(page.getByText('Older email', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Newer email', { exact: true })).toHaveCount(0);
    await expect(page.getByText('No matching emails in the last 24 hours.')).toBeVisible();
    await page.evaluate(() => window.Alpine.store('auth').clear());
    await expect(page).toHaveURL(/\/(?:index\.html)?$/);
  });

  test('redirects unauthenticated visitors', async ({ page }) => {
    await stubExternalAssets(page);
    await configureRuntime(page, { authenticated: false });
    await page.goto('/email-delivery.html');
    await expect(page).toHaveURL(/\/(?:index\.html)?$/);
  });
});
