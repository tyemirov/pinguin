// @ts-check
(function applyTauthConfig() {
  const fallback = window.PINGUIN_TAUTH_CONFIG || {};
  function resolveConfig() {
    const runtime = window.__PINGUIN_CONFIG__ || {};
    const base = runtime.tauthBaseUrl || fallback.baseUrl || '';
    const googleClientId = runtime.googleClientId || fallback.googleClientId || '';
    const tenant =
      runtime && typeof runtime.tenant === 'object' ? runtime.tenant : null;
    const tenantId =
      tenant && typeof tenant.id === 'string' ? tenant.id.trim() : '';
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
    const headers = document.querySelectorAll('mpr-header');
    headers.forEach((header) => {
      if (config.googleClientId) {
        header.setAttribute('google-site-id', config.googleClientId);
      } else {
        header.removeAttribute('google-site-id');
      }
      if (config.baseUrl) {
        header.setAttribute('tauth-url', config.baseUrl);
      } else {
        header.removeAttribute('tauth-url');
      }
      if (config.tenantId) {
        header.setAttribute('tauth-tenant-id', config.tenantId);
      } else {
        header.removeAttribute('tauth-tenant-id');
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
    });
    const loginButtons = document.querySelectorAll('mpr-login-button');
    loginButtons.forEach((button) => {
      if (config.googleClientId) {
        button.setAttribute('site-id', config.googleClientId);
      } else {
        button.removeAttribute('site-id');
      }
      if (config.baseUrl) {
        button.setAttribute('tauth-url', config.baseUrl);
      } else {
        button.removeAttribute('tauth-url');
      }
      if (config.tenantId) {
        button.setAttribute('tauth-tenant-id', config.tenantId);
      } else {
        button.removeAttribute('tauth-tenant-id');
      }
      button.setAttribute('tauth-login-path', '/auth/google');
      button.setAttribute('tauth-logout-path', '/auth/logout');
      button.setAttribute('tauth-nonce-path', '/auth/nonce');
    });
  }

  applyAttributes();
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', applyAttributes, { once: true });
  }
  window.addEventListener('pinguin:config-updated', () => requestAnimationFrame(applyAttributes));
})();
