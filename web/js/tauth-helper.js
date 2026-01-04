// @ts-check
(function loadTauthHelper() {
  if (typeof document === 'undefined') {
    return;
  }
  if (document.querySelector('script[data-pinguin-auth-helper]')) {
    return;
  }
  const bundlePlaceholder = document.querySelector('script[data-mpr-ui-src]');
  const bundleUrl = bundlePlaceholder?.getAttribute('data-mpr-ui-src') || '';
  if (bundlePlaceholder) {
    bundlePlaceholder.remove();
  }
  const REQUIRED_HELPERS = [
    'initAuthClient',
    'requestNonce',
    'exchangeGoogleCredential',
    'logout',
    'getCurrentUser',
    'setAuthTenantId',
  ];
  const AUTH_STATE_KEY = '__PINGUIN_AUTH_STATE__';
  const AUTH_LISTENER_KEY = '__PINGUIN_AUTH_LISTENING__';
  const recordAuthState = (status, profile) => {
    window[AUTH_STATE_KEY] = { status, profile: profile || null };
  };
  if (!window[AUTH_LISTENER_KEY]) {
    window[AUTH_LISTENER_KEY] = true;
    document.addEventListener('mpr-ui:auth:authenticated', (event) => {
      recordAuthState('authenticated', event?.detail?.profile || null);
    });
    document.addEventListener('mpr-ui:auth:unauthenticated', () => {
      recordAuthState('unauthenticated', null);
    });
  }
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

  const resolveConfig = () => {
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
    const baseUrl = requireString(runtime.tauthBaseUrl, 'tauth.config.base_url_missing').replace(/\/+$/, '');
    return { baseUrl, tenantId };
  };

  const loadScript = (src, attrs = {}) =>
    new Promise((resolve, reject) => {
      const script = document.createElement('script');
      script.src = src;
      script.defer = true;
      script.async = false;
      script.crossOrigin = 'anonymous';
      Object.entries(attrs).forEach(([key, value]) => {
        if (value) {
          script.setAttribute(key, value);
        }
      });
      script.addEventListener('load', () => resolve(true));
      script.addEventListener('error', () => reject(new Error(`load_failed:${src}`)));
      document.head.appendChild(script);
    });

  const setTenantId = (tenantId) => {
    window.__TAUTH_TENANT_ID__ = tenantId;
    if (document.documentElement) {
      document.documentElement.setAttribute('data-tauth-tenant-id', tenantId);
    }
  };

  const dispatchAuthReady = (detail) => {
    window.__PINGUIN_AUTH_READY__ = true;
    window.dispatchEvent(new CustomEvent('pinguin:auth-ready', { detail }));
  };

  const dispatchAuthError = (error) => {
    const message = error instanceof Error ? error.message : String(error);
    window.dispatchEvent(new CustomEvent('pinguin:auth-error', { detail: { message } }));
  };

  const requireTauthHelper = () => {
    const missing = REQUIRED_HELPERS.filter((key) => typeof window[key] !== 'function');
    if (missing.length > 0) {
      throw new Error(`tauth.helper.missing:${missing.join(',')}`);
    }
  };

  let started = false;
  const start = async () => {
    if (started) {
      return;
    }
    const { baseUrl, tenantId } = resolveConfig();
    started = true;
    setTenantId(tenantId);
    await loadScript(`${baseUrl}/tauth.js`, {
      'data-pinguin-auth-helper': 'true',
      'data-tenant-id': tenantId,
    });
    requireTauthHelper();
    if (bundleUrl) {
      await loadScript(bundleUrl, { id: 'mpr-ui-bundle' });
    }
    dispatchAuthReady({ baseUrl, tenantId });
  };

  const startWhenReady = () => {
    const runtime = getRuntimeConfig();
    if (!runtime || !runtime.tenant) {
      return;
    }
    start().catch(dispatchAuthError);
  };

  startWhenReady();
  window.addEventListener('pinguin:config-updated', () => requestAnimationFrame(startWhenReady));
})();
