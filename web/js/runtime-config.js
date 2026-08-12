// @ts-check

const PRODUCTION_API_ORIGIN = 'https://pinguin-api.mprlab.com';
const LOCAL_API_ORIGIN = 'http://localhost:8081';
const currentHostname = window.location.hostname || '';
const isProductionHost = currentHostname.endsWith('.mprlab.com');
const resolvedApiOrigin = isProductionHost ? PRODUCTION_API_ORIGIN : LOCAL_API_ORIGIN;
const resolvedRuntimeConfigUrl = `${resolvedApiOrigin}/runtime-config`;
const resolvedApiBaseUrl = `${resolvedApiOrigin}/api`;

if (!window.__PINGUIN_CONFIG__) {
  window.__PINGUIN_CONFIG__ = {};
}

if (!window.__PINGUIN_CONFIG__.runtimeConfigUrl) {
  window.__PINGUIN_CONFIG__.runtimeConfigUrl = resolvedRuntimeConfigUrl;
}

if (!window.__PINGUIN_CONFIG__.apiBaseUrl) {
  window.__PINGUIN_CONFIG__.apiBaseUrl = resolvedApiBaseUrl;
}

// Capture the latest public mpr-ui auth event before the application module loads.
// The shared header may finish session restoration during deferred orchestration.
const rememberAuthEvent = (event) => {
  const detail = event && event.detail && typeof event.detail === 'object' ? event.detail : {};
  const eventStatus = event.type === 'mpr-ui:auth:authenticated'
    ? 'authenticated'
    : event.type === 'mpr-ui:auth:unauthenticated'
      ? 'unauthenticated'
      : detail.status;
  window.__PINGUIN_AUTH_EVENT_SNAPSHOT__ = {
    status: eventStatus || 'unknown',
    profile: detail.profile || null,
  };
};
document.addEventListener('mpr-ui:auth:authenticated', rememberAuthEvent);
document.addEventListener('mpr-ui:auth:unauthenticated', rememberAuthEvent);
document.addEventListener('mpr-ui:auth:status-change', rememberAuthEvent);

const THEME_STORAGE_KEY = 'pinguin.theme';
const resolveStoredTheme = () => {
  try {
    return window.localStorage.getItem(THEME_STORAGE_KEY);
  } catch {
    return null;
  }
};
const storedTheme = resolveStoredTheme();
if (storedTheme === 'light' || storedTheme === 'dark') {
  if (document && document.documentElement) {
    document.documentElement.setAttribute('data-theme', storedTheme);
    document.documentElement.setAttribute('data-mpr-theme', storedTheme);
  }
}

document.addEventListener('mpr-ui:theme-change', (event) => {
  const mode = event?.detail?.mode;
  if (mode !== 'light' && mode !== 'dark') {
    return;
  }
  try {
    window.localStorage.setItem(THEME_STORAGE_KEY, mode);
  } catch {
    // Storage might be unavailable in private sessions.
  }
});
