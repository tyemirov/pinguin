import { expect, test, type APIResponse } from '@playwright/test';
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

  await customerCard.getByRole('button', { name: 'Manage tenant' }).click();
  await expect(editDialog.getByLabel('Configure SMS delivery')).toBeChecked();
  await editDialog.getByLabel('Configure SMS delivery').uncheck();
  await editDialog.getByRole('button', { name: 'Save changes' }).click();
  await expectToast(page, 'Tenant configuration updated');
  await customerCard.getByRole('button', { name: 'Manage tenant' }).click();
  await expect(editDialog.getByLabel('Configure SMS delivery')).not.toBeChecked();
  await editDialog.getByRole('button', { name: 'Close' }).click();

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

  test('retries ambiguous credential writes with the exact secret and request identity', async ({ page }) => {
    let createAttempt = 0;
    let firstCreateBody = '';
    let firstIdempotencyKey = '';
    let committedCreateResponse: APIResponse | undefined;
    await page.route('**/api/tenants', async (route) => {
      if (route.request().method() !== 'POST') {
        await route.continue();
        return;
      }
      createAttempt += 1;
      const requestBody = route.request().postData() || '';
      const idempotencyKey = await route.request().headerValue('idempotency-key') || '';
      if (createAttempt === 1) {
        firstCreateBody = requestBody;
        firstIdempotencyKey = idempotencyKey;
        committedCreateResponse = await route.fetch();
        await route.abort('failed');
        return;
      }
      expect(requestBody).toBe(firstCreateBody);
      expect(idempotencyKey).toBe(firstIdempotencyKey);
      await route.fulfill({ response: committedCreateResponse });
    });

    await page.goto('/tenants.html');
    await page.getByRole('button', { name: 'Create tenant' }).click();
    const createDialog = page.getByRole('dialog', { name: 'Create tenant' });
    await createDialog.getByLabel('Display name').fill('Ambiguous Customer');
    await createDialog.getByLabel('Support email').fill('support@ambiguous.example');
    await createDialog.getByLabel('SMTP host').fill('smtp.ambiguous.example');
    await createDialog.getByLabel('SMTP username').fill('ambiguous-user');
    await createDialog.getByLabel('SMTP password').fill('ambiguous-password');
    await createDialog.getByLabel('From address').fill('notify@ambiguous.example');
    await createDialog.getByRole('button', { name: 'Create tenant' }).click();
    await expectToast(page, 'Unable to create tenant. Check every required delivery field.');
    await createDialog.getByRole('button', { name: 'Create tenant' }).click();

    const createdCredentialID = JSON.parse(firstCreateBody).api_credential.id;
    const keyDialog = page.getByRole('dialog', { name: 'Copy the new API key' });
    await expect(keyDialog.getByTestId('one-time-api-key')).toContainText(createdCredentialID);
    expect(createAttempt).toBe(2);
    await keyDialog.getByRole('button', { name: 'Close' }).click();
    const customerCard = page.getByTestId('tenant-card').filter({ hasText: 'Ambiguous Customer' });
    await expect(customerCard).toHaveCount(1);

    let rotationAttempt = 0;
    let firstRotationBody = '';
    let firstRotationVersion = '';
    let committedRotationResponse: APIResponse | undefined;
    await page.route('**/api/tenants/*/api-credential', async (route) => {
      rotationAttempt += 1;
      const requestBody = route.request().postData() || '';
      const version = await route.request().headerValue('if-match') || '';
      if (rotationAttempt === 1) {
        firstRotationBody = requestBody;
        firstRotationVersion = version;
        committedRotationResponse = await route.fetch();
        await route.abort('failed');
        return;
      }
      expect(requestBody).toBe(firstRotationBody);
      expect(version).toBe(firstRotationVersion);
      await route.fulfill({ response: committedRotationResponse });
    });
    await customerCard.getByRole('button', { name: 'Rotate API key' }).click();
    await expectToast(page, 'Unable to rotate API key.');
    await customerCard.getByRole('button', { name: 'Rotate API key' }).click();

    const rotatedCredentialID = JSON.parse(firstRotationBody).id;
    const rotatedKeyDialog = page.getByRole('dialog', { name: 'Copy the rotated API key' });
    await expect(rotatedKeyDialog.getByTestId('one-time-api-key')).toContainText(rotatedCredentialID);
    expect(rotationAttempt).toBe(2);
  });

  test('clears tenant workspace data and secrets when authentication ends', async ({ page }) => {
    await page.goto('/tenants.html');
    await page.getByRole('button', { name: 'Create tenant' }).click();
    const createDialog = page.getByRole('dialog', { name: 'Create tenant' });
    await createDialog.getByLabel('Display name').fill('Logout Customer');
    await createDialog.getByLabel('Support email').fill('support@logout.example');
    await createDialog.getByLabel('SMTP host').fill('smtp.logout.example');
    await createDialog.getByLabel('SMTP username').fill('logout-user');
    await createDialog.getByLabel('SMTP password').fill('logout-password');
    await createDialog.getByLabel('From address').fill('notify@logout.example');
    await createDialog.getByRole('button', { name: 'Create tenant' }).click();
    await expect(page.getByRole('dialog', { name: 'Copy the new API key' })).toBeVisible();

    await page.route('**/index.html', (route) => route.abort('blockedbyclient'));
    const workspace = await page.evaluate(async () => {
      const alpine = (window as typeof window & { Alpine: any }).Alpine;
      alpine.store('auth').clear();
      await alpine.nextTick();
      const root = document.querySelector('[data-testid="page-work-surface"]');
      const state = alpine.$data(root);
      return {
        createDisplayName: state.createForm.displayName,
        editDisplayName: state.editForm.displayName,
        oneTimeAPIKey: state.oneTimeAPIKey,
        pendingCreate: state.pendingCreate,
        pendingRotation: state.pendingRotation,
        selectedTenant: state.selectedTenant,
        pendingDeleteTenant: state.pendingDeleteTenant,
        tenantCount: state.tenants.length,
        openDialogCount: document.querySelectorAll('dialog[open]').length,
      };
    });
    expect(workspace).toEqual({
      createDisplayName: '',
      editDisplayName: '',
      oneTimeAPIKey: '',
      pendingCreate: null,
      pendingRotation: null,
      selectedTenant: null,
      pendingDeleteTenant: null,
      tenantCount: 0,
      openDialogCount: 0,
    });
  });
});
