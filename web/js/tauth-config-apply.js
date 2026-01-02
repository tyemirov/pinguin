// @ts-check
(function applyTauthConfig() {
  const fallback = window.PINGUIN_TAUTH_CONFIG || {};
  function resolveTenantId(runtime, fallbackConfig) {
    const runtimeOverride =
      typeof runtime.tauthTenantId === 'string' ? runtime.tauthTenantId.trim() : '';
    if (runtimeOverride) {
      return runtimeOverride;
    }
    const tenant =
      runtime && typeof runtime.tenant === 'object' ? runtime.tenant : null;
    const identity =
      tenant && typeof tenant.identity === 'object' ? tenant.identity : null;
    const identityOverride =
      identity && typeof identity.tauthTenantId === 'string' ? identity.tauthTenantId.trim() : '';
    if (identityOverride) {
      return identityOverride;
    }
    const fallbackOverride =
      typeof fallbackConfig.tenantId === 'string' ? fallbackConfig.tenantId.trim() : '';
    if (fallbackOverride) {
      return fallbackOverride;
    }
    const origin =
      typeof window.location === 'object' && typeof window.location.origin === 'string'
        ? window.location.origin.trim()
        : '';
    if (origin && origin !== 'null') {
      return origin;
    }
    return tenant && typeof tenant.id === 'string' ? tenant.id.trim() : '';
  }
  function resolveConfig() {
    const runtime = window.__PINGUIN_CONFIG__ || {};
    const base = runtime.tauthBaseUrl || fallback.baseUrl || '';
    const googleClientId = runtime.googleClientId || fallback.googleClientId || '';
    const tenantId = resolveTenantId(runtime, fallback);
    if (!runtime.tauthBaseUrl && base) {
      runtime.tauthBaseUrl = base;
    }
    if (!runtime.googleClientId && googleClientId) {
      runtime.googleClientId = googleClientId;
    }
    return {
      baseUrl: typeof base === 'string' ? base.replace(/\/$/, '') : '',
      googleClientId: typeof googleClientId === 'string' ? googleClientId.trim() : '',
      tenantId,
    };
  }

  function applyAttributes() {
    const config = resolveConfig();
    const hasTenant = Boolean(config.tenantId);
    if (hasTenant) {
      window.__TAUTH_TENANT_ID__ = config.tenantId;
      if (document.documentElement) {
        document.documentElement.setAttribute('data-tauth-tenant-id', config.tenantId);
      }
    } else {
      delete window.__TAUTH_TENANT_ID__;
      if (document.documentElement) {
        document.documentElement.removeAttribute('data-tauth-tenant-id');
      }
    }
    const headers = document.querySelectorAll('mpr-header');
    headers.forEach((header) => {
      if (config.googleClientId) {
        header.setAttribute('google-site-id', config.googleClientId);
      } else {
        header.removeAttribute('google-site-id');
      }
      if (hasTenant) {
        header.setAttribute('tauth-tenant-id', config.tenantId);
        if (config.baseUrl) {
          header.setAttribute('tauth-url', config.baseUrl);
        } else {
          header.removeAttribute('tauth-url');
        }
        if (!header.getAttribute('tauth-login-path')) {
          header.setAttribute('tauth-login-path', '/auth/google');
        }
        if (!header.getAttribute('tauth-logout-path')) {
          header.setAttribute('tauth-logout-path', '/auth/logout');
        }
        if (!header.getAttribute('tauth-nonce-path')) {
          header.setAttribute('tauth-nonce-path', '/auth/nonce');
        }
      } else {
        header.removeAttribute('tauth-tenant-id');
        header.removeAttribute('tauth-url');
        header.removeAttribute('tauth-login-path');
        header.removeAttribute('tauth-logout-path');
        header.removeAttribute('tauth-nonce-path');
      }
    });
    const loginButtons = document.querySelectorAll('mpr-login-button');
    loginButtons.forEach((button) => {
      if (config.googleClientId) {
        button.setAttribute('site-id', config.googleClientId);
      } else {
        button.removeAttribute('site-id');
      }
      if (hasTenant) {
        button.setAttribute('tauth-tenant-id', config.tenantId);
        if (config.baseUrl) {
          button.setAttribute('tauth-url', config.baseUrl);
        } else {
          button.removeAttribute('tauth-url');
        }
        if (!button.getAttribute('tauth-login-path')) {
          button.setAttribute('tauth-login-path', '/auth/google');
        }
        if (!button.getAttribute('tauth-logout-path')) {
          button.setAttribute('tauth-logout-path', '/auth/logout');
        }
        if (!button.getAttribute('tauth-nonce-path')) {
          button.setAttribute('tauth-nonce-path', '/auth/nonce');
        }
      } else {
        button.removeAttribute('tauth-tenant-id');
        button.removeAttribute('tauth-url');
        button.removeAttribute('tauth-login-path');
        button.removeAttribute('tauth-logout-path');
        button.removeAttribute('tauth-nonce-path');
      }
    });
  }

  applyAttributes();
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', applyAttributes, { once: true });
  }
  window.addEventListener('pinguin:config-updated', () => requestAnimationFrame(applyAttributes));
})();
