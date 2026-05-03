import { expect, Page } from '@playwright/test';

type TenantConfig = {
  id: string;
  displayName: string;
};

type ConfigureRuntimeOptions = {
  authenticated: boolean;
  tenant?: TenantConfig;
  tauth?: {
    baseUrl?: string;
    googleClientId?: string;
    tenantId?: string;
  };
};

const PLAYWRIGHT_AVATAR_URL =
  'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40"%3E%3Crect width="40" height="40" rx="20" fill="%232563eb"/%3E%3Ctext x="20" y="25" text-anchor="middle" font-size="16" font-family="Arial" fill="white"%3EP%3C/text%3E%3C/svg%3E';

export async function configureRuntime(page: Page, options: ConfigureRuntimeOptions) {
  const baseUrl = process.env.PLAYWRIGHT_BASE_URL || 'http://127.0.0.1:4174';
  await page.context().clearCookies();
  if (options.authenticated) {
    await page.context().addCookies([
      {
        name: 'pinguin_playwright_auth',
        value: '1',
        url: baseUrl,
        sameSite: 'Lax',
      },
    ]);
  }
  const tenant: TenantConfig = options.tenant || {
    id: 'tenant-playwright',
    displayName: 'Playwright Tenant',
  };
  const tauthConfig = {
    baseUrl,
    googleClientId:
      '991677581607-r0dj8q6irjagipali0jpca7nfp8sfj9r.apps.googleusercontent.com',
    tenantId: 'tauth-playwright',
    ...(options.tauth || {}),
  };
  await page.addInitScript(
    ({ authenticated, avatarUrl }) => {
      if (!window.name) {
        const defaultProfile = {
          user_email: 'playwright@example.com',
          user_display_name: 'Playwright User',
          user_avatar_url: avatarUrl,
          display: 'Playwright User',
          given_name: 'Playwright',
          avatar_url: avatarUrl,
        };
        window.name = JSON.stringify({
          __mockAuth: {
            authenticated,
            profile: defaultProfile,
          },
        });
      }
    },
    { authenticated: options.authenticated, avatarUrl: PLAYWRIGHT_AVATAR_URL },
  );
  await page.addInitScript(
    ({ authenticated, tenantPayload, tauthPayload, avatarUrl }) => {
      window.__PINGUIN_CONFIG__ = {
        apiBaseUrl: '/api',
        tauthBaseUrl: tauthPayload.baseUrl,
        tauthTenantId: tauthPayload.tenantId,
        googleClientId: tauthPayload.googleClientId,
        landingUrl: '/index.html',
        dashboardUrl: '/dashboard.html',
        runtimeConfigUrl: '/runtime-config',
        skipRemoteConfig: true,
        tenant: tenantPayload,
      };
      window.__PINGUIN_RUNTIME_CONFIG_URL = '/runtime-config';
      const defaultProfile = {
        user_email: 'playwright@example.com',
        user_display_name: 'Playwright User',
        user_avatar_url: avatarUrl,
        display: 'Playwright User',
        given_name: 'Playwright',
        avatar_url: avatarUrl,
      };
      const storedState = (() => {
        try {
          return window.name ? JSON.parse(window.name) : null;
        } catch {
          return null;
        }
      })();
      const session = storedState?.__mockAuth || {
        authenticated,
        profile: defaultProfile,
      };
      session.profile = session.profile || defaultProfile;
      window.__mockAuth = session;
      window.__persistMockAuth = () => {
        const payload = storedState || {};
        payload.__mockAuth = window.__mockAuth;
        try {
          window.name = JSON.stringify(payload);
        } catch {
          // ignore
        }
      };
      window.__persistMockAuth();
    },
    {
      authenticated: options.authenticated,
      tenantPayload: tenant,
      tauthPayload: tauthConfig,
      avatarUrl: PLAYWRIGHT_AVATAR_URL,
    },
  );
}

export async function stubExternalAssets(page: Page) {
  await page.route('https://accounts.google.com/gsi/client', (route) => {
    const googleStub = `
      window.__playwrightGoogle = {
        callback: null,
        trigger(payload) {
          if (!this.callback) {
            return;
          }
          window.__mockAuth = window.__mockAuth || { authenticated: false };
          window.__mockAuth.authenticated = true;
          window.__mockAuth.profile =
            window.__mockAuth.profile || {
              user_email: 'playwright@example.com',
              user_display_name: 'Playwright User',
              user_avatar_url: '${PLAYWRIGHT_AVATAR_URL}',
              display: 'Playwright User',
              given_name: 'Playwright',
              avatar_url: '${PLAYWRIGHT_AVATAR_URL}',
            };
          window.__persistMockAuth && window.__persistMockAuth();
          this.callback(payload || { credential: 'playwright-token' });
        },
      };
      window.google = {
        accounts: {
          id: {
            initialize(config) {
              window.__playwrightGoogle.callback = config && config.callback;
            },
            renderButton(el, options) {
              var label = (options && options.text) || "Sign in";
              var normalizedLabel = label.replace(/_/g, " ");
              var host = el && typeof el.closest === "function"
                ? el.closest('[data-mpr-header="google-signin"]')
                : null;
              if (host && host.style) {
                host.style.display = "inline-flex";
                host.style.alignItems = "center";
                host.style.justifyContent = "flex-end";
                host.style.minWidth = "220px";
                host.style.minHeight = "44px";
                host.style.gap = "0.5rem";
              }
              if (el && el.style) {
                el.style.display = "flex";
                el.style.alignItems = "center";
                el.style.justifyContent = "center";
                el.style.minWidth = "200px";
                el.style.minHeight = "44px";
              }
              el.innerHTML =
                "<button class='button secondary' style='" +
                "display:flex;align-items:center;justify-content:center;" +
                "gap:0.5rem;padding:0.65rem 1.1rem;border-radius:999px;" +
                "border:1px solid var(--mpr-color-border, #cbd5f5);" +
                "background:var(--mpr-color-surface-elevated, #fff);" +
                "color:var(--mpr-color-text-primary, #0f172a);" +
                "font-weight:600;font-size:0.95rem;min-width:180px;" +
                "min-height:40px;box-shadow:0 2px 6px rgba(15,23,42,0.15);" +
                "'>" +
                normalizedLabel +
                "</button>";
            },
            prompt() {},
          },
        },
      };
    `;
    route.fulfill({
      contentType: 'text/javascript',
      body: googleStub,
    });
  });
  await page.route('**/tauth.js', (route) => route.abort('blockedbyclient'));
}

export async function resetNotifications(request: import('@playwright/test').APIRequestContext, overrides = {}) {
  await request.post('/testing/reset', {
    data: overrides,
  });
}

export async function expectToast(page: Page, text: string) {
  await expect(page.getByRole('button', { name: text }).first()).toBeVisible();
}

/**
 * @param {Page} page
 */
async function waitForHeaderLoginButton(page: Page) {
  await page.waitForFunction(() => {
    const header = document.querySelector('mpr-header');
    if (!header) {
      return false;
    }
    const wrapper = header.querySelector('[data-mpr-header="google-signin"]');
    if (!wrapper) {
      return false;
    }
    const button =
      wrapper.querySelector('[data-test="google-signin"]') ||
      wrapper.querySelector('[role="button"]') ||
      wrapper.querySelector('button');
    if (!button) {
      return false;
    }
    const rect = button.getBoundingClientRect();
    return rect.width > 0 && rect.height > 0;
  }, undefined, { timeout: 15000 });
}

/**
 * @typedef {{
 *   buttonRect: { x: number; y: number; width: number; height: number; right: number };
 *   headerRect: { x: number; y: number; width: number; height: number; right: number };
 *   label: string;
 * }} HeaderButtonMetrics
 */

/**
 * @param {Page} page
 * @returns {Promise<HeaderButtonMetrics | null>}
 */
async function getHeaderButtonMetrics(page: Page) {
  return page.evaluate(() => {
    const header = document.querySelector('mpr-header');
    if (!header) {
      return null;
    }
    const wrapper = header.querySelector('[data-mpr-header="google-signin"]');
    if (!wrapper) {
      return null;
    }
    const target =
      wrapper.querySelector('[data-test="google-signin"]') ||
      wrapper.querySelector('[role="button"]') ||
      wrapper.querySelector('button');
    if (!target) {
      return null;
    }
    const buttonRect = target.getBoundingClientRect();
    const headerRect = header.getBoundingClientRect();
    return {
      buttonRect: {
        x: buttonRect.x,
        y: buttonRect.y,
        width: buttonRect.width,
        height: buttonRect.height,
        right: buttonRect.right,
      },
      headerRect: {
        x: headerRect.x,
        y: headerRect.y,
        width: headerRect.width,
        height: headerRect.height,
        right: headerRect.right,
      },
      label: (target.textContent || '').trim(),
    };
  });
}

export async function expectHeaderGoogleButton(page: Page) {
  const header = page.locator('mpr-header').first();
  await expect(header).toBeVisible();
  await waitForHeaderLoginButton(page);
  const siteId =
    (await header.getAttribute('google-site-id')) ||
    (await header.getAttribute('site-id')) ||
    '';
  expect(siteId.trim(), 'login button missing google-site-id').not.toBe('');
  const tenantId = (await header.getAttribute('tauth-tenant-id')) || '';
  expect(tenantId.trim(), 'login button missing tauth-tenant-id').not.toBe('');
  const metrics = await getHeaderButtonMetrics(page);
  if (!metrics) {
    throw new Error('Unable to locate Google button inside mpr-header');
  }
  expect(metrics.buttonRect.width).toBeGreaterThan(0);
  expect(metrics.buttonRect.height).toBeGreaterThan(0);
  expect(metrics.label.toLowerCase()).toContain('sign');
}

export async function expectSharedHeaderUserMenu(page: Page) {
  const legacyProfileChip = page.getByTestId('profile-chip');
  await expect(legacyProfileChip).toHaveCount(0);

  const header = page.locator('mpr-header').first();
  await expect(header).toBeVisible();
  const userMenu = header.locator('[data-mpr-header="user-menu"]');
  await expect(userMenu).toHaveCount(1);
  await expect(userMenu).toBeVisible();
  await expect(userMenu).toHaveAttribute('data-mpr-user-status', 'authenticated');
  await expect(userMenu.locator('[data-mpr-user="trigger"]')).toBeVisible();
  await expect(userMenu.locator('[data-mpr-user="name"]')).toContainText('Playwright');
  await expect(userMenu).toHaveAttribute('data-user-display', 'Playwright User');
  await expect(userMenu.locator('[data-mpr-user="avatar"]')).toBeVisible();
}

export async function openSharedHeaderUserMenu(page: Page) {
  await expectSharedHeaderUserMenu(page);
  const userMenu = page
    .locator('mpr-header')
    .first()
    .locator('[data-mpr-header="user-menu"]');
  await userMenu.locator('[data-mpr-user="trigger"]').click();
  await expect(userMenu).toHaveAttribute('data-mpr-user-open', 'true');
  await expect(userMenu.locator('[data-mpr-user="menu"]')).toBeVisible();
  return userMenu;
}

export async function expectHeaderGoogleButtonTopRight(page: Page) {
  await waitForHeaderLoginButton(page);
  const metrics = await getHeaderButtonMetrics(page);
  if (!metrics) {
    throw new Error('Unable to measure header login button');
  }
  const { headerRect, buttonRect } = metrics;
  const headerRight = headerRect.x + headerRect.width;
  expect(buttonRect.right).toBeGreaterThan(headerRect.x + headerRect.width * 0.6);
  expect(buttonRect.right).toBeLessThanOrEqual(headerRight + 2);
  expect(buttonRect.y).toBeLessThanOrEqual(headerRect.y + headerRect.height * 0.6);
}

export async function clickHeaderGoogleButton(page: Page) {
  await waitForHeaderLoginButton(page);
  await page.evaluate(() => {
    const header = document.querySelector('mpr-header');
    if (!header) {
      return;
    }
    const container = header.querySelector('[data-mpr-header="google-signin"]');
    if (!container) {
      return;
    }
    const target =
      container.querySelector('[data-test="google-signin"]') ||
      container.querySelector('[role="button"]') ||
      container.querySelector('button');
    if (target && typeof target.click === 'function') {
      target.click();
    }
  });
}

export async function completeHeaderLogin(page: Page) {
  await expectHeaderGoogleButton(page);
  await clickHeaderGoogleButton(page);
  await triggerGoogleCredentialAndWaitForDashboard(page);
}

export async function triggerGoogleCredentialAndWaitForDashboard(page: Page) {
  
  // Wait for the Google Identity stub to be initialized with a callback
  await page.waitForFunction(() => {
    const stub = (window as any).__playwrightGoogle;
    return stub && typeof stub.callback === 'function';
  }, undefined, { timeout: 30000 }).catch(() => {
    throw new Error('Timed out waiting for Google Identity callback to be registered');
  });

  const waitForDashboard = page.url().includes('/dashboard.html')
    ? Promise.resolve()
    : page.waitForURL('**/dashboard.html', { timeout: 30000 });

  const triggered = await page.evaluate(() => {
    const googleStub = (window as any).__playwrightGoogle;
    if (googleStub && googleStub.trigger) {
      googleStub.trigger({ credential: 'playwright-token' });
      return true;
    }
    return false;
  });

  if (!triggered) {
    throw new Error('Google Identity stub unavailable or failed to trigger');
  }
  await waitForDashboard;
  await expect(page.getByTestId('notifications-table')).toBeVisible();
}

export async function loginAndVisitDashboard(page: Page) {
  await page.goto('/index.html');
  await completeHeaderLogin(page);
}
