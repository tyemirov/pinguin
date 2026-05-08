// @ts-check
import { dispatchToast } from '../core/events.js';

/** @typedef {import('../types.d.js').SMTPIdentity} SMTPIdentity */
/** @typedef {import('../types.d.js').SMTPCredentials} SMTPCredentials */

/**
 * @param {{
 *   apiClient: ReturnType<typeof import('../core/apiClient.js').createApiClient>,
 *   strings: typeof import('../constants.js').STRINGS.smtpIdentities,
 *   actions: typeof import('../constants.js').STRINGS.actions,
 * }} options
 */
export function createSMTPIdentities(options) {
  const { apiClient, strings, actions } = options;
  const authStore = () => window.Alpine.store('auth');

  return {
    strings,
    actions,
    identities: /** @type {SMTPIdentity[]} */ ([]),
    credentials: /** @type {SMTPCredentials | null} */ (null),
    credentialNotice: /** @type {{ variant: string, message: string } | null} */ (null),
    editingIdentityId: '',
    emailAddress: '',
    forwardToText: '',
    isLoading: false,
    isSubmitting: false,
    errorMessage: '',
    init() {
      this.refreshIfAuthenticated();
      this.$watch(
        () => authStore().isAuthenticated,
        (isAuthenticated) => {
          if (isAuthenticated) {
            this.loadIdentities();
      } else {
        this.identities = [];
        this.credentials = null;
        this.credentialNotice = null;
        this.cancelForwardingEdit();
      }
        },
      );
    },
    async refreshIfAuthenticated() {
      if (authStore().isAuthenticated) {
        await this.loadIdentities();
      }
    },
    async loadIdentities() {
      if (!authStore().isAuthenticated) {
        return;
      }
      this.isLoading = true;
      this.errorMessage = '';
      try {
        this.identities = await apiClient.listSMTPIdentities();
      } catch (error) {
        this.errorMessage = this.strings.loadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoading = false;
      }
    },
    async createIdentity(event) {
      event?.preventDefault();
      if (this.editingIdentityId) {
        await this.updateForwarding();
        return;
      }
      const emailAddress = this.emailAddress.trim();
      const forwardTo = this.parseForwardRecipients();
      if (!emailAddress || forwardTo.length === 0) {
        this.errorMessage = this.strings.createError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const credentials = await apiClient.createSMTPIdentity(emailAddress, forwardTo);
        if (!credentials) {
          throw new Error('missing_credentials');
        }
        this.credentials = credentials;
        this.emailAddress = '';
        this.forwardToText = '';
        await this.loadIdentities();
        this.setCredentialNotice('success', this.strings.createSuccess);
        this.openCredentialsDialog();
      } catch (error) {
        this.errorMessage = this.strings.createError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    editForwarding(identity) {
      this.editingIdentityId = identity.id;
      this.emailAddress = identity.emailAddress;
      this.forwardToText = (identity.forwardTo || []).join('\n');
      this.errorMessage = '';
    },
    cancelForwardingEdit() {
      this.editingIdentityId = '';
      this.emailAddress = '';
      this.forwardToText = '';
    },
    async updateForwarding() {
      const forwardTo = this.parseForwardRecipients();
      if (forwardTo.length === 0) {
        this.errorMessage = this.strings.updateForwardingError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        await apiClient.updateSMTPIdentityForwarding(this.editingIdentityId, forwardTo);
        this.cancelForwardingEdit();
        await this.loadIdentities();
        dispatchToast({ variant: 'success', message: this.strings.updateForwardingSuccess });
      } catch (error) {
        this.errorMessage = this.strings.updateForwardingError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async rotateIdentity(identity) {
      if (!window.confirm(this.strings.rotateConfirm)) {
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const credentials = await apiClient.rotateSMTPIdentity(identity.id);
        if (!credentials) {
          throw new Error('missing_credentials');
        }
        this.credentials = credentials;
        await this.loadIdentities();
        this.setCredentialNotice('success', this.strings.rotateSuccess);
        this.openCredentialsDialog();
      } catch (error) {
        this.errorMessage = this.strings.rotateError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async deleteIdentity(identity) {
      if (!window.confirm(this.strings.deleteConfirm)) {
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        await apiClient.deleteSMTPIdentity(identity.id);
        await this.loadIdentities();
        dispatchToast({ variant: 'success', message: this.strings.deleteSuccess });
      } catch (error) {
        this.errorMessage = this.strings.deleteError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    parseForwardRecipients() {
      return this.forwardToText
        .split(/[\n,;]/)
        .map((value) => value.trim())
        .filter(Boolean);
    },
    async copyCredentialValue(value, successMessage) {
      try {
        if (!navigator.clipboard || typeof navigator.clipboard.writeText !== 'function') {
          throw new Error('clipboard_unavailable');
        }
        await navigator.clipboard.writeText(String(value ?? ''));
        this.setCredentialNotice('success', successMessage);
      } catch (error) {
        this.setCredentialNotice('error', this.strings.copyError);
      }
    },
    setCredentialNotice(variant, message) {
      this.credentialNotice = { variant, message };
    },
    openCredentialsDialog() {
      const dialog = this.$refs.credentialsDialog;
      if (dialog && typeof dialog.showModal === 'function') {
        dialog.showModal();
      }
    },
    closeCredentialsDialog() {
      const dialog = this.$refs.credentialsDialog;
      if (dialog && typeof dialog.close === 'function') {
        dialog.close();
      }
      this.credentialNotice = null;
    },
    formatTimestamp(isoString) {
      if (!isoString) {
        return this.strings.neverUsed;
      }
      const date = new Date(isoString);
      if (Number.isNaN(date.getTime())) {
        return this.strings.neverUsed;
      }
      return date.toLocaleString();
    },
  };
}
