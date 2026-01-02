// @ts-check
(function loadTauthHelper() {
  if (typeof document === 'undefined') {
    return;
  }
  const runtime = window.__PINGUIN_CONFIG__ || {};
  const fallback = window.PINGUIN_TAUTH_CONFIG || {};
  const base =
    typeof runtime.tauthBaseUrl === 'string' && runtime.tauthBaseUrl.trim()
      ? runtime.tauthBaseUrl
      : fallback.baseUrl || '';
  const baseUrl = typeof base === 'string' ? base.trim().replace(/\/+$/, '') : '';
  const tenant =
    runtime && typeof runtime.tenant === 'object' ? runtime.tenant : null;
  const tenantId =
    tenant && typeof tenant.id === 'string' ? tenant.id.trim() : '';
  if (tenantId) {
    window.__TAUTH_TENANT_ID__ = tenantId;
    if (document.documentElement) {
      document.documentElement.setAttribute('data-tauth-tenant-id', tenantId);
    }
  }
  const bundlePlaceholder = document.querySelector('script[data-mpr-ui-src]');
  const bundleUrl = bundlePlaceholder?.getAttribute('data-mpr-ui-src') || '';
  if (bundlePlaceholder) {
    bundlePlaceholder.remove();
  }
  if (document.querySelector('script[data-pinguin-auth-helper]')) {
    return;
  }

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

  const tauthPromise = baseUrl
    ? loadScript(`${baseUrl}/tauth.js`, {
        'data-pinguin-auth-helper': 'true',
        'data-tenant-id': tenantId,
      })
    : Promise.resolve();

  tauthPromise
    .catch(() => null)
    .then(() => {
      if (bundleUrl) {
        return loadScript(bundleUrl, { id: 'mpr-ui-bundle' });
      }
      return Promise.resolve();
    })
    .catch(() => {});
})();
