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
    const tenantId = requireString(tenant.id, 'tauth.config.tenant_id_missing');
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
    if (bundleUrl) {
      await loadScript(bundleUrl, { id: 'mpr-ui-bundle' });
    }
  };

  const startWhenReady = () => {
    const runtime = getRuntimeConfig();
    if (!runtime || !runtime.tenant) {
      return;
    }
    void start();
  };

  startWhenReady();
  window.addEventListener('pinguin:config-updated', () => requestAnimationFrame(startWhenReady));
})();
