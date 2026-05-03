import { test, expect } from '@playwright/test';
import {
  completeHeaderLogin,
  configureRuntime,
  expectSharedHeaderUserMenu,
  openSharedHeaderUserMenu,
  stubExternalAssets,
} from './utils';

test.describe('Profile Menu Integration', () => {
  test.beforeEach(async ({ page, request }) => {
    await stubExternalAssets(page);
    // Seed a profile with an avatar URL to test both cases
    await configureRuntime(page, {
      authenticated: true,
      tenant: { id: 'tenant-test', displayName: 'Test Tenant' },
    });
  });

  test('uses the shared mpr-ui header user menu', async ({ page }) => {
    await page.goto('/dashboard.html');

    await expectSharedHeaderUserMenu(page);
    const settingsButton = page.locator('mpr-header [data-mpr-header="settings-button"]');
    await expect(settingsButton).toBeHidden();

    const userMenu = await openSharedHeaderUserMenu(page);
    const logoutButton = userMenu.locator('[data-mpr-user="logout"]');
    await expect(logoutButton).toBeVisible();

    await logoutButton.click();
    await expect(userMenu).toHaveAttribute('data-mpr-user-status', 'unauthenticated');
    await expect(userMenu).toBeHidden();
  });

  test('profile menu works after landing login', async ({ page }) => {
    await configureRuntime(page, {
      authenticated: false,
      tenant: { id: 'tenant-test', displayName: 'Test Tenant' },
    });
    await page.goto('/index.html');
    await completeHeaderLogin(page);

    const userMenu = await openSharedHeaderUserMenu(page);
    const logoutButton = userMenu.locator('[data-mpr-user="logout"]');
    await expect(logoutButton).toBeVisible();
    await logoutButton.click();

    await expect(userMenu).toHaveAttribute('data-mpr-user-status', 'unauthenticated');
    await expect(userMenu).toBeHidden();
  });
});
