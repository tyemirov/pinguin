// @ts-check
import { DOM_EVENTS, listen } from '../core/events.js';

export function createToastCenter() {
  return {
    toasts: [],
    init() {
      this.stopListening = listen(DOM_EVENTS.toast, (event) => {
        const detail = event.detail || {};
        const toast = {
          id: crypto.randomUUID ? crypto.randomUUID() : String(Date.now() + Math.random()),
          message: detail.message || 'Action completed',
          variant: detail.variant || 'info',
          expiresAt: Date.now() + (detail.duration ?? 4000),
        };
        this.toasts.push(toast);
        const delay = toast.expiresAt - Date.now();
        setTimeout(() => this.dismissToast(toast.id), Math.max(delay, 1000));
      });
    },
    stopListening: null,
    dismissToast(id) {
      this.toasts = this.toasts.filter((toast) => toast.id !== id);
    },
    $cleanup() {
      if (typeof this.stopListening === 'function') {
        this.stopListening();
      }
    },
  };
}
