// @ts-check
(function () {
  const channel = typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('auth') : null;
  function getSession() {
    if (!window.__mockAuth) {
      window.__mockAuth = { authenticated: false };
    }
    return window.__mockAuth;
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
  window.setAuthTenantId = function setAuthTenantId() {};
  window.initAuthClient = async function initAuthClient(options) {
    const session = getSession();
    const profile =
      session.profile || {
        user_email: 'playwright@example.com',
        user_display_name: 'Playwright User',
        user_avatar_url: '',
      };
    if (session.authenticated) {
      options?.onAuthenticated?.(profile);
      emit('refreshed');
      return profile;
    }
    options?.onUnauthenticated?.();
    return null;
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
