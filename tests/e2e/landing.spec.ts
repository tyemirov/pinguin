import { expect, test } from '@playwright/test';
import {
  completeHeaderLogin,
  configureRuntime,
  expectHeaderGoogleButton,
  expectHeaderGoogleButtonTopRight,
  resetNotifications,
  stubExternalAssets,
} from './utils';

test.describe('Landing page auth flow', () => {
  test.beforeEach(async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
    await configureRuntime(page, { authenticated: false });
  });

  test('shows CTA and disables button during GIS prep', async ({ page }) => {
    await page.goto('/index.html');
    await expect(page.getByTestId('landing-cta')).toBeVisible();
    await expectHeaderGoogleButton(page);
  });

  test('renders Google login button in top-right header slot', async ({ page }) => {
    await page.goto('/index.html');
    await expectHeaderGoogleButtonTopRight(page);
  });

  test('completes Google/TAuth handshake and redirects to dashboard', async ({ page }) => {
    await page.goto('/index.html');
    await completeHeaderLogin(page);
    await expect(page.getByTestId('notifications-table')).toBeVisible();
  });

  test('mpr-header attributes mirror runtime TAuth base URL', async ({ page }) => {
    await page.goto('/index.html');
    const runtimeBase = await page.evaluate(() => (window as any).__PINGUIN_CONFIG__?.tauthBaseUrl || '');
    const normalizedRuntimeBase = runtimeBase.replace(/\/$/, '');
    if (normalizedRuntimeBase) {
      await page.waitForFunction((expected) => {
        const header = document.querySelector('mpr-header');
        return header && header.getAttribute('base-url') === expected;
      }, normalizedRuntimeBase);
    }
    const headerBase = (await page.locator('mpr-header').first().getAttribute('base-url')) || '';
    if (normalizedRuntimeBase) {
      expect(headerBase).toBe(normalizedRuntimeBase);
    } else {
      expect(headerBase).not.toBe('');
    }
  });

  test('updates header brand label with tenant display name', async ({ page }) => {
    await page.goto('/index.html');
    await page.waitForFunction((expected) => {
      const header = document.querySelector('mpr-header');
      return header && header.getAttribute('brand-label') === expected;
    }, 'Playwright Tenant');
  });

  test.describe('Tenant branding variants', () => {
    test.beforeEach(async ({ page, request }) => {
      await resetNotifications(request);
      await stubExternalAssets(page);
      await configureRuntime(page, {
        authenticated: false,
        tenant: {
          id: 'tenant-bravo',
          slug: 'bravo',
          displayName: 'Bravo Labs',
          identity: {
            googleClientId: 'bravo-google-client',
            tauthBaseUrl: 'https://auth.bravo.test',
          },
        },
      });
    });

    test('applies runtime tenant display name and Google client ID', async ({ page }) => {
      await page.goto('/index.html');
      await page.waitForFunction((expected) => {
        const header = document.querySelector('mpr-header');
        return header && header.getAttribute('brand-label') === expected;
      }, 'Bravo Labs');
      await expect(
        page.locator('mpr-header').first(),
      ).toHaveAttribute('site-id', 'bravo-google-client');
      const slug = await page.evaluate(
        () => (window as any).__PINGUIN_CONFIG__?.tenant?.slug || '',
      );
      expect(slug).toBe('bravo');
    });
  });
});
