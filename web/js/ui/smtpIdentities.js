// @ts-check
import { dispatchToast } from '../core/events.js';
import { createSMTPCredentialsDialog } from './smtpCredentialsDialog.js';
import { createSMTPDomains } from './smtpDomains.js';

/** @typedef {import('../types.d.js').SMTPIdentity} SMTPIdentity */

/**
 * Creates the canonical SMTP relay workspace.
 *
 * @param {{
 *   apiClient: ReturnType<typeof import('../core/apiClient.js').createApiClient>,
 *   strings: typeof import('../constants.js').STRINGS.smtpIdentities,
 *   actions: typeof import('../constants.js').STRINGS.actions,
 * }} options
 */
export function createSMTPWorkspace(options) {
  const { apiClient, strings, actions } = options;
  const authStore = () => window.Alpine.store('auth');
  const domainState = createSMTPDomains({ apiClient, strings, authStore });
  const credentialsDialogState = createSMTPCredentialsDialog({ strings });

  return {
    strings,
    actions,
    ...domainState,
    ...credentialsDialogState,
    identities: /** @type {SMTPIdentity[]} */ ([]),
    editingIdentityId: '',
    identityLocalPart: '',
    selectedIdentityDomain: '',
    identityForwardToText: '',
    editingForwardToText: '',
    isLoading: false,
    isSubmitting: false,
    errorMessage: '',
    identityDialogTrigger: /** @type {HTMLElement | null} */ (null),
    forwardingEditTrigger: /** @type {HTMLElement | null} */ (null),
    pendingDeleteIdentity: /** @type {SMTPIdentity | null} */ (null),
    deleteDialogTrigger: /** @type {HTMLElement | null} */ (null),
    init() {
      this.refreshIfAuthenticated();
      this.$watch(
        () => authStore().isAuthenticated,
        (isAuthenticated) => {
          if (isAuthenticated) {
            this.loadWorkspace();
          } else {
            this.identities = [];
            this.domains = [];
            this.credentials = null;
            this.credentialNotice = null;
            this.cancelForwardingEdit();
          }
        },
      );
    },
    async refreshIfAuthenticated() {
      if (authStore().isAuthenticated) {
        await this.loadWorkspace();
      }
    },
    async loadWorkspace() {
      await Promise.all([this.loadDomains(), this.loadIdentities()]);
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
        this.errorMessage = strings.loadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoading = false;
      }
    },
    openIdentityDialog(event) {
      this.identityDialogTrigger =
        event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      this.identityLocalPart = '';
      this.selectedIdentityDomain = this.verifiedDomains()[0]?.domain || '';
      this.identityForwardToText = '';
      this.errorMessage = '';
      const dialog = this.$refs.identityDialog;
      if (dialog && typeof dialog.showModal === 'function') {
        dialog.showModal();
      }
    },
    closeIdentityDialog() {
      const dialog = this.$refs.identityDialog;
      if (dialog && typeof dialog.close === 'function') {
        dialog.close();
      }
    },
    handleIdentityDialogClosed() {
      const trigger = this.identityDialogTrigger;
      this.identityDialogTrigger = null;
      this.identityLocalPart = '';
      this.selectedIdentityDomain = '';
      this.identityForwardToText = '';
      this.$nextTick(() => {
        if (trigger && trigger.isConnected) {
          trigger.focus();
        }
      });
    },
    async createIdentity(event) {
      event?.preventDefault();
      const localPart = this.identityLocalPart.trim();
      const senderDomain = this.selectedIdentityDomain.trim();
      const forwardTo = this.parseForwardRecipients(this.identityForwardToText);
      const verifiedDomain = this.verifiedDomains().some(
        (domain) => domain.domain === senderDomain,
      );
      if (!localPart || localPart.includes('@') || !verifiedDomain || forwardTo.length === 0) {
        this.errorMessage = strings.createError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const emailAddress = `${localPart}@${senderDomain}`;
        const credentials = await apiClient.createSMTPIdentity(emailAddress, forwardTo);
        if (!credentials) {
          throw new Error('missing_credentials');
        }
        this.credentials = credentials;
        await this.loadIdentities();
        this.setCredentialNotice('success', strings.createSuccess);
        this.closeIdentityDialog();
        this.$nextTick(() => this.openCredentialsDialog());
      } catch (error) {
        this.errorMessage = strings.createError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    editForwarding(event, identity) {
      this.forwardingEditTrigger =
        event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      this.editingIdentityId = identity.id;
      this.editingForwardToText = (identity.forwardTo || []).join('\n');
      this.errorMessage = '';
    },
    cancelForwardingEdit() {
      const trigger = this.forwardingEditTrigger;
      this.forwardingEditTrigger = null;
      this.editingIdentityId = '';
      this.editingForwardToText = '';
      this.$nextTick(() => {
        if (trigger && trigger.isConnected) {
          trigger.focus();
        }
      });
    },
    async updateForwarding() {
      const forwardTo = this.parseForwardRecipients(this.editingForwardToText);
      if (forwardTo.length === 0) {
        this.errorMessage = strings.updateForwardingError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        await apiClient.updateSMTPIdentityForwarding(this.editingIdentityId, forwardTo);
        this.cancelForwardingEdit();
        await this.loadIdentities();
        dispatchToast({ variant: 'success', message: strings.updateForwardingSuccess });
      } catch (error) {
        this.errorMessage = strings.updateForwardingError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async viewCredentials(event, identity) {
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const credentials = await apiClient.getSMTPIdentityCredentials(identity.id);
        if (!credentials) {
          throw new Error('missing_credentials');
        }
        this.credentials = credentials;
        this.setCredentialNotice('success', strings.credentialsLoadSuccess);
        this.openCredentialsDialog(event?.currentTarget || null);
      } catch (error) {
        this.errorMessage = strings.credentialsLoadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async rotateIdentity(identity) {
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const credentials = await apiClient.rotateSMTPIdentity(identity.id);
        if (!credentials) {
          throw new Error('missing_credentials');
        }
        this.credentials = credentials;
        await this.loadIdentities();
        this.setCredentialNotice('success', strings.rotateSuccess);
      } catch (error) {
        this.errorMessage = strings.rotateError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async rotateCurrentCredentials() {
      if (this.credentials) {
        await this.rotateIdentity(this.credentials.identity);
      }
    },
    openDeleteDialog(event, identity) {
      this.pendingDeleteIdentity = identity;
      this.deleteDialogTrigger =
        event?.currentTarget instanceof HTMLElement ? event.currentTarget : null;
      const dialog = this.$refs.deleteDialog;
      if (dialog && typeof dialog.showModal === 'function') {
        dialog.showModal();
      }
    },
    closeDeleteDialog() {
      const dialog = this.$refs.deleteDialog;
      if (dialog && typeof dialog.close === 'function') {
        dialog.close();
      }
    },
    handleDeleteDialogClosed() {
      const trigger = this.deleteDialogTrigger;
      this.pendingDeleteIdentity = null;
      this.deleteDialogTrigger = null;
      this.$nextTick(() => {
        const focusTarget = trigger && trigger.isConnected ? trigger : this.$refs.identityList;
        if (focusTarget instanceof HTMLElement) {
          focusTarget.focus();
        }
      });
    },
    async confirmDeleteIdentity() {
      if (this.isSubmitting) {
        return;
      }
      const identity = this.pendingDeleteIdentity;
      if (!identity) {
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        await apiClient.deleteSMTPIdentity(identity.id);
        await this.loadIdentities();
        dispatchToast({ variant: 'success', message: strings.deleteSuccess });
        this.closeDeleteDialog();
      } catch (error) {
        this.errorMessage = strings.deleteError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    /** @param {string} forwardToText */
    parseForwardRecipients(forwardToText) {
      return forwardToText
        .split(/[\n,;]/)
        .map((value) => value.trim())
        .filter(Boolean);
    },
    credentialsRecordLabel(identity) {
      return `${strings.openCredentialsLabel} ${identity.emailAddress}`;
    },
    formatTimestamp(isoString) {
      if (!isoString) {
        return strings.neverUsed;
      }
      const date = new Date(isoString);
      if (Number.isNaN(date.getTime())) {
        return strings.neverUsed;
      }
      return date.toLocaleString();
    },
  };
}
