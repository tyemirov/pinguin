// @ts-check
import Alpine from 'https://cdn.jsdelivr.net/npm/alpinejs@3.13.5/dist/module.esm.js';
import { RUNTIME_CONFIG, STRINGS } from './constants.js';
import { createApiClient } from './core/apiClient.js';
import { createNotificationsTable } from './ui/notificationsTable.js';
import { createSMTPIdentities } from './ui/smtpIdentities.js';
import { dispatchRefresh } from './core/events.js';
import { createToastCenter } from './ui/toastCenter.js';

const AUTH_STATUS_TIMEOUT_MS = 12000;
const AUTH_UNAUTHENTICATED_SETTLE_MS = 350;
const PROTECTED_PAGE_IDS = new Set(['event-log', 'smtp-relay']);

window.Alpine = Alpine;

const apiClient = createApiClient(RUNTIME_CONFIG.apiBaseUrl);
const sessionBridge = createSessionBridge();

Alpine.store('auth', createAuthStore());

Alpine.data('landingAuthPanel', () => createLandingAuthPanel(sessionBridge));
Alpine.data('appShell', () => createAppShell(sessionBridge));
Alpine.data('notificationsTable', () =>
  createNotificationsTable({
    apiClient,
    strings: STRINGS.eventLog,
    actions: STRINGS.actions,
  }),
);
Alpine.data('smtpIdentities', () =>
  createSMTPIdentities({
    apiClient,
    strings: STRINGS.smtpIdentities,
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

function createAppShell(bridge) {
  return {
    strings: STRINGS.eventLog,
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
            !isAuthenticated && (this.previousAuthState || this.hasHydrated) && isProtectedPage(pageId);
          this.previousAuthState = isAuthenticated;
          if (shouldRedirect) {
            this.redirectToLanding();
          }
        },
      );
      this.stopStatusWatcher = bridge.onStatusChange((status) => {
        if (status === 'ready' || status === 'error') {
          this.hasHydrated = true;
          if (!authStore.isAuthenticated && isProtectedPage(pageId)) {
            this.redirectToLanding();
          }
        }
      });
    },
    refreshNotifications() {
      dispatchRefresh();
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
      window.location.assign(RUNTIME_CONFIG.eventLogUrl);
    }
  };

  const handleUnauthenticated = () => {
    const store = Alpine.store('auth');
    store.clear();
    if (isProtectedPage(pageId) && !redirected) {
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

  waitForMprUiOrchestration().then(startSession).catch(handleAuthError);
}

function isProtectedPage(pageId) {
  return PROTECTED_PAGE_IDS.has(pageId);
}

function waitForMprUiOrchestration() {
  const namespace = window.MPRUI;
  if (namespace && typeof namespace.whenAutoOrchestrationReady === 'function') {
    return namespace.whenAutoOrchestrationReady();
  }
  if (document.readyState !== 'loading') {
    return Promise.resolve();
  }
  return new Promise((resolve) => {
    document.addEventListener('DOMContentLoaded', resolve, { once: true });
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

function createSessionBridge() {
  let lastCallbacks = { onAuthenticated: undefined, onUnauthenticated: undefined };
  const statusListeners = new Set();
  let statusTimer = null;
  let unauthenticatedSettleTimer = null;
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
      const snapshot = readSharedShellSnapshot();
      if (statusFromSnapshot(snapshot) === 'authenticated') {
        handleHeaderAuthenticated({ detail: { profile: profileFromSnapshot(snapshot) } });
        return;
      }
      resolveUnauthenticated();
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

  if (typeof document !== 'undefined') {
    document.addEventListener('mpr-ui:auth:authenticated', handleHeaderAuthenticated);
    document.addEventListener('mpr-ui:auth:unauthenticated', handleHeaderUnauthenticated);
    document.addEventListener('mpr-ui:auth:status-change', handleHeaderStatusChange);
    document.addEventListener('mpr-ui:auth:error', () => {
      if (!hasResolved) {
        setStatus('error');
        clearStatusTimer();
      }
    });
  }

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
      handleHeaderUnauthenticated();
      return true;
    }
    return false;
  };

  const readAuthSnapshot = () => {
    const namespace = window.MPRUI;
    if (namespace && typeof namespace.resolveAuthProfileSnapshot === 'function') {
      return namespace.resolveAuthProfileSnapshot(getAuthSnapshotTarget());
    }
    return readSharedShellSnapshot();
  };

  const readSharedShellSnapshot = () => {
    const header = document.querySelector('mpr-header');
    if (!header) {
      return null;
    }
    const userMenu = header.querySelector('[data-mpr-header="user-menu"]');
    const status = (
      userMenu?.getAttribute('data-mpr-user-status') ||
      header.getAttribute('data-mpr-auth-status') ||
      ''
    ).trim();
    if (status === 'unauthenticated') {
      return { status };
    }
    if (status !== 'authenticated') {
      return null;
    }
    return {
      status,
      profile: {
        user_email: header.getAttribute('data-user-email') || userMenu?.getAttribute('data-user-email') || '',
        user_display_name:
          header.getAttribute('data-user-display') || userMenu?.getAttribute('data-user-display') || '',
        user_avatar_url:
          header.getAttribute('data-user-avatar-url') ||
          userMenu?.getAttribute('data-user-avatar-url') ||
          '',
      },
    };
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
      const store = Alpine.store('auth');
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
