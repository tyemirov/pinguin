// @ts-check
(function () {
  const channel = typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('auth') : null;
  const authState = {
    baseUrl: '',
    tenantId: '',
  };
  function getSession() {
    if (!window.__mockAuth) {
      window.__mockAuth = { authenticated: false };
    }
    return window.__mockAuth;
  }
  function ensureProfile() {
    const session = getSession();
    if (!session.profile) {
      session.profile = {
        user_email: 'playwright@example.com',
        user_display_name: 'Playwright User',
        user_avatar_url: '',
      };
    }
    return session.profile;
  }
  function emit(eventName) {
    if (channel) {
      channel.postMessage(eventName);
    }
  }
  window.getCurrentUser = function getCurrentUser() {
    const session = getSession();
    if (!session.authenticated) {
      return null;
    }
    return (
      session.profile || {
        user_email: 'playwright@example.com',
        user_display_name: 'Playwright User',
        user_avatar_url: '',
      }
    );
  };
  window.setAuthTenantId = function setAuthTenantId(tenantId) {
    authState.tenantId = tenantId || '';
  };
  window.initAuthClient = async function initAuthClient(options) {
    authState.baseUrl = options?.baseUrl || authState.baseUrl;
    if (options?.tenantId) {
      authState.tenantId = options.tenantId;
    }
    const session = getSession();
    const profile = ensureProfile();
    if (session.authenticated) {
      options?.onAuthenticated?.(profile);
      emit('refreshed');
      return profile;
    }
    options?.onUnauthenticated?.();
    return null;
  };
  window.getAuthEndpoints = function getAuthEndpoints() {
    const baseUrl = authState.baseUrl ? authState.baseUrl.replace(/\/$/, '') : '';
    const origin = baseUrl || (window.location && window.location.origin) || '';
    return {
      baseUrl: origin,
      nonce: `${origin}/auth/nonce`,
      login: `${origin}/auth/google`,
      refresh: `${origin}/auth/refresh`,
      logout: `${origin}/auth/logout`,
      me: `${origin}/me`,
    };
  };
  window.requestNonce = async function requestNonce() {
    const randomSegment = Math.random().toString(16).split('.')[1] || '';
    return `playwright-nonce-${Date.now()}-${randomSegment}`;
  };
  window.exchangeGoogleCredential = async function exchangeGoogleCredential({ credential }) {
    if (!credential) {
      throw new Error('missing_credential');
    }
    const session = getSession();
    session.authenticated = true;
    const profile = ensureProfile();
    if (typeof window.__persistMockAuth === 'function') {
      window.__persistMockAuth();
    }
    emit('refreshed');
    return profile;
  };
  window.apiFetch = function apiFetch(input, init) {
    return fetch(input, { credentials: 'include', ...(init || {}) });
  };
  window.logout = async function logout() {
    const session = getSession();
    session.authenticated = false;
    if (typeof window.__persistMockAuth === 'function') {
      window.__persistMockAuth();
    }
    emit('logged_out');
  };
})();
