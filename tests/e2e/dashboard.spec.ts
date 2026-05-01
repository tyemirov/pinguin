import { expect, test } from '@playwright/test';
import {
  configureRuntime,
  resetNotifications,
  stubExternalAssets,
  expectToast,
  expectHeaderGoogleButtonTopRight,
  loginAndVisitDashboard,
} from './utils';

const LANDING_URL_PATTERN = /\/(?:index\.html)?$/;

test.describe('Dashboard', () => {
  test.beforeEach(async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
  });

  test('redirects guests to the landing page', async ({ page }) => {
    await configureRuntime(page, { authenticated: false });
    await page.goto('/dashboard.html');
    await expect(page).toHaveURL(LANDING_URL_PATTERN);
    await expect(page.getByTestId('landing-cta')).toBeVisible();
  });

  test('shows a Google-powered login button in the header for guests', async ({ page }) => {
    await configureRuntime(page, { authenticated: false });
    await page.goto('/dashboard.html');
    await expect(page).toHaveURL(LANDING_URL_PATTERN);
    await expectHeaderGoogleButtonTopRight(page);
  });

  test('redirects after BroadcastChannel logout', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/dashboard.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
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
    await expect(page.getByTestId('landing-cta')).toBeVisible();
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
    await page.goto('/dashboard.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(2);
    const filterSelect = page.locator('label:has-text("Filter by status") select');
    await filterSelect.selectOption('queued');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.locator('.status-badge')).toHaveAttribute('data-variant', 'queued');
    await filterSelect.selectOption('cancelled');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    await expect(page.locator('.status-badge')).toHaveAttribute('data-variant', 'cancelled');
  });

  test('renders notification table and allows cancel', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/dashboard.html');
    await expect(page.getByTestId('notification-row')).toHaveCount(1);
    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expectToast(page, 'Notification cancelled');
  });

  test('shows error toast when list request fails', async ({ page, request }) => {
    await resetNotifications(request, { failList: true });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/dashboard.html');
    await expect(page.locator('.notice[data-variant="error"]')).toHaveText('Unable to load notifications.');
    await expectToast(page, 'Unable to load notifications.');
  });

  test('reschedule flow updates toast', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/dashboard.html');
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
    await page.goto('/dashboard.html');
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
    await page.goto('/dashboard.html');
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
    await page.goto('/dashboard.html');
    page.once('dialog', (dialog) => dialog.accept());
    await page.getByRole('button', { name: 'Cancel' }).click();
    await expectToast(page, 'Unable to cancel notification.');
  });

  test('creates SMTP identity and shows Gmail settings once', async ({ page }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/dashboard.html');
    const panel = page.getByTestId('smtp-identities');
    await expect(panel.getByRole('heading', { name: 'SMTP identities' })).toBeVisible();
    await panel.getByLabel('Sender address').fill('alice@example.com');
    await panel.getByRole('button', { name: 'Create' }).click();
    await expect(panel.getByTestId('smtp-identity-row')).toHaveCount(1);
    await expect(panel.getByText('alice@example.com')).toBeVisible();
    const credentials = panel.getByTestId('smtp-credentials');
    await expect(credentials.locator('input').nth(0)).toHaveValue('smtp.pinguin.test');
    await expect(credentials.locator('input').nth(1)).toHaveValue('587');
    await expect(credentials.locator('input').nth(2)).toHaveValue('starttls');
    await expect(credentials.locator('input').nth(3)).toHaveValue('smtp_test_1');
    await expect(credentials.locator('input').nth(4)).toHaveValue('pgsmtp_test_password');
    await expectToast(page, 'SMTP identity created');
  });

  test('rotates SMTP identity credentials', async ({ page, request }) => {
    const now = new Date().toISOString();
    await resetNotifications(request, {
      smtpIdentities: [
        {
          id: 'smtp-id-1',
          tenant_id: 'tenant-devserver',
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
    await page.goto('/dashboard.html');
    const panel = page.getByTestId('smtp-identities');
    page.once('dialog', (dialog) => dialog.accept());
    await panel.getByRole('button', { name: 'Rotate' }).click();
    await expect(panel.getByTestId('smtp-credentials').locator('input').nth(4)).toHaveValue('pgsmtp_rotated_password');
    await expectToast(page, 'SMTP credentials rotated');
  });

  test('deletes SMTP identity', async ({ page, request }) => {
    const now = new Date().toISOString();
    await resetNotifications(request, {
      smtpIdentities: [
        {
          id: 'smtp-id-1',
          tenant_id: 'tenant-devserver',
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
    await page.goto('/dashboard.html');
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
    await page.goto('/dashboard.html');
    const panel = page.getByTestId('smtp-identities');
    await expect(panel.locator('.notice[data-variant="error"]')).toHaveText('Unable to load SMTP identities.');
    await expectToast(page, 'Unable to load SMTP identities.');
  });
});
