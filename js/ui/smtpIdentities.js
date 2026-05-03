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
    emailAddress: '',
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
      const emailAddress = this.emailAddress.trim();
      if (!emailAddress) {
        this.errorMessage = this.strings.createError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const credentials = await apiClient.createSMTPIdentity(emailAddress);
        if (!credentials) {
          throw new Error('missing_credentials');
        }
        this.credentials = credentials;
        this.emailAddress = '';
        await this.loadIdentities();
        dispatchToast({ variant: 'success', message: this.strings.createSuccess });
        this.openCredentialsDialog();
      } catch (error) {
        this.errorMessage = this.strings.createError;
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
        dispatchToast({ variant: 'success', message: this.strings.rotateSuccess });
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
