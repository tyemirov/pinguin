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
        return header && header.getAttribute('tauth-url') === expected;
      }, normalizedRuntimeBase);
    }
    const headerBase = (await page.locator('mpr-header').first().getAttribute('tauth-url')) || '';
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
          displayName: 'Bravo Labs',
        },
        tauth: {
          baseUrl: 'https://auth.bravo.test',
          googleClientId: 'bravo-google-client',
          tenantId: 'tauth-bravo',
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
      ).toHaveAttribute('google-site-id', 'bravo-google-client');
      const id = await page.evaluate(() => (window as any).__PINGUIN_CONFIG__?.tenant?.id || '');
      expect(id).toBe('tenant-bravo');
    });
  });

  const themePersistenceCases = [
    { label: 'light-to-dark', seed: 'light', expected: 'dark' },
    { label: 'dark-to-light', seed: 'dark', expected: 'light' },
  ];

  for (const scenario of themePersistenceCases) {
    test(`persists theme from landing to dashboard (${scenario.label})`, async ({ page }) => {
      await page.addInitScript((theme) => {
        if (!theme) {
          return;
        }
        const key = 'pinguin.theme';
        if (!window.localStorage.getItem(key)) {
          window.localStorage.setItem(key, theme);
        }
      }, scenario.seed);
      await page.goto('/index.html');
      const themeToggle = page.locator(
        '[data-mpr-footer="theme-toggle"] [data-mpr-theme-toggle="control"]',
      );
      await expect(themeToggle).toBeVisible();
      await page.waitForFunction((expected) => {
        const activeTheme =
          document.body.getAttribute('data-theme') ||
          document.documentElement.getAttribute('data-theme') ||
          '';
        return activeTheme === expected;
      }, scenario.seed);

      await themeToggle.click();
      await page.waitForFunction((expected) => {
        const activeTheme =
          document.body.getAttribute('data-theme') ||
          document.documentElement.getAttribute('data-theme') ||
          '';
        return activeTheme === expected;
      }, scenario.expected);

      const storedTheme = await page.evaluate(
        () => window.localStorage.getItem('pinguin.theme') || '',
      );
      expect(storedTheme).toBe(scenario.expected);

      await page.evaluate(() => {
        if (window.__mockAuth) {
          window.__mockAuth.authenticated = true;
          window.__persistMockAuth && window.__persistMockAuth();
        }
      });
      await page.goto('/dashboard.html');
      await expect(page.getByTestId('notifications-table')).toBeVisible();
      await page.waitForFunction((expected) => {
        const activeTheme =
          document.body.getAttribute('data-theme') ||
          document.documentElement.getAttribute('data-theme') ||
          '';
        return activeTheme === expected;
      }, scenario.expected);
    });
  }
});
