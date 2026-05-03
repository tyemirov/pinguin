// @ts-check
(function registerMprUiAuthCache() {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return;
  }

  const AUTH_STATE_KEY = '__PINGUIN_AUTH_STATE__';
  const AUTH_LISTENER_KEY = '__PINGUIN_AUTH_LISTENING__';

  if (window[AUTH_LISTENER_KEY]) {
    return;
  }

  const recordAuthState = (status, profile) => {
    window[AUTH_STATE_KEY] = { status, profile: profile || null };
  };

  window[AUTH_LISTENER_KEY] = true;
  document.addEventListener('mpr-ui:auth:authenticated', (event) => {
    recordAuthState('authenticated', event?.detail?.profile || null);
  });
  document.addEventListener('mpr-ui:auth:unauthenticated', () => {
    recordAuthState('unauthenticated', null);
  });
  document.addEventListener('mpr-ui:auth:status-change', (event) => {
    if (event?.detail?.status === 'unauthenticated') {
      recordAuthState('unauthenticated', null);
    }
  });
})();
