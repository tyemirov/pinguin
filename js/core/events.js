// @ts-check

export const DOM_EVENTS = Object.freeze({
  toast: 'notifications:toast',
  refresh: 'notifications:refresh',
});

export function dispatchToast(detail) {
  document.dispatchEvent(
    new CustomEvent(DOM_EVENTS.toast, {
      detail: {
        message: detail?.message || 'Action completed',
        variant: detail?.variant || 'info',
        duration: detail?.duration ?? 4000,
      },
    }),
  );
}

export function dispatchRefresh() {
  document.dispatchEvent(new CustomEvent(DOM_EVENTS.refresh));
}

export function listen(eventName, handler) {
  document.addEventListener(eventName, handler);
  return () => document.removeEventListener(eventName, handler);
}
