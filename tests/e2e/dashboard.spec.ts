import { expect, test, type Locator, type Page } from '@playwright/test';
import {
  configureRuntime,
  resetNotifications,
  stubExternalAssets,
  expectToast,
  expectHeaderGoogleButton,
  expectPinguinHeaderBrand,
} from './utils';

const LANDING_URL_PATTERN = /\/(?:index\.html)?$/;

async function expectClipboardText(page: Page, expectedText: string) {
  await expect
    .poll(() => page.evaluate(() => navigator.clipboard.readText()))
    .toBe(expectedText);
}

async function expectInputValueFits(input: Locator) {
  await expect
    .poll(() => input.evaluate((element) => element.scrollWidth <= element.clientWidth + 1))
    .toBe(true);
}

test.describe('Authenticated pages', () => {
  test.beforeEach(async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
  });

  test('redirects guests to the landing page', async ({ page }) => {
    await configureRuntime(page, { authenticated: false });
    await page.goto('/event-log.html');
    await expect(page).toHaveURL(LANDING_URL_PATTERN);
    await expectHeaderGoogleButton(page);
  });

  test('redirects guests from SMTP relay to the landing page', async ({ page }) => {
    await configureRuntime(page, { authenticated: false });
    await page.goto('/smtp-relay.html');
    await expect(page).toHaveURL(LANDING_URL_PATTERN);
    await expectHeaderGoogleButton(page);
  });

  test('shows Pinguin logo brand and favicon on the event log page', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expectPinguinHeaderBrand(page);
    await expect(page.getByTestId('notifications-table')).toBeVisible();
    await expect(page.getByRole('button', { name: 'Refresh' })).toHaveCount(1);
  });

  test('renders event log and SMTP relay as separate authenticated pages', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    const horizontalMenu = page.locator('mpr-header [data-mpr-header="horizontal-links"]');
    await expect(horizontalMenu).toBeVisible();
    const menuLinks = horizontalMenu.locator('a');
    await expect(menuLinks).toHaveText(['Event log', 'SMTP relay']);
    await expect(menuLinks.nth(0)).toHaveAttribute('href', '/event-log.html');
    await expect(menuLinks.nth(1)).toHaveAttribute('href', '/smtp-relay.html');
    await expect(page.getByRole('heading', { name: 'Event log' })).toBeVisible();
    await expect(page.getByTestId('notifications-table')).toBeVisible();
    await expect(page.getByTestId('smtp-identities')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Pinguin dashboard' })).toHaveCount(0);
    await expect(
      page.getByText('Monitor notification delivery events and manage SMTP relay access from one shared shell.'),
    ).toHaveCount(0);

    await page.goto('/smtp-relay.html');
    const smtpMenu = page.locator('mpr-header [data-mpr-header="horizontal-links"]');
    await expect(smtpMenu.locator('a')).toHaveText(['Event log', 'SMTP relay']);
    await expect(page.getByRole('heading', { name: 'SMTP relay' })).toBeVisible();
    await expect(page.getByTestId('smtp-identities')).toBeVisible();
    await expect(page.getByTestId('notifications-table')).toHaveCount(0);
    await expect(page.getByRole('heading', { name: 'Pinguin dashboard' })).toHaveCount(0);
    await expect(
      page.getByText('Monitor notification delivery events and manage SMTP relay access from one shared shell.'),
    ).toHaveCount(0);
  });

  test('redirects after BroadcastChannel logout', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await page.context().clearCookies();
    await page.evaluate(() => {
      const channel = new BroadcastChannel('auth');
      if (window.__mockAuth) {
        window.__mockAuth.authenticated = false;
        window.__persistMockAuth && window.__persistMockAuth();
      }
      channel.postMessage('logged_out');
      channel.close();
    });
    await expect(page).toHaveURL(/\/index\.html$/);
    await expectHeaderGoogleButton(page);
  });

  test('filters notifications by status selection', async ({ page, request }) => {
    const now = new Date();
    await resetNotifications(request, {
      notifications: [
        {
          notification_id: 'notif-q',
          notification_type: 'email',
          recipient: 'queued@example.com',
          subject: 'Queued',
          message: 'Hello',
          status: 'queued',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: now.toISOString(),
          retry_count: 0,
        },
        {
          notification_id: 'notif-c',
          notification_type: 'email',
          recipient: 'cancelled@example.com',
          subject: 'Cancelled',
          message: 'Hi',
          status: 'cancelled',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: now.toISOString(),
          retry_count: 0,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(2);
    const filterSelect = page.locator('label:has-text("Filter by status") select');
    await filterSelect.selectOption('queued');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.locator('.status-badge')).toHaveAttribute('data-variant', 'queued');
    await filterSelect.selectOption('cancelled');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.locator('.status-badge')).toHaveAttribute('data-variant', 'cancelled');
  });

  test('searches notifications by message body and resets results', async ({ page, request }) => {
    const now = new Date();
    await resetNotifications(request, {
      notifications: [
        {
          notification_id: 'notif-visible-body',
          notification_type: 'email',
          recipient: 'visible@example.com',
          subject: 'Visible body match',
          message: 'The rare launch phrase appears only in the body',
          status: 'queued',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: now.toISOString(),
          retry_count: 0,
        },
        {
          notification_id: 'notif-hidden',
          notification_type: 'email',
          recipient: 'hidden@example.com',
          subject: 'Other message',
          message: 'No matching words here',
          status: 'queued',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: now.toISOString(),
          retry_count: 0,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(2);

    await page.getByLabel('Search').fill('rare launch phrase');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.getByTestId('notifications-table')).toContainText('Visible body match');
    await expect(page.getByTestId('notifications-table')).not.toContainText('Other message');

    await page.getByLabel('Search').fill('');
    await expect(page.getByTestId('notification-row')).toHaveCount(2);
  });

  test('appends notification rows with infinite scroll', async ({ page, request }) => {
    const now = Date.now();
    const notifications = Array.from({ length: 60 }, (_, index) => ({
      notification_id: `notif-scroll-${index}`,
      notification_type: 'email',
      recipient: `scroll-${index}@example.com`,
      subject: `Scroll notification ${index}`,
      message: `Scroll body ${index}`,
      status: 'queued',
      created_at: new Date(now - index * 1000).toISOString(),
      updated_at: new Date(now - index * 1000).toISOString(),
      scheduled_for: new Date(now + index * 1000).toISOString(),
      retry_count: 0,
    }));
    await resetNotifications(request, { notifications });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(50);

    await page.getByTestId('notification-scroll-sentinel').scrollIntoViewIfNeeded();
    await expect(page.getByTestId('notification-row')).toHaveCount(60);
    await expect(page.getByTestId('notifications-table')).toContainText('Scroll notification 59');
  });

  test('switches notification views between tenants', async ({ page, request }) => {
    const now = new Date();
    await resetNotifications(request, {
      tenants: [
        { id: 'tenant-alpha', displayName: 'Alpha Corp' },
        { id: 'tenant-bravo', displayName: 'Bravo Labs' },
      ],
      notifications: [
        {
          notification_id: 'notif-alpha',
          tenant_id: 'tenant-alpha',
          notification_type: 'email',
          recipient: 'alpha@example.com',
          subject: 'Alpha event',
          message: 'Hello Alpha',
          status: 'queued',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: now.toISOString(),
          retry_count: 0,
        },
        {
          notification_id: 'notif-bravo',
          tenant_id: 'tenant-bravo',
          notification_type: 'email',
          recipient: 'bravo@example.com',
          subject: 'Bravo event',
          message: 'Hello Bravo',
          status: 'queued',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: now.toISOString(),
          retry_count: 0,
        },
      ],
    });
    await configureRuntime(page, {
      authenticated: true,
      tenant: { id: 'tenant-alpha', displayName: 'Alpha Corp' },
    });
    await page.goto('/event-log.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.getByTestId('notifications-table')).toContainText('Alpha event');
    await expect(page.getByTestId('notifications-table')).not.toContainText('Bravo event');

    await page.getByLabel('Tenant').selectOption('tenant-bravo');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.getByTestId('notifications-table')).toContainText('Bravo event');
    await expect(page.getByTestId('notifications-table')).not.toContainText('Alpha event');
  });

  test('uses the tenant list for the initial admin event log view', async ({ page, request }) => {
    const now = new Date();
    await resetNotifications(request, {
      tenants: [
        { id: 'loopaware', displayName: 'Loopaware' },
        { id: 'ps', displayName: 'Poodle Scanner' },
      ],
      notifications: [
        {
          notification_id: 'notif-loopaware',
          tenant_id: 'loopaware',
          notification_type: 'email',
          recipient: 'loopaware@example.com',
          subject: 'Loopaware event',
          message: 'Hello Loopaware',
          status: 'sent',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: null,
          retry_count: 0,
        },
        {
          notification_id: 'notif-ps',
          tenant_id: 'ps',
          notification_type: 'email',
          recipient: 'ps@example.com',
          subject: 'Poodle Scanner event',
          message: 'Hello Poodle Scanner',
          status: 'sent',
          created_at: now.toISOString(),
          updated_at: now.toISOString(),
          scheduled_for: null,
          retry_count: 0,
        },
      ],
    });
    await configureRuntime(page, {
      authenticated: true,
      tenant: { id: 'ps', displayName: 'PoodleScanner' },
    });
    await page.goto('/event-log.html');

    await expect(page.getByLabel('Tenant')).toHaveValue('loopaware');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.getByTestId('notifications-table')).toContainText('Loopaware event');
    await expect(page.getByTestId('notifications-table')).not.toContainText('Poodle Scanner event');
  });

  test('renders notification table and allows cancel', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expectToast(page, 'Notification cancelled');
  });

  test('shows error toast when list request fails', async ({ page, request }) => {
    await resetNotifications(request, { failList: true });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expect(page.locator('.notice[data-variant="error"]')).toHaveText('Unable to load notifications.');
    await expectToast(page, 'Unable to load notifications.');
  });

  test('reschedule flow updates toast', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await page.getByRole('button', { name: 'Reschedule' }).click();
    const input = page.getByLabel('Delivery time');
    const newDate = new Date(Date.now() + 7200 * 1000).toISOString().slice(0, 16);
    await input.fill(newDate);
    await page.getByRole('button', { name: 'Save changes' }).click();
    await expectToast(page, 'Delivery time updated');
  });

  test('shows toast when reschedule fails', async ({ page, request }) => {
    await resetNotifications(request, { failReschedule: true });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await page.getByRole('button', { name: 'Reschedule' }).click();
    const input = page.getByLabel('Delivery time');
    const newDate = new Date(Date.now() + 3600 * 1000).toISOString().slice(0, 16);
    await input.fill(newDate);
    await page.getByRole('button', { name: 'Save changes' }).click();
    await expectToast(page, 'Unable to reschedule notification.');
  });

  test('pre-fills reschedule dialog with existing scheduled time', async ({ page, request }) => {
    const scheduledFor = new Date('2030-01-02T03:04:00Z').toISOString();
    await resetNotifications(request, {
      notifications: [
        {
          notification_id: 'notif-prefill',
          notification_type: 'email',
          recipient: 'prefill@example.com',
          subject: 'Prefilled',
          message: 'Hello',
          status: 'queued',
          created_at: scheduledFor,
          updated_at: scheduledFor,
          scheduled_for: scheduledFor,
          retry_count: 0,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await page.getByRole('button', { name: 'Reschedule' }).click();
    const input = page.getByLabel('Delivery time');
    const pad = (value: number) => String(value).padStart(2, '0');
    const expected = (() => {
      const date = new Date(scheduledFor);
      return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(
        date.getMinutes(),
      )}`;
    })();
    await expect(input).toHaveValue(expected);
  });

  test('shows toast when cancel fails', async ({ page, request }) => {
    await resetNotifications(request, { failCancel: true });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expectToast(page, 'Unable to cancel notification.');
  });

  test('creates SMTP identity and shows Gmail settings once', async ({ page }) => {
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');
    const panel = page.getByTestId('smtp-identities');
    await expect(panel.getByRole('heading', { name: 'SMTP relay' })).toBeVisible();
    await panel.getByLabel('Sender address').fill('alice@example.com');
    await panel.getByLabel('Forward copies to').fill('owner@example.com\nmaria@example.com');
    await panel.getByRole('button', { name: 'Create' }).click();
    await expect(panel.getByTestId('smtp-identity-row')).toHaveCount(1);
    await expect(panel.getByText('alice@example.com')).toBeVisible();
    await expect(panel.getByText('owner@example.com, maria@example.com')).toBeVisible();
    const credentials = panel.getByTestId('smtp-credentials');
    const credentialInputs = credentials.locator('input');
    await expect(credentialInputs).toHaveCount(5);
    await expect(credentials.locator('input:not([readonly])')).toHaveCount(0);
    await expect(credentialInputs.nth(0)).toHaveValue('smtp.pinguin.test');
    await expect(credentialInputs.nth(1)).toHaveValue('465');
    await expect(credentialInputs.nth(2)).toHaveValue('ssl');
    await expect(credentialInputs.nth(3)).toHaveValue('smtp_JAYbQkNwQvT-LZI1');
    await expect(credentialInputs.nth(4)).toHaveValue('pgsmtp_UVSZ9mxDW6ZeV-tNwApoddcyCjOM5uA');
    await expectInputValueFits(credentialInputs.nth(3));
    await expectInputValueFits(credentialInputs.nth(4));
    const copyButtons = credentials.locator('.copy-field__button');
    await expect(copyButtons).toHaveCount(3);
    await expect(copyButtons).toHaveText(['', '', '']);
    await expect(credentials.getByRole('button', { name: /^Copy$/ })).toHaveCount(0);
    const credentialNotice = credentials.getByTestId('smtp-credential-notice');
    await expect(credentialNotice).toHaveText('SMTP identity created');
    await expect(page.locator('.toast', { hasText: 'SMTP identity created' })).toHaveCount(0);
    await credentials.getByRole('button', { name: 'Copy SMTP server' }).click();
    await expectClipboardText(page, 'smtp.pinguin.test');
    await expect(credentialNotice).toHaveText('SMTP server copied');
    await expect(page.locator('.toast', { hasText: 'SMTP server copied' })).toHaveCount(0);
    await credentials.getByRole('button', { name: 'Copy username' }).click();
    await expectClipboardText(page, 'smtp_JAYbQkNwQvT-LZI1');
    await expect(credentialNotice).toHaveText('Username copied');
    await expect(page.locator('.toast', { hasText: 'Username copied' })).toHaveCount(0);
    await credentials.getByRole('button', { name: 'Copy password' }).click();
    await expectClipboardText(page, 'pgsmtp_UVSZ9mxDW6ZeV-tNwApoddcyCjOM5uA');
    await expect(credentialNotice).toHaveText('Password copied');
    await expect(page.locator('.toast', { hasText: 'Password copied' })).toHaveCount(0);
    await panel.getByRole('button', { name: 'Close Gmail SMTP settings' }).click();
    await expect(credentials).toBeHidden();
  });

  test('rotates SMTP identity credentials', async ({ page, request }) => {
    const now = new Date().toISOString();
    await resetNotifications(request, {
      smtpIdentities: [
        {
          id: 'smtp-id-1',
          email_address: 'alice@example.com',
          username: 'smtp_test_1',
          status: 'active',
          last_used_at: null,
          created_at: now,
          updated_at: now,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');
    const panel = page.getByTestId('smtp-identities');
    page.once('dialog', (dialog) => dialog.accept());
    await panel.getByRole('button', { name: 'Rotate' }).click();
    const credentials = panel.getByTestId('smtp-credentials');
    const credentialInputs = credentials.locator('input');
    await expect(credentialInputs.nth(3)).toHaveValue('smtp_rotated_JAYbQkNwQvT-LZI1');
    await expect(credentialInputs.nth(4)).toHaveValue('pgsmtp_rotated_UVSZ9mxDW6ZeV-tNwApoddcyCjOM5uA');
    await expectInputValueFits(credentialInputs.nth(3));
    await expectInputValueFits(credentialInputs.nth(4));
    await expect(credentials.getByTestId('smtp-credential-notice')).toHaveText('SMTP credentials rotated');
    await expect(page.locator('.toast', { hasText: 'SMTP credentials rotated' })).toHaveCount(0);
  });

  test('updates SMTP identity forwarding owners', async ({ page, request }) => {
    const now = new Date().toISOString();
    await resetNotifications(request, {
      smtpIdentities: [
        {
          id: 'smtp-id-1',
          email_address: 'support@example.com',
          username: 'smtp_test_1',
          forward_to: ['owner@example.com'],
          status: 'active',
          last_used_at: null,
          created_at: now,
          updated_at: now,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');
    const panel = page.getByTestId('smtp-identities');
    await expect(panel.getByText('owner@example.com')).toBeVisible();
    await panel.getByRole('button', { name: 'Edit forwarding owners' }).click();
    await expect(panel.getByLabel('Sender address')).toHaveValue('support@example.com');
    await panel.getByLabel('Forward copies to').fill('owner@example.com\nmaria@example.com');
    await panel.getByRole('button', { name: 'Save changes' }).click();
    await expect(panel.getByText('owner@example.com, maria@example.com')).toBeVisible();
    await expectToast(page, 'Forwarding owners updated');
  });

  test('deletes SMTP identity', async ({ page, request }) => {
    const now = new Date().toISOString();
    await resetNotifications(request, {
      smtpIdentities: [
        {
          id: 'smtp-id-1',
          email_address: 'alice@example.com',
          username: 'smtp_test_1',
          status: 'active',
          last_used_at: null,
          created_at: now,
          updated_at: now,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');
    const panel = page.getByTestId('smtp-identities');
    await expect(panel.getByTestId('smtp-identity-row')).toHaveCount(1);
    page.once('dialog', (dialog) => dialog.accept());
    await panel.getByRole('button', { name: 'Delete' }).click();
    await expect(panel.getByTestId('smtp-identity-row')).toHaveCount(0);
    await expectToast(page, 'SMTP identity deleted');
  });

  test('shows SMTP identity load errors', async ({ page, request }) => {
    await resetNotifications(request, { failSMTPList: true });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');
    const panel = page.getByTestId('smtp-identities');
    await expect(panel.locator('.notice[data-variant="error"]')).toHaveText('Unable to load SMTP identities.');
    await expectToast(page, 'Unable to load SMTP identities.');
  });
});
