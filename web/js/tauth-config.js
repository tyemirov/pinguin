// @ts-check

const PRODUCTION_API_ORIGIN = 'https://pinguin-api.mprlab.com';
const LOCAL_API_ORIGIN = 'http://localhost:8080';
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
