(function applyTauthConfig() {
  const fallback = window.PINGUIN_TAUTH_CONFIG || {};
  function resolveConfig() {
    const runtime = window.__PINGUIN_CONFIG__ || {};
    const base = runtime.tauthBaseUrl || fallback.baseUrl || '';
    const googleClientId = runtime.googleClientId || fallback.googleClientId || '';
    if (!runtime.tauthBaseUrl && base) {
      runtime.tauthBaseUrl = base;
    }
    if (!runtime.googleClientId && googleClientId) {
      runtime.googleClientId = googleClientId;
    }
    return {
      baseUrl: typeof base === 'string' ? base.replace(/\/$/, '') : '',
      googleClientId: typeof googleClientId === 'string' ? googleClientId.trim() : '',
    };
  }

  function applyAttributes() {
    const config = resolveConfig();
    const headers = document.querySelectorAll('mpr-header');
    headers.forEach((header) => {
      if (config.googleClientId) {
        header.setAttribute('site-id', config.googleClientId);
      }
      if (config.baseUrl) {
        header.setAttribute('base-url', config.baseUrl);
      }
      if (!header.getAttribute('login-path')) {
        header.setAttribute('login-path', '/auth/google');
      }
      if (!header.getAttribute('logout-path')) {
        header.setAttribute('logout-path', '/auth/logout');
      }
      if (!header.getAttribute('nonce-path')) {
        header.setAttribute('nonce-path', '/auth/nonce');
      }
    });
    const loginButtons = document.querySelectorAll('mpr-login-button');
    loginButtons.forEach((button) => {
      if (config.googleClientId) {
        button.setAttribute('site-id', config.googleClientId);
      }
      if (config.baseUrl) {
        button.setAttribute('base-url', config.baseUrl);
      }
      button.setAttribute('login-path', '/auth/google');
      button.setAttribute('logout-path', '/auth/logout');
      button.setAttribute('nonce-path', '/auth/nonce');
    });
  }

  const initialize = () => requestAnimationFrame(applyAttributes);
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initialize, { once: true });
  } else {
    initialize();
  }
  window.addEventListener('pinguin:config-updated', () => requestAnimationFrame(applyAttributes));
})();
