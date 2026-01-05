import { test, expect } from '@playwright/test';
import { completeHeaderLogin, configureRuntime, stubExternalAssets } from './utils';

test.describe('Profile Menu Integration', () => {
  test.beforeEach(async ({ page, request }) => {
    await stubExternalAssets(page);
    // Seed a profile with an avatar URL to test both cases
    await configureRuntime(page, {
      authenticated: true,
      tenant: { id: 'tenant-test', displayName: 'Test Tenant' },
    });
  });

  test('replaces default header elements with custom avatar menu', async ({ page }) => {
    await page.goto('/dashboard.html');
    
    // 1. Assert custom avatar trigger is visible
    const avatarTrigger = page
      .getByTestId('profile-chip')
      .locator('[data-mpr-settings="toggle"]');
    await expect(avatarTrigger).toBeVisible();

    // 2. Assert it has correct circular styling and background
    // We check for width/height and that it has a background-image (either gradient or URL)
    const box = await avatarTrigger.boundingBox();
    expect(box?.width).toBe(40);
    expect(box?.height).toBe(40);
    
    const borderRadius = await avatarTrigger.evaluate(el => window.getComputedStyle(el).borderRadius);
    expect(borderRadius).toBe('50%');

    const backgroundImage = await avatarTrigger.evaluate(el => window.getComputedStyle(el).backgroundImage);
    expect(backgroundImage).not.toBe('none');
    expect(backgroundImage).not.toBe('');

    // 3. Assert default header profile chip is HIDDEN
    // The chip usually contains the name and a "Sign out" button
    const defaultProfileChip = page.locator('mpr-header [data-mpr-header="profile"]');
    await expect(defaultProfileChip).toBeHidden();

    // Verify that NO element containing the test user name is visible inside the header
    // (except maybe in my dropdown, but I haven't added it there yet)
    const profileName = page.locator('mpr-header [data-mpr-header="profile-name"]');
    await expect(profileName).toBeHidden();
    
    // Check for any visible text "Playwright User" in header
    const header = page.locator('mpr-header');
    await expect(header.getByText('Playwright User')).toBeHidden();

    // 4. Assert default settings button is HIDDEN
    const settingsButton = page.locator('mpr-header [data-mpr-header="settings-button"]');
    await expect(settingsButton).toBeHidden();

    // 5. Test dropdown functionality
    const dropdownPanel = page
      .getByTestId('profile-chip')
      .locator('[data-mpr-settings="panel"]');
    await expect(dropdownPanel).toBeHidden();

    await avatarTrigger.click();
    await expect(dropdownPanel).toBeVisible();

    const logoutButton = dropdownPanel.locator('button', { hasText: 'Sign out' });
    await expect(logoutButton).toBeVisible();

    // 6. Test logout action
    await logoutButton.click();
    // After logout, the avatar menu should be hidden
    await expect(avatarTrigger).toBeHidden();
  });

  test('profile menu works on landing page', async ({ page }) => {
    await page.goto('/index.html');
    await completeHeaderLogin(page); // This redirects to dashboard, so we need to go back
    await page.goto('/index.html');
    
    const avatarTrigger = page
      .getByTestId('profile-chip')
      .locator('[data-mpr-settings="toggle"]');
    await expect(avatarTrigger).toBeVisible();

    await avatarTrigger.click();
    const dropdownPanel = page
      .getByTestId('profile-chip')
      .locator('[data-mpr-settings="panel"]');
    await expect(dropdownPanel).toBeVisible();

    const logoutButton = dropdownPanel.locator('button', { hasText: 'Sign out' });
    await logoutButton.click();
    
    // On landing page, logout might just reload or hide things
    await expect(avatarTrigger).toBeHidden();
  });
});
