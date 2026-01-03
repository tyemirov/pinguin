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
    };
  }

  function applyAttributes() {
    const config = resolveConfig();
    window.__TAUTH_TENANT_ID__ = config.tenantId;
    if (document.documentElement) {
      document.documentElement.setAttribute('data-tauth-tenant-id', config.tenantId);
    }
    const headers = document.querySelectorAll('mpr-header');
    headers.forEach((header) => {
      header.setAttribute('google-site-id', config.googleClientId);
      header.setAttribute('tauth-tenant-id', config.tenantId);
      header.setAttribute('tauth-url', config.baseUrl);
      if (!header.getAttribute('tauth-login-path')) {
        header.setAttribute('tauth-login-path', '/auth/google');
      }
      if (!header.getAttribute('tauth-logout-path')) {
        header.setAttribute('tauth-logout-path', '/auth/logout');
      }
      if (!header.getAttribute('tauth-nonce-path')) {
        header.setAttribute('tauth-nonce-path', '/auth/nonce');
      }
    });
    const loginButtons = document.querySelectorAll('mpr-login-button');
    loginButtons.forEach((button) => {
      button.setAttribute('site-id', config.googleClientId);
      button.setAttribute('tauth-tenant-id', config.tenantId);
      button.setAttribute('tauth-url', config.baseUrl);
      if (!button.getAttribute('tauth-login-path')) {
        button.setAttribute('tauth-login-path', '/auth/google');
      }
      if (!button.getAttribute('tauth-logout-path')) {
        button.setAttribute('tauth-logout-path', '/auth/logout');
      }
      if (!button.getAttribute('tauth-nonce-path')) {
        button.setAttribute('tauth-nonce-path', '/auth/nonce');
      }
    });
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
