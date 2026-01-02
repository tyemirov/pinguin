// @ts-check
import Alpine from 'https://cdn.jsdelivr.net/npm/alpinejs@3.13.5/dist/module.esm.js';
import { RUNTIME_CONFIG, STRINGS } from './constants.js';
import { createApiClient } from './core/apiClient.js';
import { createNotificationsTable } from './ui/notificationsTable.js';
import { dispatchRefresh } from './core/events.js';
import { createToastCenter } from './ui/toastCenter.js';

window.Alpine = Alpine;

const apiClient = createApiClient(RUNTIME_CONFIG.apiBaseUrl);
const sessionBridge = createSessionBridge(RUNTIME_CONFIG);

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
  controller
    .hydrate({
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
    })
    .catch((error) => {
      console.error('auth bootstrap failed', error);
    });
}

function createSessionBridge(config) {
  let lastCallbacks = { onAuthenticated: undefined, onUnauthenticated: undefined };
  const statusListeners = new Set();

  const applyProfile = (profile) => {
    const store = Alpine.store('auth');
    if (profile) {
      store.setProfile(profile);
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

  const sessionChannel =
    typeof BroadcastChannel !== 'undefined' ? new BroadcastChannel('auth') : null;
  if (sessionChannel) {
    sessionChannel.addEventListener('message', (event) => {
      if (event.data === 'logged_out') {
        applyProfile(null);
        invokeCallback('onUnauthenticated');
      }
      if (event.data === 'refreshed') {
        hydrate(lastCallbacks).catch((error) => {
          console.error('hydrate after refresh failed', error);
        });
      }
    });
  }

  const handleHeaderAuthenticated = (event) => {
    const profile = event?.detail?.profile || null;
    setStatus('ready');
    applyProfile(profile);
    invokeCallback('onAuthenticated', profile);
  };

  const handleHeaderUnauthenticated = () => {
    setStatus('ready');
    applyProfile(null);
    invokeCallback('onUnauthenticated');
  };

  if (typeof document !== 'undefined') {
    document.addEventListener('mpr-ui:auth:authenticated', handleHeaderAuthenticated);
    document.addEventListener('mpr-ui:auth:unauthenticated', handleHeaderUnauthenticated);
  }

  async function hydrate(callbacks = {}) {
    lastCallbacks = callbacks;
    setStatus('hydrating');
    try {
      await waitFor(() => typeof window.initAuthClient === 'function');
      const tenantId =
        config.tenant && typeof config.tenant.id === 'string'
          ? config.tenant.id.trim()
          : '';
      if (tenantId && typeof window.setAuthTenantId === 'function') {
        window.setAuthTenantId(tenantId);
      }
      const result = await window.initAuthClient({
        baseUrl: config.tauthBaseUrl,
        tenantId: tenantId || undefined,
        onAuthenticated(profile) {
          applyProfile(profile);
          invokeCallback('onAuthenticated', profile);
        },
        onUnauthenticated() {
          applyProfile(null);
          invokeCallback('onUnauthenticated');
        },
      });
      setStatus('ready');
      return result;
    } catch (error) {
      setStatus('error');
      throw error;
    }
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

  return { hydrate, logout, onStatusChange };
}

function waitFor(checkFn, timeout = 12000) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const tick = () => {
      const result = checkFn();
      if (result) {
        resolve(result);
        return;
      }
      if (Date.now() - start > timeout) {
        reject(new Error('timeout'));
        return;
      }
      setTimeout(tick, 80);
    };
    tick();
  });
}
