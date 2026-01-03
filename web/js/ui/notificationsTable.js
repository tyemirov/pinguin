// @ts-check
import { RUNTIME_CONFIG, STATUS_LABELS, STATUS_OPTIONS } from '../constants.js';
import { DOM_EVENTS, dispatchToast, listen } from '../core/events.js';

/** @typedef {import('../types.d.js').NotificationItem} NotificationItem */

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
 *   strings: typeof import('../constants.js').STRINGS.dashboard,
 *   actions: typeof import('../constants.js').STRINGS.actions,
 * }} options
 */
export function createNotificationsTable(options) {
  const { apiClient, strings, actions } = options;
  const authStore = () => window.Alpine.store('auth');
  const isGlobalView = () => RUNTIME_CONFIG.viewScope === 'global';

  return {
    strings,
    actions,
    viewScope: RUNTIME_CONFIG.viewScope,
    notifications: /** @type {NotificationItem[]} */ ([]),
    statusFilter: 'all',
    isLoading: false,
    errorMessage: '',
    scheduleDialogVisible: false,
    scheduleForm: {
      id: '',
      tenantId: '',
      scheduledTime: '',
    },
    stopListening: null,
    STATUS_OPTIONS,
    init() {
      this.refreshIfAuthenticated();
      this.$watch(
        () => authStore().isAuthenticated,
        (isAuthenticated) => {
          if (isAuthenticated) {
            this.loadNotifications();
          } else {
            this.notifications = [];
          }
        },
      );
      this.stopListening = listen(DOM_EVENTS.refresh, () => {
        if (authStore().isAuthenticated) {
          this.loadNotifications();
        }
      });
    },
    async loadNotifications() {
      if (!authStore().isAuthenticated) {
        return;
      }
      this.isLoading = true;
      this.errorMessage = '';
      try {
        const statuses = this.statusFilter === 'all' ? [] : [this.statusFilter];
        this.notifications = await apiClient.listNotifications(statuses);
      } catch (error) {
        this.errorMessage = this.strings.loadError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
      } finally {
        this.isLoading = false;
      }
    },
    async refreshIfAuthenticated() {
      if (authStore().isAuthenticated) {
        await this.loadNotifications();
      }
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
      if (isGlobalView() && !this.scheduleForm.tenantId) {
        this.errorMessage = this.strings.rescheduleError;
        dispatchToast({ variant: 'error', message: this.errorMessage });
        return;
      }
      try {
        const targetTenantId = isGlobalView() ? this.scheduleForm.tenantId : '';
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
        if (isGlobalView() && !notification.tenantId) {
          throw new Error('missing_tenant_id');
        }
        const targetTenantId = isGlobalView() ? notification.tenantId : '';
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
    },
  };
}
