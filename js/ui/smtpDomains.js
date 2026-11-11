// @ts-check
import { dispatchToast } from '../core/events.js';

/** @typedef {import('../types.d.js').SMTPSenderDomain} SMTPSenderDomain */
/** @typedef {import('../types.d.js').SMTPDomainDNSRecord} SMTPDomainDNSRecord */

/**
 * Creates the sender-domain portion of the SMTP workspace.
 *
 * @param {{
 *   apiClient: ReturnType<typeof import('../core/apiClient.js').createApiClient>,
 *   strings: typeof import('../constants.js').STRINGS.smtpIdentities,
 *   authStore: () => { isAuthenticated: boolean },
 * }} options
 */
export function createSMTPDomains(options) {
  const { apiClient, strings, authStore } = options;

  return {
    domains: /** @type {SMTPSenderDomain[]} */ ([]),
    domainName: '',
    isLoadingDomains: false,
    checkingDomainId: 0,
    expandedDomainId: 0,
    async loadDomains() {
      if (!authStore().isAuthenticated) {
        return;
      }
      this.isLoadingDomains = true;
      this.errorMessage = '';
      try {
        this.domains = await apiClient.listSMTPDomains(this.selectedTenantId);
        if (!this.domains.some((domain) => domain.id === this.expandedDomainId)) {
          this.expandedDomainId = 0;
        }
      } catch (error) {
        this.errorMessage = strings.domainLoadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoadingDomains = false;
      }
    },
    async createDomain(event) {
      event?.preventDefault();
      const domainName = this.domainName.trim();
      if (!domainName) {
        this.errorMessage = strings.domainCreateError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      this.isSubmitting = true;
      this.errorMessage = '';
      try {
        const domain = await apiClient.createSMTPDomain(this.selectedTenantId, domainName);
        if (!domain) {
          throw new Error('missing_domain');
        }
        this.upsertDomain(domain);
        this.expandedDomainId = domain.id;
        this.domainName = '';
        dispatchToast({ variant: 'success', message: strings.domainCreateSuccess });
      } catch (error) {
        this.errorMessage = this.domainCreateErrorMessage(error);
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isSubmitting = false;
      }
    },
    domainCreateErrorMessage(error) {
      if (error instanceof Error) {
        if (error.message === 'sender domain is already registered') {
          return strings.domainCreateExistsError;
        }
        if (error.message === 'sender domain is invalid') {
          return strings.domainCreateInvalidError;
        }
      }
      return strings.domainCreateError;
    },
    async checkDomain(domain) {
      this.checkingDomainId = domain.id;
      this.expandedDomainId = domain.id;
      this.errorMessage = '';
      try {
        const checkedDomain = await apiClient.checkSMTPDomainDNS(this.selectedTenantId, domain.id);
        if (!checkedDomain) {
          throw new Error('missing_domain');
        }
        this.upsertDomain(checkedDomain);
        dispatchToast({ variant: 'success', message: strings.domainCheckSuccess });
      } catch (error) {
        this.errorMessage = strings.domainCheckError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.checkingDomainId = 0;
      }
    },
    upsertDomain(domain) {
      const index = this.domains.findIndex((candidate) => candidate.id === domain.id);
      if (index === -1) {
        this.domains = [...this.domains, domain].sort((left, right) =>
          left.domain.localeCompare(right.domain),
        );
        return;
      }
      this.domains = this.domains.map((candidate) =>
        candidate.id === domain.id ? domain : candidate,
      );
    },
    toggleDomain(domain) {
      this.expandedDomainId = this.expandedDomainId === domain.id ? 0 : domain.id;
    },
    domainToggleLabel(domain) {
      const action =
        this.expandedDomainId === domain.id
          ? strings.domainCollapseLabel
          : strings.domainExpandLabel;
      return `${action} ${domain.domain} DNS setup`;
    },
    domainStatusLabel(domain) {
      return domain.status === 'verified'
        ? strings.domainVerifiedLabel
        : strings.domainPendingLabel;
    },
    domainLastCheckedLabel(domain) {
      if (!domain.lastCheckedAt) {
        return strings.domainNeverCheckedLabel;
      }
      const checkedAt = new Date(domain.lastCheckedAt);
      if (Number.isNaN(checkedAt.getTime())) {
        return strings.domainNeverCheckedLabel;
      }
      return `${strings.domainLastCheckedLabel} ${checkedAt.toLocaleString()}`;
    },
    domainRecordCheck(domain, record) {
      return (
        domain.dnsChecks.find(
          (check) => check.type === record.type && check.host === record.host,
        ) || null
      );
    },
    domainRecordState(domain, record) {
      const check = this.domainRecordCheck(domain, record);
      if (!check) {
        return 'pending';
      }
      return check.passed ? 'verified' : 'errored';
    },
    domainRecordStatusLabel(domain, record) {
      const check = this.domainRecordCheck(domain, record);
      if (!check) {
        return strings.domainCheckPendingLabel;
      }
      return check.passed ? strings.domainVerifiedLabel : strings.domainPendingLabel;
    },
    domainRecordMessage(domain, record) {
      const check = this.domainRecordCheck(domain, record);
      if (!check) {
        return strings.domainCheckPendingLabel;
      }
      if (check.message) {
        return check.message;
      }
      return check.passed ? strings.domainCheckPassedLabel : strings.domainCheckFailedLabel;
    },
    verifiedDomains() {
      return this.domains.filter((domain) => domain.status === 'verified');
    },
    async copyDNSValue(value, successMessage) {
      try {
        if (!navigator.clipboard || typeof navigator.clipboard.writeText !== 'function') {
          throw new Error('clipboard_unavailable');
        }
        await navigator.clipboard.writeText(String(value));
        dispatchToast({ variant: 'success', message: successMessage });
      } catch (error) {
        dispatchToast({ variant: 'error', message: strings.copyError });
      }
    },
  };
}
