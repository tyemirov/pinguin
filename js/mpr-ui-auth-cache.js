// @ts-check
(function registerMprUiAuthCache() {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return;
  }

  const AUTH_STATE_KEY = '__PINGUIN_AUTH_STATE__';
  const AUTH_LISTENER_KEY = '__PINGUIN_AUTH_LISTENING__';
  const AUTH_BRIDGE_KEY = '__PINGUIN_AUTH_BRIDGE__';
  const PINGUIN_AUTH_STATE_EVENT = 'pinguin:auth-state';
  const DEFAULT_TAUTH_PROFILE_PATH = '/me';
  const DEFAULT_TAUTH_REFRESH_PATH = '/auth/refresh';
  const REQUESTED_WITH_HEADER = 'XMLHttpRequest';
  const HEADER_SELECTOR = 'mpr-header';
  const HTTP_STATUS_NO_CONTENT = 204;
  const HTTP_STATUS_UNAUTHORIZED = 401;
  const HTTP_STATUS_NOT_FOUND = 404;
  const HTTP_STATUS_METHOD_NOT_ALLOWED = 405;
  const HTTP_STATUS_NOT_IMPLEMENTED = 501;

  /**
   * @typedef {{
   *   tenantId: string,
   *   profile: Record<string, unknown> | null,
   *   status: 'unknown' | 'authenticated' | 'unauthenticated',
   *   profilePromise: Promise<Record<string, unknown> | null> | null,
   * }} AuthBridgeState
   */

  /**
   * @returns {AuthBridgeState}
   */
  const ensureBridgeState = () => {
    const globalWindow = /** @type {Window & Record<string, unknown>} */ (window);
    if (!globalWindow[AUTH_BRIDGE_KEY]) {
      globalWindow[AUTH_BRIDGE_KEY] = {
        tenantId: '',
        profile: null,
        status: 'unknown',
        profilePromise: null,
      };
    }
    return /** @type {AuthBridgeState} */ (globalWindow[AUTH_BRIDGE_KEY]);
  };

  if (window[AUTH_LISTENER_KEY]) {
    return;
  }

  const bridgeState = ensureBridgeState();

  /**
   * @param {unknown} value
   * @returns {string}
   */
  const normalizeString = (value) => (typeof value === 'string' ? value.trim() : '');

  /**
   * @param {string} baseUrl
   * @param {string} path
   * @returns {string}
   */
  const joinUrl = (baseUrl, path) => {
    const normalizedBase = baseUrl.replace(/\/+$/, '');
    const normalizedPath = path.startsWith('/') ? path : `/${path}`;
    return `${normalizedBase}${normalizedPath}`;
  };

  /**
   * @returns {Element | null}
   */
  const resolveHeaderElement = () => document.querySelector(HEADER_SELECTOR);

  /**
   * @param {Element | null} header
   * @param {string} attributeName
   * @returns {string}
   */
  const readHeaderAttribute = (header, attributeName) =>
    header && typeof header.getAttribute === 'function'
      ? normalizeString(header.getAttribute(attributeName))
      : '';

  /**
   * @returns {string}
   */
  const resolveTauthBaseUrl = () => {
    const header = resolveHeaderElement();
    const headerUrl = readHeaderAttribute(header, 'tauth-url');
    if (headerUrl) {
      return headerUrl;
    }
    const pinguinConfig =
      /** @type {{ tauthBaseUrl?: unknown } | null} */ (window.__PINGUIN_CONFIG__ || null);
    const configuredUrl = normalizeString(pinguinConfig?.tauthBaseUrl);
    if (configuredUrl) {
      return configuredUrl;
    }
    return window.location.origin;
  };

  /**
   * @returns {string}
   */
  const resolveTenantId = () => {
    if (bridgeState.tenantId) {
      return bridgeState.tenantId;
    }
    const header = resolveHeaderElement();
    const headerTenantId = readHeaderAttribute(header, 'tauth-tenant-id');
    if (headerTenantId) {
      return headerTenantId;
    }
    const pinguinConfig =
      /** @type {{ tauthTenantId?: unknown } | null} */ (window.__PINGUIN_CONFIG__ || null);
    return normalizeString(pinguinConfig?.tauthTenantId);
  };

  /**
   * @returns {Record<string, string>}
   */
  const buildAuthHeaders = () => {
    const headers = {
      'X-Requested-With': REQUESTED_WITH_HEADER,
    };
    const tenantId = resolveTenantId();
    if (tenantId) {
      headers['X-TAuth-Tenant'] = tenantId;
    }
    return headers;
  };

  /**
   * @param {Record<string, unknown> | null} profile
   */
  const recordAuthState = (status, profile) => {
    window[AUTH_STATE_KEY] = { status, profile: profile || null };
  };

  /**
   * @param {Record<string, unknown> | null} profile
   */
  const recordAuthenticatedProfile = (profile) => {
    bridgeState.profile = profile;
    bridgeState.status = profile ? 'authenticated' : 'unauthenticated';
    bridgeState.profilePromise = null;
    recordAuthState(bridgeState.status, profile);
    dispatchPinguinAuthState();
  };

  const markAuthUnknown = () => {
    bridgeState.profile = null;
    bridgeState.status = 'unknown';
    bridgeState.profilePromise = null;
  };

  const dispatchPinguinAuthState = () => {
    document.dispatchEvent(
      new CustomEvent(PINGUIN_AUTH_STATE_EVENT, {
        detail: {
          status: bridgeState.status,
          profile: bridgeState.profile,
        },
      }),
    );
  };

  /**
   * @param {Response} response
   * @param {boolean} allowFallback
   * @returns {Promise<boolean>}
   */
  const handleRefreshResponse = async (response, allowFallback) => {
    if (response.ok) {
      return true;
    }
    if (
      allowFallback &&
      (response.status === HTTP_STATUS_NOT_FOUND ||
        response.status === HTTP_STATUS_METHOD_NOT_ALLOWED ||
        response.status === HTTP_STATUS_NOT_IMPLEMENTED)
    ) {
      const fallbackResponse = await fetch(
        joinUrl(resolveTauthBaseUrl(), DEFAULT_TAUTH_REFRESH_PATH),
        {
          method: 'GET',
          credentials: 'include',
          headers: buildAuthHeaders(),
        },
      );
      return handleRefreshResponse(fallbackResponse, false);
    }
    if (response.status === HTTP_STATUS_UNAUTHORIZED) {
      return false;
    }
    throw new Error(`auth_session_refresh_failed:${response.status}`);
  };

  /**
   * @returns {Promise<boolean>}
   */
  const refreshSession = async () => {
    const response = await fetch(joinUrl(resolveTauthBaseUrl(), DEFAULT_TAUTH_REFRESH_PATH), {
      method: 'POST',
      credentials: 'include',
      headers: buildAuthHeaders(),
    });
    return handleRefreshResponse(response, true);
  };

  /**
   * @param {boolean} allowRefresh
   * @returns {Promise<Record<string, unknown> | null>}
   */
  const requestCurrentProfile = async (allowRefresh) => {
    const response = await fetch(joinUrl(resolveTauthBaseUrl(), DEFAULT_TAUTH_PROFILE_PATH), {
      method: 'GET',
      credentials: 'include',
      headers: buildAuthHeaders(),
    });
    if (response.status === HTTP_STATUS_NO_CONTENT) {
      return null;
    }
    if (response.status === HTTP_STATUS_UNAUTHORIZED) {
      if (!allowRefresh) {
        return null;
      }
      const refreshed = await refreshSession();
      return refreshed ? requestCurrentProfile(false) : null;
    }
    if (!response.ok) {
      throw new Error(`auth_profile_request_failed:${response.status}`);
    }
    const payload = await response.json();
    return payload && typeof payload === 'object'
      ? /** @type {Record<string, unknown>} */ (payload)
      : null;
  };

  /**
   * @returns {Promise<Record<string, unknown> | null> | Record<string, unknown> | null}
   */
  const getCurrentUser = () => {
    if (bridgeState.status === 'authenticated') {
      return bridgeState.profile;
    }
    if (bridgeState.status === 'unauthenticated') {
      return null;
    }
    if (bridgeState.profilePromise) {
      return bridgeState.profilePromise;
    }
    bridgeState.profilePromise = requestCurrentProfile(true)
      .then((profile) => {
        recordAuthenticatedProfile(profile);
        return profile;
      })
      .catch((error) => {
        markAuthUnknown();
        throw error;
      });
    return bridgeState.profilePromise;
  };

  /**
   * @param {unknown} tenantId
   */
  const setAuthTenantId = (tenantId) => {
    const normalizedTenantId = normalizeString(tenantId);
    if (!normalizedTenantId) {
      return;
    }
    if (bridgeState.tenantId === normalizedTenantId) {
      return;
    }
    bridgeState.tenantId = normalizedTenantId;
    markAuthUnknown();
  };

  if (typeof window.getCurrentUser !== 'function') {
    window.getCurrentUser = getCurrentUser;
  }

  if (typeof window.setAuthTenantId !== 'function') {
    window.setAuthTenantId = setAuthTenantId;
  }

  window[AUTH_LISTENER_KEY] = true;
  document.addEventListener('mpr-ui:auth:authenticated', (event) => {
    const profile = event?.detail?.profile || null;
    recordAuthenticatedProfile(profile);
  });
  document.addEventListener('mpr-ui:auth:unauthenticated', () => {
    recordAuthenticatedProfile(null);
  });
  document.addEventListener('mpr-ui:auth:status-change', (event) => {
    if (event?.detail?.status === 'unauthenticated') {
      recordAuthenticatedProfile(null);
    }
  });
  if (typeof BroadcastChannel !== 'undefined') {
    const sessionChannel = new BroadcastChannel('auth');
    sessionChannel.addEventListener('message', (event) => {
      if (event.data === 'refreshed') {
        markAuthUnknown();
      }
      if (event.data === 'logged_out') {
        recordAuthenticatedProfile(null);
      }
    });
  }
})();
