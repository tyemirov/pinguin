// @ts-check

const PRODUCTION_TAUTH_BASE_URL = 'https://tauth.mprlab.com';
const LOCAL_TAUTH_BASE_URL = 'http://localhost:8081';
const PRODUCTION_API_ORIGIN = 'https://pinguin-api.mprlab.com';
const LOCAL_API_ORIGIN = 'http://localhost:8080';
const currentHostname = window.location.hostname || '';
const isProductionHost = currentHostname.endsWith('.mprlab.com');
const resolvedBaseUrl = isProductionHost ? PRODUCTION_TAUTH_BASE_URL : LOCAL_TAUTH_BASE_URL;
const resolvedApiOrigin = isProductionHost ? PRODUCTION_API_ORIGIN : LOCAL_API_ORIGIN;
const resolvedRuntimeConfigUrl = `${resolvedApiOrigin}/runtime-config`;
const resolvedApiBaseUrl = `${resolvedApiOrigin}/api`;

window.PINGUIN_TAUTH_CONFIG =
  window.PINGUIN_TAUTH_CONFIG ||
  Object.freeze({
    baseUrl: resolvedBaseUrl,
    googleClientId: '991677581607-r0dj8q6irjagipali0jpca7nfp8sfj9r.apps.googleusercontent.com',
  });

if (!window.__PINGUIN_CONFIG__) {
  window.__PINGUIN_CONFIG__ = {};
}

if (!window.__PINGUIN_CONFIG__.runtimeConfigUrl) {
  window.__PINGUIN_CONFIG__.runtimeConfigUrl = resolvedRuntimeConfigUrl;
}

if (!window.__PINGUIN_CONFIG__.apiBaseUrl) {
  window.__PINGUIN_CONFIG__.apiBaseUrl = resolvedApiBaseUrl;
}
