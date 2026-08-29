import { expect, test, type Locator, type Page } from '@playwright/test';
import { configureRuntime, resetNotifications, stubExternalAssets } from './utils';

const VIEWPORTS = [
  { name: 'mobile', width: 390, height: 844 },
  { name: 'tablet', width: 768, height: 1024 },
  { name: 'desktop', width: 1440, height: 1000 },
];

const PLATFORM_SERVICES = [
  { label: 'LLM Proxy', href: 'https://llm-proxy.mprlab.com/' },
  { label: 'TAuth', href: 'https://tauth.mprlab.com/' },
  { label: 'Ledger', href: 'https://github.com/tyemirov/ledger' },
  { label: 'ISSUES.md', href: 'https://issues.mprlab.com/' },
  { label: 'Dictator', href: 'https://github.com/tyemirov/dictator' },
  { label: 'Pinguin', href: 'https://pinguin.mprlab.com/' },
  { label: 'LoopAware', href: 'https://loopaware.mprlab.com/' },
];

const LOOPAWARE_PIXEL_URL =
  'https://loopaware.mprlab.com/pixel.js?site_id=f42bd5af-db6b-41e1-811e-5c04332ecdee';

async function expectNoHorizontalOverflow(page: Page) {
  await expect
    .poll(() =>
      page.evaluate(() => ({
        documentWidth: document.documentElement.scrollWidth,
        viewportWidth: window.innerWidth,
      })),
    )
    .toEqual(await page.evaluate(() => ({
      documentWidth: window.innerWidth,
      viewportWidth: window.innerWidth,
    })));
}

async function expectWorkSurfaceClearOfFooter(page: Page) {
  const workSurface = page.getByTestId('page-work-surface');
  const footer = page.locator('mpr-footer');
  await workSurface.scrollIntoViewIfNeeded();
  await page.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
  await expect
    .poll(async () => {
      const workSurfaceBox = await workSurface.boundingBox();
      const footerBox = await footer.boundingBox();
      if (!workSurfaceBox || !footerBox) {
        return false;
      }
      return workSurfaceBox.y + workSurfaceBox.height <= footerBox.y + 1;
    })
    .toBe(true);
}

async function expectCompactMPRTheme(page: Page) {
  await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  await expect(page.locator('body')).toHaveAttribute('data-theme', 'dark');
  await expect
    .poll(() => page.locator('body').evaluate((element) => getComputedStyle(element).backgroundColor))
    .toBe('rgb(15, 17, 20)');
}

async function expectWorkspaceNavigation(page: Page, currentLabel: string) {
  const header = page.locator('mpr-header');
  const navigation = header.getByRole('navigation', { name: 'Primary navigation' });
  const workspaceLinks = navigation.locator('.workspace-navigation__link');
  await expect(navigation).toBeVisible();
  await expect(workspaceLinks).toHaveText(['Tenants', 'Event log', 'SMTP relay']);
  await expect(workspaceLinks.filter({ hasText: currentLabel })).toHaveAttribute(
    'aria-current',
    'page',
  );
  await expect(page.locator('workspace-navigation')).toHaveCount(0);
}

async function expectFocusRestored(trigger: Locator) {
  await expect(trigger).toBeFocused();
}

test.describe('Compact MPR UX', () => {
  test.beforeEach(async ({ page, request }) => {
    await resetNotifications(request);
    await stubExternalAssets(page);
  });

  for (const viewport of VIEWPORTS) {
    test(`keeps every page within the ${viewport.name} viewport and above the footer`, async ({
      page,
    }) => {
      await page.setViewportSize({ width: viewport.width, height: viewport.height });

      await configureRuntime(page, { authenticated: false });
      await page.goto('/index.html');
      await expectNoHorizontalOverflow(page);
      await expectWorkSurfaceClearOfFooter(page);

      await configureRuntime(page, { authenticated: true });
      for (const path of ['/tenants.html', '/event-log.html', '/smtp-relay.html']) {
        await page.goto(path);
        await expectNoHorizontalOverflow(page);
        await expectWorkSurfaceClearOfFooter(page);
      }
    });
  }

  test('loads the current Pinguin LoopAware pixel on every page', async ({ page }) => {
    const pages = [
      { path: '/index.html', authenticated: false },
      { path: '/tenants.html', authenticated: true },
      { path: '/event-log.html', authenticated: true },
      { path: '/smtp-relay.html', authenticated: true },
    ];

    for (const pageContract of pages) {
      await configureRuntime(page, { authenticated: pageContract.authenticated });
      await page.goto(pageContract.path);
      await expect(page.locator(`script[src="${LOOPAWARE_PIXEL_URL}"]`)).toHaveCount(1);
    }
  });

  test('uses the compact dark shell and workspace navigation', async ({ page }) => {
    await configureRuntime(page, { authenticated: false });
    await page.goto('/index.html');
    await expectCompactMPRTheme(page);
    await expect(page.getByRole('heading', { name: 'Pinguin notification workspace' })).toBeVisible();
    await expect(page.getByTestId('notification-preview')).toBeVisible();

    await configureRuntime(page, { authenticated: true });
    await page.goto('/tenants.html');
    await expectCompactMPRTheme(page);
    await expectWorkspaceNavigation(page, 'Tenants');

    await page.goto('/event-log.html');
    await expectCompactMPRTheme(page);
    await expectWorkspaceNavigation(page, 'Event log');

    await page.goto('/smtp-relay.html');
    await expectCompactMPRTheme(page);
    await expectWorkspaceNavigation(page, 'SMTP relay');
  });

  test('places the workspace menu to the right of the Pinguin identity', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');

    const brand = page.locator('mpr-header .pinguin-brand');
    const firstWorkspaceLink = page
      .locator('mpr-header [data-mpr-header="nav"] .workspace-navigation__link')
      .first();
    await expect(brand).toBeVisible();
    await expect(firstWorkspaceLink).toBeVisible();
    const [brandBox, linkBox] = await Promise.all([
      brand.boundingBox(),
      firstWorkspaceLink.boundingBox(),
    ]);
    expect(brandBox).not.toBeNull();
    expect(linkBox).not.toBeNull();
    expect(linkBox!.x).toBeGreaterThan(brandBox!.x + brandBox!.width);
  });

  test('opens the seven-service MPR Platform catalog from every footer', async ({ page }) => {
    const pages = [
      { path: '/index.html', authenticated: false },
      { path: '/tenants.html', authenticated: true },
      { path: '/event-log.html', authenticated: true },
      { path: '/smtp-relay.html', authenticated: true },
    ];

    for (const pageContract of pages) {
      await configureRuntime(page, { authenticated: pageContract.authenticated });
      await page.goto(pageContract.path);

      const footer = page.locator('mpr-footer');
      const toggle = footer.getByRole('button', {
        name: 'Built By Marco Polo Research Lab',
        exact: true,
      });
      await expect(toggle).toBeVisible();
      await toggle.click();
      await expect(toggle).toHaveAttribute('aria-expanded', 'true');

      const serviceLinks = footer.locator('[data-mpr-footer="menu-link"]');
      await expect(serviceLinks).toHaveText(PLATFORM_SERVICES.map((service) => service.label));
      for (const [index, service] of PLATFORM_SERVICES.entries()) {
        await expect(serviceLinks.nth(index)).toHaveAttribute('href', service.href);
      }
    }
  });

  test('renders dense notification records with actions only for queued work', async ({ page, request }) => {
    const now = new Date().toISOString();
    await resetNotifications(request, {
      notifications: [
        {
          notification_id: 'notif-queued',
          tenant_id: 'tenant-devserver',
          notification_type: 'email',
          recipient: 'queued@example.com',
          subject: 'Queued launch notice',
          message: 'Queued body',
          status: 'queued',
          created_at: now,
          updated_at: now,
          scheduled_for: now,
          retry_count: 0,
        },
        {
          notification_id: 'notif-sent',
          tenant_id: 'tenant-devserver',
          notification_type: 'email',
          recipient: 'sent@example.com',
          subject: 'Sent launch notice',
          message: 'Sent body',
          status: 'sent',
          created_at: now,
          updated_at: now,
          scheduled_for: null,
          retry_count: 0,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');

    const list = page.getByTestId('notifications-list');
    const records = list.getByTestId('notification-row');
    await expect(list).toHaveAttribute('role', 'list');
    await expect(records).toHaveCount(2);
    await expect(page.getByTestId('loaded-count')).toHaveText('2 loaded');

    const queuedRecord = records.filter({ hasText: 'Queued launch notice' });
    await expect(queuedRecord).toContainText('queued@example.com');
    await expect(queuedRecord).toContainText('tenant-devserver');
    await expect(queuedRecord.getByRole('button', { name: 'Reschedule' })).toBeVisible();
    await expect(queuedRecord.getByRole('button', { name: 'Cancel' })).toBeVisible();

    const sentRecord = records.filter({ hasText: 'Sent launch notice' });
    await expect(sentRecord).toContainText('sent@example.com');
    await expect(sentRecord.getByRole('button')).toHaveCount(0);
  });

  test('uses an application confirmation dialog and restores notification action focus', async ({
    page,
  }) => {
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    const cancelTrigger = page
      .getByTestId('notification-row')
      .getByRole('button', { name: 'Cancel', exact: true });
    await cancelTrigger.click();

    const dialog = page.getByRole('dialog', { name: 'Cancel notification' });
    await expect(dialog).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusRestored(cancelTrigger);
  });

  test('shows one sender-domain DNS panel with row-level results and copy feedback', async ({
    page,
    request,
  }) => {
    await page.context().grantPermissions(['clipboard-read', 'clipboard-write']);
    await resetNotifications(request, {
      smtpDomains: [
        { domain: 'alpha.example', status: 'pending' },
        { domain: 'bravo.example', status: 'pending' },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');

    const domains = page.getByTestId('smtp-domain-card');
    const alphaDomain = domains.filter({ hasText: 'alpha.example' });
    const bravoDomain = domains.filter({ hasText: 'bravo.example' });
    await expect(alphaDomain.getByTestId('smtp-domain-details')).toBeHidden();
    await expect(bravoDomain.getByTestId('smtp-domain-details')).toBeHidden();

    await alphaDomain.getByRole('button', { name: 'Expand alpha.example DNS setup' }).click();
    await expect(alphaDomain.getByTestId('smtp-domain-details')).toBeVisible();
    await bravoDomain.getByRole('button', { name: 'Expand bravo.example DNS setup' }).click();
    await expect(bravoDomain.getByTestId('smtp-domain-details')).toBeVisible();
    await expect(alphaDomain.getByTestId('smtp-domain-details')).toBeHidden();

    await bravoDomain.getByRole('button', { name: 'Check DNS' }).click();
    const dnsRows = bravoDomain.getByTestId('dns-record-row');
    await expect(dnsRows).toHaveCount(3);
    await expect(dnsRows).toContainText(['DNS record matched', 'DNS record matched', 'DNS record matched']);

    await dnsRows.first().getByRole('button', { name: 'Copy DNS host' }).click();
    await expect(page.getByRole('button', { name: 'DNS host copied' }).first()).toBeVisible();
  });

  test('builds identities from a local part and verified sender domain', async ({ page, request }) => {
    await resetNotifications(request, {
      smtpDomains: [{ domain: 'example.com', status: 'verified' }],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');

    await page.getByRole('button', { name: 'Create identity' }).click();
    const dialog = page.getByRole('dialog', { name: 'Create SMTP identity' });
    await dialog.getByLabel('Local part').fill('alice');
    await dialog.getByLabel('Sender domain').selectOption('example.com');
    await dialog.getByLabel('Forward copies to').fill('owner@example.com');
    await dialog.getByRole('button', { name: 'Create identity' }).click();

    await expect(page.getByTestId('smtp-identity-row')).toContainText('alice@example.com');
    await expect(page.getByRole('dialog', { name: 'Gmail SMTP settings' })).toBeVisible();
  });

  test('uses an application confirmation dialog and restores identity action focus', async ({
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
    await configureRuntime(page, { authenticated: true });
    await page.goto('/smtp-relay.html');
    const deleteTrigger = page.getByRole('button', { name: 'Delete' });
    await deleteTrigger.click();

    const dialog = page.getByRole('dialog', { name: 'Delete SMTP identity' });
    await expect(dialog).toBeVisible();
    await dialog.getByRole('button', { name: 'Keep identity' }).click();
    await expect(dialog).toBeHidden();
    await expectFocusRestored(deleteTrigger);
  });

  test('captures focused Pinguin-owned visual surfaces', async ({ page, request }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await configureRuntime(page, { authenticated: false });
    await page.goto('/index.html');
    await expect(page.getByRole('status')).toHaveText('Workspace ready');
    await expect(page.getByTestId('page-work-surface')).toHaveScreenshot('compact-landing.png');

    const fixedTimestamp = '2030-01-02T03:04:00.000Z';
    await resetNotifications(request, {
      notifications: [
        {
          notification_id: 'notif-visual',
          tenant_id: 'tenant-devserver',
          notification_type: 'email',
          recipient: 'operator@example.com',
          subject: 'Queued notification',
          message: 'Visual baseline',
          status: 'queued',
          created_at: fixedTimestamp,
          updated_at: fixedTimestamp,
          scheduled_for: fixedTimestamp,
          retry_count: 0,
        },
      ],
    });
    await configureRuntime(page, { authenticated: true });
    await page.goto('/event-log.html');
    await expect(page.getByTestId('page-work-surface')).toHaveScreenshot('compact-event-log.png');

    await resetNotifications(request, {
      smtpDomains: [{ domain: 'example.com', status: 'verified' }],
      smtpIdentities: [
        {
          id: 'smtp-visual',
          email_address: 'alerts@example.com',
          username: 'smtp_visual',
          forward_to: ['operator@example.com'],
          status: 'active',
          last_used_at: fixedTimestamp,
          created_at: fixedTimestamp,
          updated_at: fixedTimestamp,
        },
      ],
    });
    await page.goto('/smtp-relay.html');
    await expect(page.getByTestId('page-work-surface')).toHaveScreenshot('compact-smtp-relay.png', {
      mask: [page.locator('.domain-item__checked')],
      maskColor: '#16181c',
    });
  });
});
