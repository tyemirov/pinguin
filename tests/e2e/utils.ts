import { expect, Page } from '@playwright/test';

type TenantConfig = {
  id: string;
  displayName: string;
};

type ConfigureRuntimeOptions = {
  authenticated: boolean;
  tenant?: TenantConfig;
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
    ({ authenticated, tenantPayload, avatarUrl }) => {
      window.__PINGUIN_CONFIG__ = {
        apiBaseUrl: '/api',
        landingUrl: '/index.html',
        eventLogUrl: '/event-log.html',
        smtpRelayUrl: '/smtp-relay.html',
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
      avatarUrl: PLAYWRIGHT_AVATAR_URL,
    },
  );
}

export async function stubExternalAssets(page: Page) {
  await page.route('https://loopaware.mprlab.com/**', (route) => {
    route.fulfill({
      contentType: 'text/javascript',
      body: '',
    });
  });
  await page.route('https://accounts.google.com/gsi/client', (route) => {
    const googleStub = `
      window.__playwrightSharedAuth = {
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
              window.__playwrightSharedAuth.callback = config && config.callback;
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

export async function expectSharedHeaderSignInButton(page: Page) {
  const header = page.locator('mpr-header').first();
  await expect(header).toBeVisible();
  await waitForHeaderLoginButton(page);
  const tenantId = (await header.getAttribute('tauth-tenant-id')) || '';
  expect(tenantId.trim(), 'login button missing tauth-tenant-id').not.toBe('');
  const metrics = await getHeaderButtonMetrics(page);
  if (!metrics) {
    throw new Error('Unable to locate shared sign-in button inside mpr-header');
  }
  expect(metrics.buttonRect.width).toBeGreaterThan(0);
  expect(metrics.buttonRect.height).toBeGreaterThan(0);
  expect(metrics.label.toLowerCase()).toContain('sign');
}

export async function expectPinguinHeaderBrand(page: Page) {
  const header = page.locator('mpr-header').first();
  await expect(header).toBeVisible();
  await expect(header).toHaveAttribute('brand-label', 'Pinguin');
  const brand = header.locator('.pinguin-brand').first();
  await expect(brand).toBeVisible();
  await expect(brand).toHaveText('Pinguin');
  await expect(brand.locator('img.pinguin-brand__mark')).toHaveAttribute(
    'src',
    '/favicon.svg',
  );
  await expect(page.locator('head link[rel="icon"][href="/favicon.svg"]')).toHaveAttribute(
    'type',
    'image/svg+xml',
  );
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

export async function expectSharedHeaderSignInButtonTopRight(page: Page) {
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

export async function clickSharedHeaderSignInButton(page: Page) {
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
  await expectSharedHeaderSignInButton(page);
  await clickSharedHeaderSignInButton(page);
  await triggerSharedAuthCredentialAndWaitForEventLog(page);
}

export async function triggerSharedAuthCredentialAndWaitForEventLog(page: Page) {
  await page.waitForFunction(() => {
    const stub = (window as any).__playwrightSharedAuth;
    return stub && typeof stub.callback === 'function';
  }, undefined, { timeout: 30000 }).catch(() => {
    throw new Error('Timed out waiting for shared auth callback to be registered');
  });

  const waitForEventLog = page.url().includes('/event-log.html')
    ? Promise.resolve()
    : page.waitForURL('**/event-log.html', { timeout: 30000 });

  const triggered = await page.evaluate(() => {
    const authStub = (window as any).__playwrightSharedAuth;
    if (authStub && authStub.trigger) {
      authStub.trigger({ credential: 'playwright-token' });
      return true;
    }
    return false;
  });

  if (!triggered) {
    throw new Error('Shared auth stub unavailable or failed to trigger');
  }
  await waitForEventLog;
  await expect(page.getByTestId('notifications-table')).toBeVisible();
}

export async function loginAndVisitEventLog(page: Page) {
  await page.goto('/index.html');
  await completeHeaderLogin(page);
}
