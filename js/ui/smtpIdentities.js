// @ts-check
import { dispatchToast } from '../core/events.js';

/** @typedef {import('../types.d.js').SMTPIdentity} SMTPIdentity */
/** @typedef {import('../types.d.js').SMTPCredentials} SMTPCredentials */
/** @typedef {import('../types.d.js').SMTPSenderDomain} SMTPSenderDomain */

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
    domains: /** @type {SMTPSenderDomain[]} */ ([]),
    credentials: /** @type {SMTPCredentials | null} */ (null),
    credentialNotice: /** @type {{ variant: string, message: string } | null} */ (null),
    editingIdentityId: '',
    domainName: '',
    emailAddress: '',
    forwardToText: '',
    isLoading: false,
    isLoadingDomains: false,
    isSubmitting: false,
    checkingDomainId: 0,
    errorMessage: '',
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
    async loadDomains() {
      if (!authStore().isAuthenticated) {
        return;
      }
      this.isLoadingDomains = true;
      this.errorMessage = '';
      try {
        this.domains = await apiClient.listSMTPDomains();
      } catch (error) {
        this.errorMessage = this.strings.domainLoadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoadingDomains = false;
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
    async createDomain(event) {
      event?.preventDefault();
      const domainName = this.domainName.trim();
      if (!domainName) {
        this.errorMessage = this.strings.domainCreateError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const domain = await apiClient.createSMTPDomain(domainName);
        if (!domain) {
          throw new Error('missing_domain');
        }
        this.upsertDomain(domain);
        this.domainName = '';
        dispatchToast({ variant: 'success', message: this.strings.domainCreateSuccess });
      } catch (error) {
        this.errorMessage = this.strings.domainCreateError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async checkDomain(domain) {
      this.checkingDomainId = domain.id;
      this.errorMessage = '';
      try {
        const checkedDomain = await apiClient.checkSMTPDomainDNS(domain.id);
        if (!checkedDomain) {
          throw new Error('missing_domain');
        }
        this.upsertDomain(checkedDomain);
        dispatchToast({ variant: 'success', message: this.strings.domainCheckSuccess });
      } catch (error) {
        this.errorMessage = this.strings.domainCheckError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.checkingDomainId = 0;
      }
    },
    upsertDomain(domain) {
      const index = this.domains.findIndex((candidate) => candidate.id === domain.id);
      if (index === -1) {
        this.domains = [...this.domains, domain].sort((left, right) => left.domain.localeCompare(right.domain));
        return;
      }
      this.domains = this.domains.map((candidate) => (candidate.id === domain.id ? domain : candidate));
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
      if (!this.isSenderDomainVerified()) {
        this.errorMessage = this.strings.domainRequiredNotice;
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
    async viewCredentials(identity) {
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const credentials = await apiClient.getSMTPIdentityCredentials(identity.id);
        if (!credentials) {
          throw new Error('missing_credentials');
        }
        this.credentials = credentials;
        this.setCredentialNotice('success', this.strings.credentialsLoadSuccess);
        this.openCredentialsDialog();
      } catch (error) {
        this.errorMessage = this.strings.credentialsLoadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    async rotateIdentity(identity, shouldConfirm = false) {
      if (shouldConfirm && !window.confirm(this.strings.rotateConfirm)) {
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
    async rotateCurrentCredentials() {
      if (this.credentials) {
        await this.rotateIdentity(this.credentials.identity, false);
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
    senderDomainFromEmail() {
      const normalizedEmail = this.emailAddress.trim().toLowerCase();
      const atIndex = normalizedEmail.lastIndexOf('@');
      if (atIndex === -1) {
        return '';
      }
      return normalizedEmail.slice(atIndex + 1).trim();
    },
    senderDomainRecord() {
      const domainName = this.senderDomainFromEmail();
      if (!domainName) {
        return null;
      }
      return this.domains.find((domain) => domain.domain === domainName) || null;
    },
    isSenderDomainVerified() {
      if (this.editingIdentityId) {
        return true;
      }
      const domain = this.senderDomainRecord();
      return Boolean(domain && domain.status === 'verified');
    },
    senderDomainNotice() {
      if (this.editingIdentityId || !this.emailAddress.trim()) {
        return '';
      }
      return this.isSenderDomainVerified() ? '' : this.strings.domainRequiredNotice;
    },
    domainStatusLabel(domain) {
      return domain.status === 'verified' ? this.strings.domainVerifiedLabel : this.strings.domainPendingLabel;
    },
    checkStatusLabel(check) {
      return check.passed ? this.strings.domainVerifiedLabel : this.strings.domainPendingLabel;
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
