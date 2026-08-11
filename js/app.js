// @ts-check
import Alpine from 'https://cdn.jsdelivr.net/npm/alpinejs@3.13.5/dist/module.esm.js';
import { RUNTIME_CONFIG, STRINGS } from './constants.js';
import { createApiClient } from './core/apiClient.js';
import { createSessionBridge } from './core/sessionBridge.js';
import { createNotificationsList } from './ui/notificationsList.js';
import { createSMTPWorkspace } from './ui/smtpIdentities.js';
import { dispatchRefresh } from './core/events.js';
import { createToastCenter } from './ui/toastCenter.js';

const PROTECTED_PAGE_IDS = new Set(['event-log', 'smtp-relay']);

window.Alpine = Alpine;
Alpine.store('auth', createAuthStore());

const apiClient = createApiClient(RUNTIME_CONFIG.apiBaseUrl);
const sessionBridge = createSessionBridge();

Alpine.data('landingAuthPanel', () => createLandingAuthPanel(sessionBridge));
Alpine.data('appShell', () => createAppShell(sessionBridge));
Alpine.data('notificationsList', () =>
  createNotificationsList({
    apiClient,
    strings: STRINGS.eventLog,
    actions: STRINGS.actions,
  }),
);
Alpine.data('smtpWorkspace', () =>
  createSMTPWorkspace({
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
    STRINGS,
    RUNTIME_CONFIG,
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
