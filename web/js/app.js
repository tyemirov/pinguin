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
Alpine.data('dashboardShell', () => createDashboardShell(sessionBridge));
Alpine.data('notificationsTable', () =>
  createNotificationsTable({
    apiClient,
    strings: STRINGS.dashboard,
    actions: STRINGS.actions,
  }),
);
Alpine.data('toastCenter', () => createToastCenter());

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

function createDashboardShell(bridge) {
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
      this.previousAuthState = authStore.isAuthenticated;
      this.hasHydrated = false;
      this.hasRedirected = false;
      this.stopAuthWatcher = this.$watch(
        () => authStore.isAuthenticated,
        (isAuthenticated) => {
          const shouldRedirect =
            !isAuthenticated && (this.previousAuthState || this.hasHydrated);
          this.previousAuthState = isAuthenticated;
          if (shouldRedirect) {
            this.redirectToLanding();
          }
        },
      );
      this.stopStatusWatcher = bridge.onStatusChange((status) => {
        if (status === 'ready' || status === 'error') {
          this.hasHydrated = true;
          if (!authStore.isAuthenticated) {
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
      this.redirectToLanding();
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
  controller.start({
    onAuthenticated(profile) {
      const store = Alpine.store('auth');
      store.setProfile(profile);
      if (pageId === 'landing' && !redirected) {
        redirected = true;
        window.location.assign(RUNTIME_CONFIG.dashboardUrl);
      }
    },
    onUnauthenticated() {
      const store = Alpine.store('auth');
      store.clear();
      if (pageId === 'dashboard' && !redirected) {
        redirected = true;
        window.location.assign(RUNTIME_CONFIG.landingUrl);
      }
    },
  });
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

function readProfileFromHeader() {
  if (typeof document === 'undefined') {
    return null;
  }
  const header = document.querySelector('mpr-header');
  if (!header || typeof header.getAttribute !== 'function') {
    return null;
  }
  const email = header.getAttribute('data-user-email') || '';
  const display = header.getAttribute('data-user-display') || '';
  const avatarUrl = header.getAttribute('data-user-avatar-url') || '';
  if (!email && !display && !avatarUrl) {
    return null;
  }
  return normalizeProfile({
    user_email: email,
    display,
    avatar_url: avatarUrl,
  });
}

function createSessionBridge() {
  let lastCallbacks = { onAuthenticated: undefined, onUnauthenticated: undefined };
  const statusListeners = new Set();
  let statusTimer = null;
  let bootstrapFallbackTimer = null;
  let bootstrapDeadline = 0;
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

  const clearBootstrapFallback = () => {
    if (bootstrapFallbackTimer) {
      clearTimeout(bootstrapFallbackTimer);
      bootstrapFallbackTimer = null;
    }
  };

  const scheduleBootstrapFallback = () => {
    clearBootstrapFallback();
    bootstrapFallbackTimer = setTimeout(() => {
      if (hasResolved) {
        return;
      }
      const helperReady = typeof window.getCurrentUser === 'function';
      const refreshedProfile = helperReady ? window.getCurrentUser() : null;
      const headerProfile = readProfileFromHeader();
      const resolvedProfile = refreshedProfile || headerProfile;
      if (resolvedProfile) {
        handleHeaderAuthenticated({ detail: { profile: resolvedProfile } });
        return;
      }
      if (!helperReady && Date.now() < bootstrapDeadline) {
        scheduleBootstrapFallback();
        return;
      }
      if (helperReady && Date.now() < bootstrapDeadline) {
        scheduleBootstrapFallback();
        return;
      }
      handleHeaderUnauthenticated();
    }, 100);
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
    clearBootstrapFallback();
    applyProfile(profile);
    setStatus('ready');
    invokeCallback('onAuthenticated', profile);
  };

  const handleHeaderUnauthenticated = () => {
    hasResolved = true;
    clearStatusTimer();
    clearBootstrapFallback();
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
    setStatus('hydrating');
    startStatusTimer();
    const seededProfile =
      typeof window.getCurrentUser === 'function' ? window.getCurrentUser() : null;
    if (seededProfile) {
      handleHeaderAuthenticated({ detail: { profile: seededProfile } });
      return;
    }
    const initialProfile = readProfileFromHeader();
    if (initialProfile) {
      handleHeaderAuthenticated({ detail: { profile: initialProfile } });
      return;
    }
    bootstrapDeadline = Date.now() + 1500;
    scheduleBootstrapFallback();
  }

  async function logout() {
    if (typeof window.logout === 'function') {
      await window.logout();
    }
    applyProfile(null);
  }

  function onStatusChange(listener) {
    statusListeners.add(listener);
    return () => statusListeners.delete(listener);
  }

  return { start, logout, onStatusChange };
}
