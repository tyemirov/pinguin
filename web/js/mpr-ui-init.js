// @ts-check
(function registerMprUiInit() {
  if (typeof window === 'undefined' || typeof document === 'undefined') {
    return;
  }

  /**
   * @typedef {Object} MprUiAuthConfig
   * @property {string} googleSiteId
   * @property {string} tauthTenantId
   * @property {string} tauthUrl
   * @property {string} tauthLoginPath
   * @property {string} tauthLogoutPath
   * @property {string} tauthNoncePath
   */

  /**
   * @typedef {Object} MprUiHeaderInit
   * @property {string=} brandLabel
   * @property {string=} brandHref
   * @property {Array<{ label: string, href: string }>|string=} navLinks
   * @property {Object|string=} themeConfig
   * @property {boolean=} settings
   * @property {string=} settingsLabel
   * @property {string=} signInLabel
   * @property {string=} signOutLabel
   * @property {boolean=} sticky
   * @property {string=} size
   */

  /**
   * @typedef {Object} MprUiLoginButtonInit
   * @property {string=} buttonText
   * @property {string=} buttonSize
   * @property {string=} buttonTheme
   * @property {string=} buttonShape
   */

  /**
   * @typedef {Object} MprUiInitConfig
   * @property {MprUiAuthConfig} auth
   * @property {MprUiHeaderInit=} header
   * @property {MprUiLoginButtonInit=} loginButton
   */

  const requireString = (value, code) => {
    const normalized = typeof value === 'string' ? value.trim() : '';
    if (!normalized) {
      throw new Error(code);
    }
    return normalized;
  };

  const setAttribute = (element, name, value) => {
    if (!element || typeof element.setAttribute !== 'function') {
      return;
    }
    if (value === undefined || value === null) {
      return;
    }
    if (typeof value === 'boolean') {
      element.setAttribute(name, value ? 'true' : 'false');
      return;
    }
    element.setAttribute(name, String(value));
  };

  const setJsonAttribute = (element, name, value) => {
    if (value === undefined || value === null) {
      return;
    }
    const payload = typeof value === 'string' ? value : JSON.stringify(value);
    setAttribute(element, name, payload);
  };

  const normalizeAuthConfig = (config) => {
    if (!config || typeof config !== 'object') {
      throw new Error('mpr-ui.init.auth_missing');
    }
    return {
      googleSiteId: requireString(
        config.googleSiteId,
        'mpr-ui.init.google_site_id_missing',
      ),
      tauthTenantId: requireString(
        config.tauthTenantId,
        'mpr-ui.init.tauth_tenant_id_missing',
      ),
      tauthUrl: requireString(config.tauthUrl, 'mpr-ui.init.tauth_url_missing'),
      tauthLoginPath: requireString(
        config.tauthLoginPath,
        'mpr-ui.init.tauth_login_path_missing',
      ),
      tauthLogoutPath: requireString(
        config.tauthLogoutPath,
        'mpr-ui.init.tauth_logout_path_missing',
      ),
      tauthNoncePath: requireString(
        config.tauthNoncePath,
        'mpr-ui.init.tauth_nonce_path_missing',
      ),
    };
  };

  const applyAuthToHeader = (header, auth) => {
    setAttribute(header, 'google-site-id', auth.googleSiteId);
    setAttribute(header, 'tauth-tenant-id', auth.tauthTenantId);
    setAttribute(header, 'tauth-url', auth.tauthUrl);
    setAttribute(header, 'tauth-login-path', auth.tauthLoginPath);
    setAttribute(header, 'tauth-logout-path', auth.tauthLogoutPath);
    setAttribute(header, 'tauth-nonce-path', auth.tauthNoncePath);
  };

  const applyHeaderConfig = (header, config) => {
    if (!config || typeof config !== 'object') {
      return;
    }
    setAttribute(header, 'brand-label', config.brandLabel);
    setAttribute(header, 'brand-href', config.brandHref);
    setJsonAttribute(header, 'nav-links', config.navLinks);
    setJsonAttribute(header, 'theme-config', config.themeConfig);
    setAttribute(header, 'settings', config.settings);
    setAttribute(header, 'settings-label', config.settingsLabel);
    setAttribute(header, 'sign-in-label', config.signInLabel);
    setAttribute(header, 'sign-out-label', config.signOutLabel);
    setAttribute(header, 'sticky', config.sticky);
    setAttribute(header, 'size', config.size);
  };

  const applyAuthToLoginButton = (button, auth) => {
    setAttribute(button, 'site-id', auth.googleSiteId);
    setAttribute(button, 'tauth-tenant-id', auth.tauthTenantId);
    setAttribute(button, 'tauth-url', auth.tauthUrl);
    setAttribute(button, 'tauth-login-path', auth.tauthLoginPath);
    setAttribute(button, 'tauth-logout-path', auth.tauthLogoutPath);
    setAttribute(button, 'tauth-nonce-path', auth.tauthNoncePath);
  };

  const applyLoginButtonConfig = (button, config) => {
    if (!config || typeof config !== 'object') {
      return;
    }
    setAttribute(button, 'button-text', config.buttonText);
    setAttribute(button, 'button-size', config.buttonSize);
    setAttribute(button, 'button-theme', config.buttonTheme);
    setAttribute(button, 'button-shape', config.buttonShape);
  };

  /**
   * @param {MprUiInitConfig} config
   * @returns {{ applied: { headers: number, loginButtons: number } }}
   */
  const init = (config) => {
    if (!config || typeof config !== 'object') {
      throw new Error('mpr-ui.init.config_missing');
    }
    const auth = normalizeAuthConfig(config.auth);
    const headers = Array.from(document.querySelectorAll('mpr-header'));
    headers.forEach((header) => {
      applyAuthToHeader(header, auth);
      applyHeaderConfig(header, config.header);
    });
    const loginButtons = Array.from(document.querySelectorAll('mpr-login-button'));
    loginButtons.forEach((button) => {
      applyAuthToLoginButton(button, auth);
      applyLoginButtonConfig(button, config.loginButton);
    });
    return {
      applied: {
        headers: headers.length,
        loginButtons: loginButtons.length,
      },
    };
  };

  const namespace = window.MPRUI || {};
  namespace.init = init;
  window.MPRUI = namespace;

  const preloaded = window.__MPR_UI_INIT__;
  if (preloaded) {
    init(preloaded);
  }
  window.addEventListener('mpr-ui:init', (event) => {
    const payload = event?.detail;
    if (payload) {
      init(payload);
    }
  });
})();
