import { expect, test } from '@playwright/test';
import { configureRuntime, resetNotifications, stubExternalAssets } from './utils';

const DESTRUCTIVE_REQUEST_DELAY_MS = 1000;

test.describe('Compact MPR UX review regressions', () => {
  test.beforeEach(async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
  });

  test('keeps an active forwarding draft while the creation dialog opens and closes', async ({
    page,
    request,
  }) => {
    const now = new Date().toISOString();
    await resetNotifications(request, {
      smtpDomains: [{ domain: 'example.com', status: 'verified' }],
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

    const identityRow = page.getByTestId('smtp-identity-row');
    await identityRow.getByRole('button', { name: 'Edit forwarding owners' }).click();
    const forwardingDraft = identityRow.getByLabel('Forward copies to');
    await forwardingDraft.fill('draft-owner@example.com');

    await page.getByRole('button', { name: 'Create identity' }).click();
    const creationDialog = page.getByRole('dialog', { name: 'Create SMTP identity' });
    await expect(creationDialog).toBeVisible();
    await creationDialog.getByRole('button', { name: 'Close' }).click();

    await expect(forwardingDraft).toBeVisible();
    await expect(forwardingDraft).toHaveValue('draft-owner@example.com');
  });

  test('submits notification cancellation once and focuses the stable event list', async ({
    page,
  }) => {
    let cancellationRequests = 0;
    await page.route(/\/api\/notifications\/notif-1\/cancel\?/, async (route) => {
      cancellationRequests += 1;
      await new Promise((resolve) => setTimeout(resolve, DESTRUCTIVE_REQUEST_DELAY_MS));
      await route.continue();
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');

    await page
      .getByTestId('notification-row')
      .getByRole('button', { name: 'Cancel', exact: true })
      .click();
    const dialog = page.getByRole('dialog', { name: 'Cancel notification' });
    const confirmation = dialog.getByRole('button', { name: 'Cancel notification' });
    await confirmation.dblclick();

    await expect(confirmation).toBeDisabled();
    await expect.poll(() => cancellationRequests).toBe(1);
    await expect(dialog).toBeHidden();
    await expect(page.getByTestId('notifications-list')).toBeFocused();
  });

  test('submits identity deletion once and focuses the stable identity list', async ({
    page,
    request,
  }) => {
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
    let deletionRequests = 0;
    await page.route(/\/api\/smtp-identities\/smtp-id-1$/, async (route) => {
      deletionRequests += 1;
      await new Promise((resolve) => setTimeout(resolve, DESTRUCTIVE_REQUEST_DELAY_MS));
      await route.continue();
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');

    await page.getByTestId('smtp-identity-row').getByRole('button', { name: 'Delete' }).click();
    const dialog = page.getByRole('dialog', { name: 'Delete SMTP identity' });
    const confirmation = dialog.getByRole('button', { name: 'Delete identity' });
    await confirmation.dblclick();

    await expect(confirmation).toBeDisabled();
    await expect.poll(() => deletionRequests).toBe(1);
    await expect(dialog).toBeHidden();
    await expect(page.getByRole('list', { name: 'SMTP identities' })).toBeFocused();
  });

  test('labels every SMTP identity metadata value', async ({ page, request }) => {
    const lastUsedAt = '2030-01-02T03:04:00.000Z';
    await resetNotifications(request, {
      smtpIdentities: [
        {
          id: 'smtp-id-1',
          email_address: 'alice@example.com',
          username: 'smtp_test_1',
          forward_to: ['owner@example.com'],
          status: 'active',
          last_used_at: lastUsedAt,
          created_at: lastUsedAt,
          updated_at: lastUsedAt,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');

    const identityRow = page.getByTestId('smtp-identity-row');
    const metadataTerms = identityRow.locator('dt');
    const metadataValues = identityRow.locator('dd');
    await expect(metadataTerms).toHaveText(['Username', 'Forwarding owners', 'Last used']);
    await expect(metadataValues.nth(0)).toHaveText('smtp_test_1');
    await expect(metadataValues.nth(1)).toHaveText('owner@example.com');
    await expect(metadataValues.nth(2)).not.toBeEmpty();
  });
});
