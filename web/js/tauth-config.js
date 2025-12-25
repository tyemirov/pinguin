const PRODUCTION_TAUTH_BASE_URL = 'https://tauth.mprlab.com';
const LOCAL_TAUTH_BASE_URL = 'http://localhost:8081';
const currentHostname = window.location.hostname || '';
const resolvedBaseUrl = currentHostname.endsWith('.mprlab.com')
  ? PRODUCTION_TAUTH_BASE_URL
  : LOCAL_TAUTH_BASE_URL;

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
  window.__PINGUIN_CONFIG__.runtimeConfigUrl = 'http://localhost:8080/runtime-config';
}

if (!window.__PINGUIN_CONFIG__.apiBaseUrl) {
  window.__PINGUIN_CONFIG__.apiBaseUrl = 'http://localhost:8080/api';
}
