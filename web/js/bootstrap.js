// @ts-check

const DEFAULT_CONFIG = Object.freeze({
  tauthBaseUrl: 'http://localhost:8081',
  landingUrl: '/index.html',
  dashboardUrl: '/dashboard.html',
});
const AUTH_CLIENT_SCRIPT_ATTRIBUTE = 'data-pinguin-auth-client';
const TAUTH_CONFIG = typeof window.PINGUIN_TAUTH_CONFIG === 'object' && window.PINGUIN_TAUTH_CONFIG
  ? window.PINGUIN_TAUTH_CONFIG
  : {};
const RUNTIME_CONFIG_URL_HINT =
  typeof window.__PINGUIN_RUNTIME_CONFIG_URL === 'string'
    ? window.__PINGUIN_RUNTIME_CONFIG_URL.trim()
    : '';

function deriveApiOriginFromConfig(config) {
  const apiBase = config && typeof config.apiBaseUrl === 'string' ? config.apiBaseUrl : '';
  if (apiBase.startsWith('http://') || apiBase.startsWith('https://')) {
    try {
      return new URL(apiBase).origin;
    } catch {
      // ignore invalid URL
    }
  }
  if (apiBase.startsWith('/')) {
    return '';
  }
  const { protocol, hostname, port } = window.location;
  if (port === '4173') {
    return `${protocol}//${hostname}:8080`;
  }
  if (port && port.length > 0) {
    return `${protocol}//${hostname}:${port}`;
  }
  return `${protocol}//${hostname}`;
}

function resolveRuntimeConfigCandidates(config, hint) {
  const candidates = [];
  if (hint) {
    candidates.push(hint);
  }
  const candidate =
    config && typeof config.runtimeConfigUrl === 'string'
      ? config.runtimeConfigUrl.trim()
      : '';
  if (candidate && !candidates.includes(candidate)) {
    candidates.push(candidate);
  }
  if (!candidates.length) {
    candidates.push('/runtime-config');
  }
  const origin = deriveApiOriginFromConfig(config);
  const apiCandidate = origin ? `${origin}/runtime-config` : null;
  if (apiCandidate && apiCandidate !== candidates[0]) {
    candidates.push(apiCandidate);
  }
  return candidates;
}

async function fetchRuntimeConfig(config, hint) {
  const candidates = resolveRuntimeConfigCandidates(config, hint);
  let lastError;
  for (const url of candidates) {
    try {
      console.info('runtime_config_candidate', url);
      const response = await fetch(url, { credentials: 'omit' });
      if (!response.ok) {
        lastError = new Error(`runtime_config_${response.status}`);
        continue;
      }
      return response.json();
    } catch (error) {
      lastError = error;
    }
  }
  throw lastError || new Error('runtime_config_failed');
}

function mergeConfig(base, overrides) {
  if (!overrides || typeof overrides !== 'object') {
    return { ...base };
  }
  return { ...base, ...overrides };
}

function buildAuthClientUrl(baseUrl) {
  if (typeof baseUrl !== 'string') {
    return '';
  }
  const trimmed = baseUrl.trim().replace(/\/+$/, '');
  if (!trimmed) {
    return '';
  }
  return `${trimmed}/static/auth-client.js`;
}

function ensureAuthClientScript(baseUrl) {
  if (typeof document === 'undefined') {
    return;
  }
  if (typeof window.initAuthClient === 'function') {
    return;
  }
  if (document.querySelector(`script[${AUTH_CLIENT_SCRIPT_ATTRIBUTE}]`)) {
    return;
  }
  const authClientUrl = buildAuthClientUrl(baseUrl);
  if (!authClientUrl) {
    return;
  }
  const script = document.createElement('script');
  script.defer = true;
  script.src = authClientUrl;
  script.crossOrigin = 'anonymous';
  script.setAttribute(AUTH_CLIENT_SCRIPT_ATTRIBUTE, 'true');
  document.head.appendChild(script);
}

(async function bootstrap() {
  const preloaded = window.__PINGUIN_CONFIG__ || {};
  const skipRemote = Boolean(preloaded && preloaded.skipRemoteConfig);
  const preloadedTenant =
    preloaded && typeof preloaded.tenant === 'object' ? preloaded.tenant : null;
  const tauthSeed = {};
  if (typeof TAUTH_CONFIG.baseUrl === 'string') {
    tauthSeed.tauthBaseUrl = TAUTH_CONFIG.baseUrl;
  }
  if (typeof TAUTH_CONFIG.googleClientId === 'string') {
    tauthSeed.googleClientId = TAUTH_CONFIG.googleClientId;
  }
  let effectiveConfig = mergeConfig(DEFAULT_CONFIG, tauthSeed);
  effectiveConfig = mergeConfig(effectiveConfig, preloaded);
  let resolvedTenant = preloadedTenant;
  if (!skipRemote) {
    try {
      const remote = await fetchRuntimeConfig(preloaded || null, RUNTIME_CONFIG_URL_HINT);
      if (remote && typeof remote.tenant === 'object') {
        resolvedTenant = remote.tenant;
      }
      const apiOverride =
        remote && typeof remote.apiBaseUrl === 'string' ? { apiBaseUrl: remote.apiBaseUrl } : {};
      effectiveConfig = mergeConfig(effectiveConfig, apiOverride);
    } catch (error) {
      console.warn('runtime config fetch failed', error);
    }
  }
  if (resolvedTenant) {
    effectiveConfig.tenant = resolvedTenant;
    const identity = resolvedTenant.identity || {};
    if (typeof identity.googleClientId === 'string' && identity.googleClientId.trim()) {
      effectiveConfig.googleClientId = identity.googleClientId.trim();
    }
    if (typeof identity.tauthBaseUrl === 'string' && identity.tauthBaseUrl.trim()) {
      effectiveConfig.tauthBaseUrl = identity.tauthBaseUrl.trim();
    }
  }
  const finalConfig = {
    apiBaseUrl: effectiveConfig.apiBaseUrl,
    tauthBaseUrl: effectiveConfig.tauthBaseUrl || DEFAULT_CONFIG.tauthBaseUrl,
    googleClientId: effectiveConfig.googleClientId,
    landingUrl: effectiveConfig.landingUrl || DEFAULT_CONFIG.landingUrl,
    dashboardUrl: effectiveConfig.dashboardUrl || DEFAULT_CONFIG.dashboardUrl,
    tenant: effectiveConfig.tenant || null,
  };
  window.__PINGUIN_CONFIG__ = finalConfig;
  ensureAuthClientScript(finalConfig.tauthBaseUrl);
  window.dispatchEvent(new CustomEvent('pinguin:config-updated', { detail: finalConfig }));
  if (finalConfig.tenant) {
    applyTenantBranding(finalConfig.tenant);
  }
  if (typeof window.initAuthClient !== 'function') {
    console.warn('auth-client.js has not finished loading; authentication may be delayed.');
  }
  await import('./app.js');
})();

function applyTenantBranding(tenantConfig) {
  const label =
    (tenantConfig && typeof tenantConfig.displayName === 'string' && tenantConfig.displayName.trim()) ||
    'Pinguin';
  const update = () => {
    document.querySelectorAll('mpr-header').forEach((header) => {
      header.setAttribute('brand-label', label);
    });
    document.documentElement.dataset.tenantId = tenantConfig?.id || '';
    if (document.title && document.title.toLowerCase().includes('pinguin')) {
      document.title = `${label} — Notification Service`;
    }
  };
  requestAnimationFrame(update);
}
