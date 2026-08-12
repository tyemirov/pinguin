// @ts-check

const AUTH_STATUS_TIMEOUT_MS = 12000;
const AUTH_UNAUTHENTICATED_SETTLE_MS = 350;

/**
 * Normalizes the shared-shell profile fields used by Pinguin.
 *
 * @param {Record<string, any> | null} profile
 * @returns {Record<string, any> | null}
 */
function normalizeProfile(profile) {
  if (!profile || typeof profile !== 'object') {
    return null;
  }
  const display =
    typeof profile.display === 'string' && profile.display.trim()
      ? profile.display.trim()
      : typeof profile.user_display_name === 'string'
        ? profile.user_display_name.trim()
        : '';
  const avatarUrl =
    typeof profile.avatar_url === 'string' && profile.avatar_url.trim()
      ? profile.avatar_url.trim()
      : typeof profile.user_avatar_url === 'string'
        ? profile.user_avatar_url.trim()
        : '';
  return {
    ...profile,
    display,
    avatar_url: avatarUrl,
    user_display_name: display || profile.user_display_name || '',
    user_avatar_url: avatarUrl || profile.user_avatar_url || '',
  };
}

/**
 * Creates the single bridge from mpr-ui authentication events and snapshots to Alpine auth state.
 */
export function createSessionBridge() {
  let lastCallbacks = { onAuthenticated: undefined, onUnauthenticated: undefined };
  const statusListeners = new Set();
  let statusTimer = null;
  let unauthenticatedSettleTimer = null;
  let hasResolved = false;

  const applyProfile = (profile) => {
    const store = window.Alpine.store('auth');
    const normalized = normalizeProfile(profile);
    if (normalized) {
      store.setProfile(normalized);
    } else {
      store.clear();
    }
  };

  const invokeCallback = (name, payload) => {
    const callback = lastCallbacks[name];
    if (typeof callback === 'function') {
      callback(payload);
    }
  };

  const setStatus = (status) => {
    statusListeners.forEach((listener) => listener(status));
  };

  const clearStatusTimer = () => {
    if (statusTimer) {
      clearTimeout(statusTimer);
      statusTimer = null;
    }
  };

  const clearUnauthenticatedSettleTimer = () => {
    if (unauthenticatedSettleTimer) {
      clearTimeout(unauthenticatedSettleTimer);
      unauthenticatedSettleTimer = null;
    }
  };

  const startStatusTimer = () => {
    clearStatusTimer();
    statusTimer = setTimeout(() => {
      if (!hasResolved) {
        setStatus('error');
      }
    }, AUTH_STATUS_TIMEOUT_MS);
  };

  const handleHeaderAuthenticated = (event) => {
    const profile = event?.detail?.profile || null;
    hasResolved = true;
    clearUnauthenticatedSettleTimer();
    clearStatusTimer();
    applyProfile(profile);
    setStatus('ready');
    invokeCallback('onAuthenticated', profile);
  };

  const resolveUnauthenticated = () => {
    hasResolved = true;
    clearUnauthenticatedSettleTimer();
    clearStatusTimer();
    applyProfile(null);
    setStatus('ready');
    invokeCallback('onUnauthenticated');
  };

  const handleHeaderUnauthenticated = () => {
    clearUnauthenticatedSettleTimer();
    unauthenticatedSettleTimer = setTimeout(() => {
	  Promise.resolve(readAuthSnapshot())
		.then((snapshot) => {
		  if (statusFromSnapshot(snapshot) === 'authenticated') {
			handleHeaderAuthenticated({ detail: { profile: profileFromSnapshot(snapshot) } });
			return;
		  }
		  resolveUnauthenticated();
		})
		.catch(resolveUnauthenticated);
    }, AUTH_UNAUTHENTICATED_SETTLE_MS);
  };

  const handleHeaderStatusChange = (event) => {
    const status = event?.detail?.status || '';
    if (status === 'bootstrapping' || status === 'authenticating') {
      setStatus('hydrating');
      return;
    }
    if (status === 'unauthenticated' && !hasResolved) {
      handleHeaderUnauthenticated();
    }
  };

  document.addEventListener('mpr-ui:auth:authenticated', handleHeaderAuthenticated);
  document.addEventListener('mpr-ui:auth:unauthenticated', handleHeaderUnauthenticated);
  document.addEventListener('mpr-ui:auth:status-change', handleHeaderStatusChange);
  document.addEventListener('mpr-ui:auth:error', () => {
    if (!hasResolved) {
      setStatus('error');
      clearStatusTimer();
    }
  });

  const getAuthSnapshotTarget = () => {
    const header = document.querySelector('mpr-header');
    if (header && header.id) {
      return `#${header.id}`;
    }
    return 'mpr-header';
  };

  const looksLikeProfile = (value) => {
    if (!value || typeof value !== 'object') {
      return false;
    }
    return Boolean(
      value.user_email ||
        value.email ||
        value.user_display_name ||
        value.display ||
        value.user_id ||
        value.avatar_url,
    );
  };

  const profileFromSnapshot = (snapshot) => {
    if (!snapshot || typeof snapshot !== 'object') {
      return null;
    }
    if (looksLikeProfile(snapshot.profile)) {
      return snapshot.profile;
    }
    if (looksLikeProfile(snapshot)) {
      return snapshot;
    }
    return null;
  };

  const statusFromSnapshot = (snapshot) => {
    if (!snapshot || typeof snapshot !== 'object') {
      return 'unknown';
    }
    if (snapshot.status === 'authenticated' || snapshot.authenticated === true) {
      return profileFromSnapshot(snapshot) ? 'authenticated' : 'unknown';
    }
    if (snapshot.status === 'unauthenticated' || snapshot.authenticated === false) {
      return 'unauthenticated';
    }
    return profileFromSnapshot(snapshot) ? 'authenticated' : 'unknown';
  };

  const handleAuthSnapshot = (snapshot) => {
    const status = statusFromSnapshot(snapshot);
    if (status === 'authenticated') {
      handleHeaderAuthenticated({ detail: { profile: profileFromSnapshot(snapshot) } });
      return true;
    }
    if (status === 'unauthenticated') {
	  resolveUnauthenticated();
      return true;
    }
    return false;
  };

  const readAuthSnapshot = () => {
    const eventSnapshot = window.__PINGUIN_AUTH_EVENT_SNAPSHOT__;
    if (eventSnapshot && typeof eventSnapshot === 'object') {
      return eventSnapshot;
    }
    const namespace = window.MPRUI;
    if (namespace && typeof namespace.resolveAuthProfileSnapshot === 'function') {
      return namespace.resolveAuthProfileSnapshot(getAuthSnapshotTarget());
    }
	return null;
  };

  const applyAuthSnapshotResult = (snapshotResult) => {
    if (!snapshotResult) {
      return;
    }
    if (typeof snapshotResult.then === 'function') {
      snapshotResult
        .then((snapshot) => {
          if (!hasResolved) {
            handleAuthSnapshot(snapshot);
          }
        })
        .catch(() => {
          if (!hasResolved) {
            setStatus('error');
            clearStatusTimer();
          }
        });
      return;
    }
    handleAuthSnapshot(snapshotResult);
  };

  function start(callbacks = {}) {
    lastCallbacks = callbacks;
    if (hasResolved) {
      const store = window.Alpine.store('auth');
      if (store && store.profile) {
        invokeCallback('onAuthenticated', store.profile);
      } else {
        invokeCallback('onUnauthenticated');
      }
      setStatus('ready');
      return;
    }
    setStatus('hydrating');
    startStatusTimer();
    applyAuthSnapshotResult(readAuthSnapshot());
  }

  function fail() {
    hasResolved = true;
    clearUnauthenticatedSettleTimer();
    clearStatusTimer();
    applyProfile(null);
    setStatus('error');
  }

  function onStatusChange(listener) {
    statusListeners.add(listener);
    return () => statusListeners.delete(listener);
  }

  return { start, onStatusChange, fail };
}
