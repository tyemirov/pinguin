// @ts-check
(function applyTauthConfig() {
  const getRuntimeConfig = () => {
    const runtime = window.__PINGUIN_CONFIG__;
    return runtime && typeof runtime === 'object' ? runtime : null;
  };

  const requireString = (value, code) => {
    const normalized = typeof value === 'string' ? value.trim() : '';
    if (!normalized) {
      throw new Error(code);
    }
    return normalized;
  };

  function resolveConfig() {
    const runtime = getRuntimeConfig();
    if (!runtime) {
      throw new Error('tauth.config.runtime_missing');
    }
    const tenant =
      runtime && typeof runtime.tenant === 'object' ? runtime.tenant : null;
    if (!tenant) {
      throw new Error('tauth.config.tenant_missing');
    }
    const tenantId = requireString(
      runtime.tauthTenantId,
      'tauth.config.tenant_id_missing',
    );
    const baseUrl = requireString(runtime.tauthBaseUrl, 'tauth.config.base_url_missing').replace(/\/$/, '');
    const googleClientId = requireString(
      runtime.googleClientId,
      'tauth.config.google_client_id_missing',
    );
    return {
      baseUrl,
      googleClientId,
      tenantId,
      tenant,
    };
  }

  const requireMprUiInit = () => {
    const api = window.MPRUI && typeof window.MPRUI.init === 'function' ? window.MPRUI.init : null;
    if (!api) {
      throw new Error('mpr-ui.init.missing');
    }
    return api;
  };

  const buildMprUiInitConfig = (config) => {
    const initConfig = {
      auth: {
        googleSiteId: config.googleClientId,
        tauthTenantId: config.tenantId,
        tauthUrl: config.baseUrl,
        tauthLoginPath: '/auth/google',
        tauthLogoutPath: '/auth/logout',
        tauthNoncePath: '/auth/nonce',
      },
    };
    const displayName =
      config.tenant &&
      typeof config.tenant.displayName === 'string' &&
      config.tenant.displayName.trim()
        ? config.tenant.displayName.trim()
        : '';
    if (displayName) {
      initConfig.header = {
        brandLabel: displayName,
      };
    }
    return initConfig;
  };

  function applyAttributes() {
    const config = resolveConfig();
    window.__TAUTH_TENANT_ID__ = config.tenantId;
    if (document.documentElement) {
      document.documentElement.setAttribute('data-tauth-tenant-id', config.tenantId);
    }
    const initConfig = buildMprUiInitConfig(config);
    window.__MPR_UI_INIT__ = initConfig;
    const initMprUi = requireMprUiInit();
    initMprUi(initConfig);
  }

  const applyWhenReady = () => {
    const runtime = getRuntimeConfig();
    if (!runtime || !runtime.tenant) {
      return;
    }
    applyAttributes();
  };

  applyWhenReady();
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', applyWhenReady, { once: true });
  }
  window.addEventListener('pinguin:config-updated', () => requestAnimationFrame(applyAttributes));
})();
