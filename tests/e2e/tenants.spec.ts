import { expect, test } from '@playwright/test';
import { configureRuntime, expectToast, resetNotifications, stubExternalAssets } from './utils';

test.describe('Managed tenant configuration', () => {
  test.beforeEach(async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
    await configureRuntime(page, { authenticated: true });
  });

  test('creates, updates, rotates, and permanently deletes a tenant', async ({ page }) => {
    await page.goto('/tenants.html');
    await expect(page.getByRole('heading', { name: 'Tenants' })).toBeVisible();
    await expect(page.getByTestId('tenant-card')).toHaveCount(2);

    await page.getByRole('button', { name: 'Create tenant' }).click();
    const createDialog = page.getByRole('dialog', { name: 'Create tenant' });
    await createDialog.getByLabel('Display name').fill('Customer Operations');
    await createDialog.getByLabel('Support email').fill('support@customer.example');
    await createDialog.getByLabel('SMTP host').fill('smtp.customer.example');
    await createDialog.getByLabel('SMTP port').fill('587');
    await createDialog.getByLabel('SMTP username').fill('customer-user');
    await createDialog.getByLabel('SMTP password').fill('customer-password');
    await createDialog.getByLabel('From address').fill('notify@customer.example');
    await createDialog.getByLabel('Configure SMS delivery').check();
    await createDialog.getByLabel('Twilio account SID').fill('AC123');
    await createDialog.getByLabel('Twilio auth token').fill('twilio-token');
    await createDialog.getByLabel('SMS from number').fill('+15551234567');
    await createDialog.getByRole('button', { name: 'Create tenant' }).click();

    const keyDialog = page.getByRole('dialog', { name: 'Copy the new API key' });
    await expect(keyDialog).toBeVisible();
    const oneTimeKey = keyDialog.getByTestId('one-time-api-key');
    await expect(oneTimeKey).toHaveText(/^pgn_1_[0-9a-f-]{36}_[A-Za-z0-9_-]{43}$/);
    await expectToast(page, 'Tenant created');
    await keyDialog.getByRole('button', { name: 'Close' }).click();
    await expect(keyDialog).toBeHidden();
    await expect(oneTimeKey).toHaveCount(0);

    let customerCard = page.getByTestId('tenant-card').filter({ hasText: 'Customer Operations' });
    await expect(customerCard).toHaveCount(1);
    await expect(customerCard).toContainText('notify@customer.example');

    await customerCard.getByRole('button', { name: 'Manage tenant' }).click();
    const editDialog = page.getByRole('dialog', { name: 'Manage tenant' });
    await editDialog.getByLabel('Display name').fill('Customer Delivery');
    await editDialog.getByLabel('SMTP host').fill('smtp.updated.example');
    await editDialog.getByRole('button', { name: 'Save changes' }).click();
    await expectToast(page, 'Tenant configuration updated');
    customerCard = page.getByTestId('tenant-card').filter({ hasText: 'Customer Delivery' });
    await expect(customerCard).toHaveCount(1);

    await customerCard.getByRole('button', { name: 'Rotate API key' }).click();
    const rotatedKeyDialog = page.getByRole('dialog', { name: 'Copy the rotated API key' });
    await expect(rotatedKeyDialog.getByTestId('one-time-api-key')).toHaveText(
      /^pgn_1_[0-9a-f-]{36}_[A-Za-z0-9_-]{43}$/,
    );
    await rotatedKeyDialog.getByRole('button', { name: 'Close' }).click();
    await expect(rotatedKeyDialog.getByTestId('one-time-api-key')).toHaveCount(0);

    await customerCard.getByRole('button', { name: 'Delete' }).click();
    const deleteDialog = page.getByRole('dialog', { name: 'Permanently delete tenant' });
    await expect(deleteDialog).toContainText('every notification, delivery profile, API credential, and SMTP resource');
    await deleteDialog.getByRole('button', { name: 'Delete' }).click();
    await expectToast(page, 'Tenant permanently deleted');
    await expect(page.getByTestId('tenant-card')).toHaveCount(2);
    await expect(page.getByTestId('tenant-card').filter({ hasText: 'Customer Delivery' })).toHaveCount(0);
  });
});
