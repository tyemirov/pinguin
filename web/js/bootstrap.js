// @ts-check

const DEFAULT_CONFIG = Object.freeze({
  landingUrl: '/index.html',
  dashboardUrl: '/dashboard.html',
});
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


(async function bootstrap() {
  const preloaded = window.__PINGUIN_CONFIG__ || {};
  const skipRemote = Boolean(preloaded && preloaded.skipRemoteConfig);
  const preloadedTenant =
    preloaded && typeof preloaded.tenant === 'object' ? preloaded.tenant : null;
  let effectiveConfig = mergeConfig(DEFAULT_CONFIG, null);
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
      const tauthOverrides = {};
      if (remote && typeof remote.tauthBaseUrl === 'string') {
        tauthOverrides.tauthBaseUrl = remote.tauthBaseUrl;
      }
      if (remote && typeof remote.googleClientId === 'string') {
        tauthOverrides.googleClientId = remote.googleClientId;
      }
      if (remote && typeof remote.tauthTenantId === 'string') {
        tauthOverrides.tauthTenantId = remote.tauthTenantId;
      }
      effectiveConfig = mergeConfig(effectiveConfig, apiOverride);
      effectiveConfig = mergeConfig(effectiveConfig, tauthOverrides);
    } catch (error) {
      console.warn('runtime config fetch failed', error);
    }
  }
  if (resolvedTenant) {
    effectiveConfig.tenant = resolvedTenant;
  }
  const finalConfig = {
    apiBaseUrl: effectiveConfig.apiBaseUrl,
    tauthBaseUrl: effectiveConfig.tauthBaseUrl,
    tauthTenantId: effectiveConfig.tauthTenantId,
    googleClientId: effectiveConfig.googleClientId,
    landingUrl: effectiveConfig.landingUrl || DEFAULT_CONFIG.landingUrl,
    dashboardUrl: effectiveConfig.dashboardUrl || DEFAULT_CONFIG.dashboardUrl,
    tenant: effectiveConfig.tenant || null,
  };
  window.__PINGUIN_CONFIG__ = finalConfig;
  window.dispatchEvent(new CustomEvent('pinguin:config-updated', { detail: finalConfig }));
  if (finalConfig.tenant) {
    applyTenantBranding(finalConfig.tenant);
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
