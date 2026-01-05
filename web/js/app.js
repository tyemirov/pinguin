// @ts-check
import Alpine from 'https://cdn.jsdelivr.net/npm/alpinejs@3.13.5/dist/module.esm.js';
import { RUNTIME_CONFIG, STRINGS } from './constants.js';
import { createApiClient } from './core/apiClient.js';
import { createNotificationsTable } from './ui/notificationsTable.js';
import { dispatchRefresh } from './core/events.js';
import { createToastCenter } from './ui/toastCenter.js';

window.Alpine = Alpine;

const apiClient = createApiClient(RUNTIME_CONFIG.apiBaseUrl);
const sessionBridge = createSessionBridge();

Alpine.store('auth', createAuthStore());

Alpine.data('landingAuthPanel', () => createLandingAuthPanel(sessionBridge));
Alpine.data('appShell', () => createAppShell(sessionBridge));
Alpine.data('notificationsTable', () =>
  createNotificationsTable({
    apiClient,
    strings: STRINGS.dashboard,
    actions: STRINGS.actions,
  }),
);
Alpine.data('toastCenter', () => createToastCenter());

registerThemePersistence();
Alpine.start();

function startApp() {
  bootstrapPage(sessionBridge);
}

if (document.readyState === 'loading') {
  document.addEventListener('DOMContentLoaded', startApp);
} else {
  startApp();
}

function createAuthStore() {
  return {
    profile: null,
    isAuthenticated: false,
    setProfile(profile) {
      this.profile = profile;
      this.isAuthenticated = Boolean(profile);
    },
    clear() {
      this.profile = null;
      this.isAuthenticated = false;
    },
  };
}

function registerThemePersistence() {
  if (typeof document === 'undefined') {
    return;
  }
  const THEME_STORAGE_KEY = 'pinguin.theme';
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
}

function createLandingAuthPanel(controller) {
  return {
    STRINGS,
    notice: STRINGS.auth.signingIn,
    stopStatusWatcher: null,
    init() {
      this.stopStatusWatcher = controller.onStatusChange((status) => {
        switch (status) {
          case 'hydrating':
            this.notice = STRINGS.auth.signingIn;
            break;
          case 'ready':
            if (!window.Alpine.store('auth').isAuthenticated) {
              this.notice = STRINGS.auth.ready;
            } else {
              this.notice = '';
            }
            break;
          case 'error':
            this.notice = STRINGS.auth.failed;
            break;
          default:
            break;
        }
      });
    },
    $cleanup() {
      if (typeof this.stopStatusWatcher === 'function') {
        this.stopStatusWatcher();
      }
    },
  };
}

function createAppShell(bridge) {
  return {
    strings: STRINGS.dashboard,
    actions: STRINGS.actions,
    stopAuthWatcher: null,
    stopStatusWatcher: null,
    hasHydrated: false,
    hasRedirected: false,
    previousAuthState: false,
    init() {
      const authStore = window.Alpine.store('auth');
      const pageId = document.body.dataset.page || 'landing';
      this.previousAuthState = authStore.isAuthenticated;
      this.hasHydrated = false;
      this.hasRedirected = false;
      this.stopAuthWatcher = this.$watch(
        () => authStore.isAuthenticated,
        (isAuthenticated) => {
          const shouldRedirect =
            !isAuthenticated && (this.previousAuthState || this.hasHydrated) && pageId === 'dashboard';
          this.previousAuthState = isAuthenticated;
          if (shouldRedirect) {
            this.redirectToLanding();
          }
        },
      );
      this.stopStatusWatcher = bridge.onStatusChange((status) => {
        if (status === 'ready' || status === 'error') {
          this.hasHydrated = true;
          if (!authStore.isAuthenticated && pageId === 'dashboard') {
            this.redirectToLanding();
          }
        }
      });
    },
    refreshNotifications() {
      dispatchRefresh();
    },
    async handleLogout() {
      await bridge.logout();
      const pageId = document.body.dataset.page || 'landing';
      if (pageId === 'dashboard') {
        this.redirectToLanding();
      } else {
        window.location.reload();
      }
    },
    getAvatarUrl() {
      const profile = Alpine.store('auth').profile;
      if (!profile || typeof profile !== 'object') {
        return null;
      }
      const url =
        (typeof profile.avatar_url === 'string' && profile.avatar_url.trim()) ||
        (typeof profile.user_avatar_url === 'string' &&
          profile.user_avatar_url.trim());
      return url || null;
    },
    profileMenuStyle() {
      const avatarUrl = this.getAvatarUrl();
      if (!avatarUrl) {
        return {};
      }
      return { '--profile-avatar-url': `url("${avatarUrl}")` };
    },
    redirectToLanding() {
      if (this.hasRedirected) {
        return;
      }
      this.hasRedirected = true;
      window.location.assign(RUNTIME_CONFIG.landingUrl);
    },
    $cleanup() {
      if (typeof this.stopAuthWatcher === 'function') {
        this.stopAuthWatcher();
      }
      if (typeof this.stopStatusWatcher === 'function') {
        this.stopStatusWatcher();
      }
    },
  };
}

function bootstrapPage(controller) {
  const pageId = document.body.dataset.page || 'landing';
  let redirected = false;
  let started = false;

  const handleAuthenticated = (profile) => {
    const store = Alpine.store('auth');
    store.setProfile(profile);
    if (pageId === 'landing' && !redirected) {
      redirected = true;
      window.location.assign(RUNTIME_CONFIG.dashboardUrl);
    }
  };

  const handleUnauthenticated = () => {
    const store = Alpine.store('auth');
    store.clear();
    if (pageId === 'dashboard' && !redirected) {
      redirected = true;
      window.location.assign(RUNTIME_CONFIG.landingUrl);
    }
  };

  const startSession = () => {
    if (started) {
      return;
    }
    started = true;
    controller.start({
      onAuthenticated: handleAuthenticated,
      onUnauthenticated: handleUnauthenticated,
    });
  };

  const handleAuthError = () => {
    if (started) {
      return;
    }
    started = true;
    controller.fail();
    handleUnauthenticated();
  };

  if (window.__PINGUIN_AUTH_READY__) {
    startSession();
    return;
  }
  window.addEventListener('pinguin:auth-ready', startSession, { once: true });
  window.addEventListener('pinguin:auth-error', handleAuthError, { once: true });
}

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

function createSessionBridge() {
  let lastCallbacks = { onAuthenticated: undefined, onUnauthenticated: undefined };
  const statusListeners = new Set();
  let statusTimer = null;
  let hasResolved = false;

  const applyProfile = (profile) => {
    const store = Alpine.store('auth');
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

  const startStatusTimer = () => {
    clearStatusTimer();
    statusTimer = setTimeout(() => {
      if (!hasResolved) {
        setStatus('error');
      }
    }, 12000);
  };

  const sessionChannel =
    typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('auth') : null;
  if (sessionChannel) {
    sessionChannel.addEventListener('message', (event) => {
      if (event.data === 'logged_out') {
        applyProfile(null);
        invokeCallback('onUnauthenticated');
      }
      if (event.data === 'refreshed') {
        if (typeof window.getCurrentUser === 'function') {
          const refreshedProfile = window.getCurrentUser();
          if (refreshedProfile) {
            applyProfile(refreshedProfile);
            invokeCallback('onAuthenticated', refreshedProfile);
          }
        }
      }
    });
  }

  const handleHeaderAuthenticated = (event) => {
    const profile = event?.detail?.profile || null;
    hasResolved = true;
    clearStatusTimer();
    applyProfile(profile);
    setStatus('ready');
    invokeCallback('onAuthenticated', profile);
  };

  const handleHeaderUnauthenticated = () => {
    hasResolved = true;
    clearStatusTimer();
    applyProfile(null);
    setStatus('ready');
    invokeCallback('onUnauthenticated');
  };

  if (typeof document !== 'undefined') {
    document.addEventListener('mpr-ui:auth:authenticated', handleHeaderAuthenticated);
    document.addEventListener('mpr-ui:auth:unauthenticated', handleHeaderUnauthenticated);
    document.addEventListener('mpr-ui:auth:error', () => {
      if (!hasResolved) {
        setStatus('error');
        clearStatusTimer();
      }
    });
  }

  function start(callbacks = {}) {
    lastCallbacks = callbacks;
    if (hasResolved) {
      const store = Alpine.store('auth');
      if (store && store.profile) {
        invokeCallback('onAuthenticated', store.profile);
      } else {
        invokeCallback('onUnauthenticated');
      }
      setStatus('ready');
      return;
    }
    const cachedState =
      typeof window !== 'undefined' ? window.__PINGUIN_AUTH_STATE__ : null;
    if (cachedState && typeof cachedState === 'object') {
      if (cachedState.status === 'authenticated') {
        handleHeaderAuthenticated({ detail: { profile: cachedState.profile } });
        return;
      }
      if (cachedState.status === 'unauthenticated') {
        handleHeaderUnauthenticated();
        return;
      }
    }
    setStatus('hydrating');
    startStatusTimer();
    if (typeof window.getCurrentUser !== 'function') {
      fail();
      throw new Error('auth.helper.missing');
    }
    const seededProfile = window.getCurrentUser();
    if (seededProfile) {
      handleHeaderAuthenticated({ detail: { profile: seededProfile } });
    }
  }

  async function logout() {
    if (typeof window.logout === 'function') {
      await window.logout();
    }
    applyProfile(null);
  }

  function fail() {
    hasResolved = true;
    clearStatusTimer();
    applyProfile(null);
    setStatus('error');
  }

  function onStatusChange(listener) {
    statusListeners.add(listener);
    return () => statusListeners.delete(listener);
  }

  return { start, logout, onStatusChange, fail };
}
