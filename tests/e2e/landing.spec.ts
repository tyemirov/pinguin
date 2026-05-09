import { expect, test } from '@playwright/test';
import {
  completeHeaderLogin,
  configureRuntime,
  expectSharedHeaderUserMenu,
  expectHeaderGoogleButton,
  expectPinguinHeaderBrand,
  resetNotifications,
  stubExternalAssets,
} from './utils';

test.describe('Landing page auth flow', () => {
  test.beforeEach(async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
    await configureRuntime(page, { authenticated: false });
  });

  test('shows a focused sign-in page and login button', async ({ page }) => {
    await page.goto('/index.html');
    await expect(page.getByRole('heading', { name: /notification delivery/i })).toBeVisible();
    await expectPinguinHeaderBrand(page);
    await expectHeaderGoogleButton(page);
    await expect(page.getByLabel('Notification workspace preview')).toBeVisible();
  });

  test('completes Google/TAuth handshake and redirects to event log', async ({ page }) => {
    await page.goto('/index.html');
    await completeHeaderLogin(page);
    await expect(page.getByTestId('notifications-table')).toBeVisible();
  });

  test('starts login from the landing page header login button', async ({ page }) => {
    await page.goto('/index.html');
    await completeHeaderLogin(page);
  });

  test('shows the shared mpr-ui header user menu after login', async ({ page }) => {
    await page.goto('/index.html');
    await completeHeaderLogin(page);
    await expectSharedHeaderUserMenu(page);
    await expect(
      page.locator('mpr-header [data-mpr-header="sign-out-button"]'),
    ).toBeHidden();
  });

  test('mpr-header uses the config-ui auth contract', async ({ page }) => {
    await page.goto('/index.html');
    await expect(page.locator('mpr-header').first()).toHaveAttribute(
      'data-config-url',
      '/config-ui.yaml',
    );
    await expect(page.locator('script[data-mpr-ui-bundle-src]')).toHaveAttribute(
      'data-mpr-ui-bundle-src',
      /mpr-ui@latest\/mpr-ui\.js$/,
    );
    await expect(page.locator('script[src*="tauth.js"]')).toHaveCount(0);
    await expect(page.locator('mpr-header').first()).toHaveAttribute(
      'tauth-url',
      'http://127.0.0.1:4174',
    );
  });

  test('deduplicates unauthenticated shared-header profile checks', async ({ page }) => {
    let profileRequestCount = 0;
    let refreshRequestCount = 0;
    page.on('request', (request) => {
      const requestUrl = new URL(request.url());
      if (request.method() === 'GET' && requestUrl.pathname === '/me') {
        profileRequestCount += 1;
      }
      if (request.method() === 'POST' && requestUrl.pathname === '/auth/refresh') {
        refreshRequestCount += 1;
      }
    });

    await page.goto('/index.html');
    await expectHeaderGoogleButton(page);
    await page.waitForFunction(
      () => (window as any).__PINGUIN_AUTH_STATE__?.status === 'unauthenticated',
    );
    await page.waitForTimeout(250);

    expect(profileRequestCount).toBe(1);
    expect(refreshRequestCount).toBe(1);
  });

  test('keeps the Pinguin header brand when tenant metadata is present', async ({ page }) => {
    await page.goto('/index.html');
    await expectPinguinHeaderBrand(page);
    await page.waitForFunction(
      (expected) => document.documentElement.dataset.tenantId === expected,
      'tenant-playwright',
    );
  });

  test.describe('Tenant branding variants', () => {
    test.beforeEach(async ({ page, request }) => {
      await resetNotifications(request);
      await stubExternalAssets(page);
      await configureRuntime(page, {
        authenticated: false,
        tenant: {
          id: 'ps',
          displayName: 'PoodleScanner',
        },
        tauth: {
          baseUrl: 'https://tauth-api.mprlab.com',
          googleClientId: 'production-google-client',
          tenantId: 'pinguin',
        },
      });
    });

    test('preserves product chrome when runtime config returns another tenant display name', async ({ page }) => {
      await page.goto('/index.html');
      await expectPinguinHeaderBrand(page);
      await page.waitForFunction(
        (expected) => document.documentElement.dataset.tenantId === expected,
        'ps',
      );
      await expect(
        page.locator('mpr-header').first(),
      ).toHaveAttribute('google-site-id', 'playwright-client');
      const id = await page.evaluate(() => (window as any).__PINGUIN_CONFIG__?.tenant?.id || '');
      expect(id).toBe('ps');
    });
  });

  const themePersistenceCases = [
    { label: 'light-to-dark', seed: 'light', expected: 'dark' },
    { label: 'dark-to-light', seed: 'dark', expected: 'light' },
  ];

  for (const scenario of themePersistenceCases) {
    test(`persists theme from landing to event log (${scenario.label})`, async ({ page }) => {
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

      await completeHeaderLogin(page);
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
