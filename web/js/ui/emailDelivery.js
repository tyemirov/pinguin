// @ts-check
import { EMAIL_DELIVERY_STATUS_LABELS, STRINGS } from '../constants.js';

const POLL_INTERVAL_MS = 15_000;
const EXPIRY_INTERVAL_MS = 1_000;
const SEARCH_DEBOUNCE_MS = 300;
const PAGE_SIZE = 50;
const OUTCOME_ORDER = ['bounced', 'rejected', 'rendering_failed', 'complaint', 'delayed', 'unconfirmed', 'delivered'];


/** @param {{ apiClient: ReturnType<typeof import('../core/apiClient.js').createApiClient> }} options */
export function createEmailDelivery({ apiClient }) {
  const authenticated = () => window.Alpine.store('auth').isAuthenticated;
  const accountKey = () => {
    const store = window.Alpine.store('auth');
    if (!store.isAuthenticated) return '';
    return JSON.stringify([store.profile?.user_id || '', store.profile?.user_email || '']);
  };
  return {
    tenants: /** @type {import('../types.d.js').TenantOption[]} */ ([]),
    selectedTenantId: '', searchQuery: '',
    messages: /** @type {import('../types.d.js').EmailDeliveryMessage[]} */ ([]), nextCursor: '',
    isLoadingTenants: false, isLoading: false, isLoadingMore: false, hasLoaded: false, errorMessage: '', loadedPages: 1,
    requestSequence: 0, tenantSequence: 0, controller: null, pollTimer: null, expiryTimer: null,
    searchTimer: null, stopWatch: null, stopAuthWatch: null, resumeListener: null, visibilityListener: null,
    init() {
      this.stopAuthWatch = this.$watch(() => accountKey(), key => {
        this.clearSession();
        if (key) this.loadTenants();
      });
      this.stopWatch = this.$watch('searchQuery', () => this.changeSearch());
      this.resumeListener = () => {
        this.expireMessages();
        if (document.visibilityState !== 'hidden') this.loadCurrentData();
      };
      this.visibilityListener = () => {
        this.expireMessages();
        if (document.visibilityState !== 'hidden') this.loadCurrentData();
      };
      window.addEventListener('focus', this.resumeListener);
      window.addEventListener('online', this.resumeListener);
      window.addEventListener('pageshow', this.resumeListener);
      document.addEventListener('visibilitychange', this.visibilityListener);
      this.pollTimer = setInterval(() => {
        if (document.visibilityState !== 'hidden' && !this.isLoading && !this.isLoadingMore) this.loadCurrentData();
      }, POLL_INTERVAL_MS);
      this.expiryTimer = setInterval(() => this.expireMessages(), EXPIRY_INTERVAL_MS);
      if (authenticated()) this.loadTenants();
    },
    clearSession() {
      this.tenantSequence += 1;
      this.isLoadingTenants = false;
      this.invalidateRequest();
      this.tenants = []; this.selectedTenantId = ''; this.messages = []; this.nextCursor = '';
      this.errorMessage = ''; this.hasLoaded = false; this.loadedPages = 1;
      if (this.searchTimer) clearTimeout(this.searchTimer);
    },
    invalidateRequest() {
      this.requestSequence += 1;
      if (this.controller) this.controller.abort();
      this.controller = null; this.isLoading = false; this.isLoadingMore = false;
    },
    loadCurrentData() {
      return this.selectedTenantId ? this.loadMessages() : this.loadTenants();
    },
    async loadTenants() {
      if (!authenticated() || this.isLoadingTenants) return;
      const sequence = ++this.tenantSequence;
      this.isLoadingTenants = true;
      this.invalidateRequest();
      try {
        const tenants = await apiClient.listTenants();
        if (!authenticated() || sequence !== this.tenantSequence) return;
        this.tenants = tenants;
        this.errorMessage = '';
        this.selectedTenantId = tenants.some(tenant => tenant.id === this.selectedTenantId)
          ? this.selectedTenantId : tenants[0]?.id || '';
        if (this.selectedTenantId) await this.loadMessages();
      } catch (error) {
        if (!authenticated() || sequence !== this.tenantSequence) return;
        this.messages = []; this.tenants = []; this.selectedTenantId = '';
        this.errorMessage = STRINGS.tenants.loadError;
      } finally {
        if (sequence === this.tenantSequence) this.isLoadingTenants = false;
      }
    },
    changeTenant() {
      this.invalidateRequest();
      this.messages = []; this.nextCursor = ''; this.hasLoaded = false; this.errorMessage = ''; this.loadedPages = 1;
      return this.loadMessages();
    },
    changeSearch() {
      this.invalidateRequest();
      this.messages = []; this.nextCursor = ''; this.hasLoaded = false; this.errorMessage = ''; this.loadedPages = 1;
      if (this.searchTimer) clearTimeout(this.searchTimer);
      this.searchTimer = setTimeout(() => { this.searchTimer = null; this.loadMessages(); }, SEARCH_DEBOUNCE_MS);
    },
    async loadMessages(append = false) {
      if (!authenticated() || !this.selectedTenantId || (append && !this.nextCursor)) return;
      const tenantId = this.selectedTenantId;
      const query = this.searchQuery;
      const cursor = append ? this.nextCursor : '';
      this.invalidateRequest();
      const sequence = this.requestSequence;
      const controller = new AbortController();
      this.controller = controller;
      this.isLoading = !append; this.isLoadingMore = append;
      try {
        let page = await apiClient.listEmailDeliveries(tenantId, { query, cursor, limit: PAGE_SIZE, signal: controller.signal });
        const refreshed = [...page.messages];
        if (!append) {
          for (let index = 1; index < this.loadedPages && page.nextCursor; index += 1) {
            if (sequence !== this.requestSequence || !authenticated()) return;
            page = await apiClient.listEmailDeliveries(tenantId, { query, cursor: page.nextCursor, limit: PAGE_SIZE, signal: controller.signal });
            refreshed.push(...page.messages);
          }
        }
        if (!authenticated() || sequence !== this.requestSequence || tenantId !== this.selectedTenantId || query !== this.searchQuery) return;
        const records = append ? [...this.messages, ...page.messages] : refreshed;
        if (append) this.loadedPages += 1;
        this.messages = [...new Map(records.map(message => [message.id, message])).values()];
        this.nextCursor = page.nextCursor; this.errorMessage = ''; this.hasLoaded = true;
        this.expireMessages();
      } catch (error) {
        if (sequence !== this.requestSequence || !authenticated()) return;
        this.messages = []; this.nextCursor = ''; this.hasLoaded = false; this.loadedPages = 1;
        this.errorMessage = error?.statusCode === 503
          ? 'Delivery history is unavailable. It will reconnect automatically.'
          : 'Unable to load delivery history. It will retry automatically.';
      } finally {
        if (sequence === this.requestSequence) {
          this.isLoading = false; this.isLoadingMore = false; this.controller = null;
        }
      }
    },
    expireMessages() {
      const now = Date.now();
      this.messages = this.messages.filter(message => Date.parse(message.expiresAt) > now);
    },
    visibleParent(message) {
      return this.messages.some(item => item.id === message.parentMessageId) ? message.parentMessageId : '';
    },
    visibleForwards(message) {
      const ids = new Set(this.messages.map(item => item.id));
      return message.forwardedMessageIds.filter(id => ids.has(id));
    },
    statusLabel(status) { return EMAIL_DELIVERY_STATUS_LABELS[status]; },
    summaryOutcomes(message) {
      return OUTCOME_ORDER.map(status => ({ status, count: message.recipients.filter(recipient => recipient.status === status).length }))
        .filter(outcome => outcome.count > 0);
    },
    recipientAddresses(message) { return message.recipients.map(recipient => recipient.address).join(', '); },
    formatShortTime(timestamp) { return new Date(timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', hour12: false }); },
    formatTime(timestamp) { return new Date(timestamp).toLocaleString(); },
    messageAnchor(id) { return `delivery-${encodeURIComponent(id)}`; },
    destroy() {
      this.clearSession();
      clearInterval(this.pollTimer); clearInterval(this.expiryTimer);
      if (this.searchTimer) clearTimeout(this.searchTimer);
      window.removeEventListener('focus', this.resumeListener);
      window.removeEventListener('online', this.resumeListener);
      window.removeEventListener('pageshow', this.resumeListener);
      document.removeEventListener('visibilitychange', this.visibilityListener);
      if (typeof this.stopWatch === 'function') this.stopWatch();
      if (typeof this.stopAuthWatch === 'function') this.stopAuthWatch();
    },
  };
}
