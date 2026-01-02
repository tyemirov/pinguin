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
  if (!baseUrl) {
    return;
  }
  if (typeof window.initAuthClient === 'function') {
    return;
  }
  if (document.querySelector('script[data-pinguin-auth-helper]')) {
    return;
  }
  const script = document.createElement('script');
  script.defer = true;
  script.async = false;
  script.src = `${baseUrl}/tauth.js`;
  script.crossOrigin = 'anonymous';
  script.setAttribute('data-pinguin-auth-helper', 'true');
  if (tenantId) {
    script.setAttribute('data-tenant-id', tenantId);
  }
  document.head.appendChild(script);
})();
