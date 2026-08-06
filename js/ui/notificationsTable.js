// @ts-check
import { STATUS_LABELS, STATUS_OPTIONS } from '../constants.js';
import { DOM_EVENTS, dispatchToast, listen } from '../core/events.js';

/** @typedef {import('../types.d.js').NotificationItem} NotificationItem */
/** @typedef {import('../types.d.js').TenantOption} TenantOption */

const NOTIFICATION_PAGE_LIMIT = 50;
const SCROLL_ROOT_MARGIN = '240px 0px';
const SEARCH_DEBOUNCE_MS = 300;

const inputFormatter = {
  toControlValue(isoString) {
    if (!isoString) {
      return '';
    }
    const date = new Date(isoString);
    if (Number.isNaN(date.getTime())) {
      return '';
    }
    const pad = (value) => String(value).padStart(2, '0');
    return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(
      date.getHours(),
    )}:${pad(date.getMinutes())}`;
  },
  toIso(controlValue) {
    if (!controlValue) {
      return null;
    }
    const date = new Date(controlValue);
    if (Number.isNaN(date.getTime())) {
      return null;
    }
    return date.toISOString();
  },
};

/**
 * @param {{
 *   apiClient: ReturnType<typeof import('../core/apiClient.js').createApiClient>,
 *   strings: typeof import('../constants.js').STRINGS.eventLog,
 *   actions: typeof import('../constants.js').STRINGS.actions,
 * }} options
 */
export function createNotificationsTable(options) {
  const { apiClient, strings, actions } = options;
  const authStore = () => window.Alpine.store('auth');

  return {
    strings,
    actions,
    notifications: /** @type {NotificationItem[]} */ ([]),
    tenants: /** @type {TenantOption[]} */ ([]),
    selectedTenantId: '',
    statusFilter: 'all',
    searchQuery: '',
    nextCursor: '',
    isLoading: false,
    isLoadingMore: false,
    isLoadingTenants: false,
    hasLoadedNotifications: false,
    hasUserScrolled: false,
    errorMessage: '',
    scheduleDialogVisible: false,
    scheduleForm: {
      id: '',
      tenantId: '',
      scheduledTime: '',
    },
    stopListening: null,
    stopScrollWatcher: null,
    scrollObserver: null,
    searchDebounceTimer: null,
    requestSequence: 0,
    STATUS_OPTIONS,
    init() {
      this.selectedTenantId = '';
      this.refreshIfAuthenticated();
      this.$watch(
        () => authStore().isAuthenticated,
        (isAuthenticated) => {
          if (isAuthenticated) {
            this.loadTenants();
          } else {
            this.notifications = [];
            this.tenants = [];
            this.nextCursor = '';
            this.hasLoadedNotifications = false;
          }
        },
      );
      this.$watch('searchQuery', () => {
        this.changeSearch();
      });
      this.stopListening = listen(DOM_EVENTS.refresh, () => {
        if (authStore().isAuthenticated) {
          this.loadNotifications();
        }
      });
      const handleWindowScroll = () => {
        if (window.scrollY > 0) {
          this.hasUserScrolled = true;
        }
      };
      window.addEventListener('scroll', handleWindowScroll, { passive: true });
      this.stopScrollWatcher = () => {
        window.removeEventListener('scroll', handleWindowScroll);
      };
      this.$nextTick(() => {
        this.startScrollObserver();
      });
    },
    async loadNotifications() {
      await this.loadNotificationPage(true);
    },
    async loadNextPage() {
      await this.loadNotificationPage(false);
    },
    async loadNotificationPage(resetPage) {
      if (!authStore().isAuthenticated) {
        return;
      }
      if (!this.selectedTenantId) {
        this.notifications = [];
        this.nextCursor = '';
        this.hasLoadedNotifications = false;
        this.hasUserScrolled = false;
        this.errorMessage = this.strings.tenantRequired;
        return;
      }
      if (!resetPage && (!this.nextCursor || this.isLoading || this.isLoadingMore)) {
        return;
      }
      const requestId = this.requestSequence + 1;
      this.requestSequence = requestId;
      if (resetPage) {
        this.isLoading = true;
        this.notifications = [];
        this.nextCursor = '';
        this.hasLoadedNotifications = false;
        this.hasUserScrolled = false;
      } else {
        this.isLoadingMore = true;
      }
      this.errorMessage = '';
      try {
        const statuses = this.statusFilter === 'all' ? [] : [this.statusFilter];
        const page = await apiClient.listNotifications(statuses, this.selectedTenantId, {
          query: this.searchQuery,
          cursor: resetPage ? '' : this.nextCursor,
          limit: NOTIFICATION_PAGE_LIMIT,
        });
        if (requestId !== this.requestSequence) {
          return;
        }
        this.notifications = resetPage
          ? page.notifications
          : appendUniqueNotifications(this.notifications, page.notifications);
        this.nextCursor = page.nextCursor;
        this.hasLoadedNotifications = true;
      } catch (error) {
        if (requestId !== this.requestSequence) {
          return;
        }
        this.errorMessage = this.strings.loadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        if (requestId === this.requestSequence) {
          this.isLoading = false;
          this.isLoadingMore = false;
        }
      }
    },
    async refreshIfAuthenticated() {
      if (authStore().isAuthenticated) {
        await this.loadTenants();
      }
    },
    async loadTenants() {
      if (!authStore().isAuthenticated) {
        return;
      }
      this.isLoadingTenants = true;
      this.errorMessage = '';
      try {
        this.tenants = await apiClient.listTenants();
        this.selectedTenantId = selectTenantId(this.selectedTenantId, this.tenants);
        await this.loadNotifications();
      } catch (error) {
        this.tenants = [];
        this.selectedTenantId = '';
        this.notifications = [];
        this.errorMessage = this.strings.tenantLoadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoadingTenants = false;
      }
    },
    async changeTenant() {
      await this.loadNotifications();
    },
    changeSearch() {
      if (this.searchDebounceTimer) {
        clearTimeout(this.searchDebounceTimer);
      }
      this.searchDebounceTimer = setTimeout(() => {
        this.searchDebounceTimer = null;
        this.loadNotifications();
      }, SEARCH_DEBOUNCE_MS);
    },
    startScrollObserver() {
      const sentinel = this.$refs.scrollSentinel;
      if (!sentinel || typeof IntersectionObserver !== 'function') {
        return;
      }
      this.scrollObserver = new IntersectionObserver(
        (entries) => {
          if (this.hasUserScrolled && entries.some((entry) => entry.isIntersecting)) {
            this.loadNextPage();
          }
        },
        { rootMargin: SCROLL_ROOT_MARGIN },
      );
      this.scrollObserver.observe(sentinel);
    },
    formatStatus(status) {
      return STATUS_LABELS[status] || status;
    },
    formatTimestamp(isoString) {
      if (!isoString) {
        return '—';
      }
      const date = new Date(isoString);
      if (Number.isNaN(date.getTime())) {
        return '—';
      }
      return date.toLocaleString();
    },
    openScheduleDialog(notification) {
      this.scheduleForm.id = notification.id;
      this.scheduleForm.tenantId = notification.tenantId || '';
      this.scheduleForm.scheduledTime = inputFormatter.toControlValue(notification.scheduledFor);
      this.scheduleDialogVisible = true;
      const dialog = this.$refs.scheduleDialog;
      if (dialog && typeof dialog.showModal === 'function') {
        dialog.showModal();
      }
    },
    closeScheduleDialog() {
      this.scheduleDialogVisible = false;
      const dialog = this.$refs.scheduleDialog;
      if (dialog && typeof dialog.close === 'function') {
        dialog.close();
      }
    },
    async submitSchedule(event) {
      event?.preventDefault();
      const isoValue = inputFormatter.toIso(this.scheduleForm.scheduledTime);
      if (!isoValue) {
        this.errorMessage = this.strings.rescheduleError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      if (!this.scheduleForm.tenantId) {
        this.errorMessage = this.strings.rescheduleError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      try {
        const targetTenantId = this.scheduleForm.tenantId;
        await apiClient.rescheduleNotification(this.scheduleForm.id, isoValue, targetTenantId);
        await this.loadNotifications();
        dispatchToast({ variant: 'success', message: this.strings.scheduleSuccess });
        this.closeScheduleDialog();
      } catch (error) {
        this.errorMessage = this.strings.rescheduleError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      }
    },
    async cancelNotification(notification) {
      if (!authStore().isAuthenticated) {
        return;
      }
      if (!window.confirm(this.strings.cancelConfirm)) {
        return;
      }
      this.isLoading = true;
      try {
        if (!notification.tenantId) {
          throw new Error('missing_tenant_id');
        }
        const targetTenantId = notification.tenantId;
        await apiClient.cancelNotification(notification.id, targetTenantId);
        await this.loadNotifications();
        dispatchToast({ variant: 'success', message: this.strings.cancelSuccess });
      } catch (error) {
        this.errorMessage = this.strings.cancelError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoading = false;
      }
    },
    $cleanup() {
      if (typeof this.stopListening === 'function') {
        this.stopListening();
      }
      if (typeof this.stopScrollWatcher === 'function') {
        this.stopScrollWatcher();
      }
      if (this.searchDebounceTimer) {
        clearTimeout(this.searchDebounceTimer);
      }
      if (this.scrollObserver) {
        this.scrollObserver.disconnect();
      }
    },
  };
}

/**
 * @param {NotificationItem[]} currentItems
 * @param {NotificationItem[]} nextItems
 * @returns {NotificationItem[]}
 */
function appendUniqueNotifications(currentItems, nextItems) {
  const existingIds = new Set(currentItems.map((item) => item.id));
  const mergedItems = currentItems.slice();
  nextItems.forEach((item) => {
    if (!existingIds.has(item.id)) {
      existingIds.add(item.id);
      mergedItems.push(item);
    }
  });
  return mergedItems;
}

/**
 * @param {string} selectedTenantId
 * @param {TenantOption[]} tenants
 */
function selectTenantId(selectedTenantId, tenants) {
  const normalizedSelected = typeof selectedTenantId === 'string' ? selectedTenantId.trim() : '';
  if (normalizedSelected && tenants.some((tenant) => tenant.id === normalizedSelected)) {
    return normalizedSelected;
  }
  return tenants.length > 0 ? tenants[0].id : '';
}
