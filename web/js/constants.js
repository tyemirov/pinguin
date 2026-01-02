// @ts-check

/** @type {Window & typeof globalThis & { __PINGUIN_CONFIG__?: Record<string, unknown> }} */
const runtimeWindow = window;
const rawConfig = runtimeWindow.__PINGUIN_CONFIG__ ?? {};
const tauthConfig = runtimeWindow.PINGUIN_TAUTH_CONFIG || {};
const tenantConfig =
  rawConfig && typeof rawConfig.tenant === 'object' ? rawConfig.tenant : null;

const normalizeUrl = (value, fallback) => {
  if (!value || typeof value !== "string") {
    return fallback;
  }
  return value.trim().replace(/\/$/, "") || fallback;
};

const deriveDefaultApiBaseUrl = () => {
  try {
    const { protocol, hostname, port } = window.location;
    if (port === "4173") {
      return `${protocol}//${hostname}:8080/api`;
    }
    if (port && port.length > 0) {
      return `${protocol}//${hostname}:${port}/api`;
    }
    return `${protocol}//${hostname}/api`;
  } catch {
    return "/api";
  }
};

const PLACEHOLDER_GOOGLE_IDS = new Set([
  "YOUR_GOOGLE_WEB_CLIENT_ID",
  "YOUR_GOOGLE_CLIENT_ID",
  "playwright-client",
  "demo-google-client-id",
]);

const deriveGoogleClientId = () => {
  try {
    const host = document.querySelector("mpr-header");
    const siteId =
      host && typeof host.getAttribute === "function"
        ? host.getAttribute("google-site-id") || host.getAttribute("site-id")
        : "";
    if (siteId && siteId.trim()) {
      return siteId.trim();
    }
  } catch {
    // ignore
  }
  return "991677581607-r0dj8q6irjagipali0jpca7nfp8sfj9r.apps.googleusercontent.com";
};

const normalizeGoogleClientId = (value) => {
  if (!value || typeof value !== "string") {
    return "";
  }
  const trimmed = value.trim();
  if (!trimmed || PLACEHOLDER_GOOGLE_IDS.has(trimmed)) {
    return "";
  }
  return trimmed;
};

export const RUNTIME_CONFIG = Object.freeze({
  apiBaseUrl: normalizeUrl(rawConfig.apiBaseUrl, deriveDefaultApiBaseUrl()),
  tauthBaseUrl: normalizeUrl(rawConfig.tauthBaseUrl || tauthConfig.baseUrl, "http://localhost:8081"),
  googleClientId:
    normalizeGoogleClientId(rawConfig.googleClientId || tauthConfig.googleClientId) || deriveGoogleClientId(),
  landingUrl: String(rawConfig.landingUrl || "/index.html"),
  dashboardUrl: String(rawConfig.dashboardUrl || "/dashboard.html"),
  tenant: tenantConfig,
});

const tenantDisplayName =
  (tenantConfig && typeof tenantConfig.displayName === "string" && tenantConfig.displayName.trim()) ||
  "Pinguin Notification Service";

export const STRINGS = Object.freeze({
  appName: tenantDisplayName,
  landing: {
    eyebrow: "Trusted delivery infrastructure",
    headline: "Deliver email and SMS notifications with confidence",
    subheadline:
      "Preview schedules, manage queued notifications, and keep deliveries on track from a single workspace.",
    ctaPrimary: "Enter workspace",
    ctaSecondary: "Explore platform",
    securityCopy: "Your session stays protected by HttpOnly cookies.",
  },
  dashboard: {
    title: "Scheduled notifications",
    subtitle: "Review delivery status, adjust schedules, or cancel queued jobs in a single view.",
    emptyState: "No notifications yet. Start by sending one via the CLI or gRPC client.",
    scheduleDialogTitle: "Reschedule notification",
    scheduleDialogDescription: "Select a new delivery time. Notifications can only be edited while queued.",
    scheduleSuccess: "Delivery time updated",
    cancelSuccess: "Notification cancelled",
    cancelConfirm: "Cancel this queued notification?",
    cancelError: "Unable to cancel notification.",
    rescheduleError: "Unable to reschedule notification.",
    loadError: "Unable to load notifications.",
  },
  auth: {
    signingIn: "Preparing secure session…",
    ready: "Workspace ready",
    failed: "We could not reach the authentication service. Please retry.",
    loggedOut: "Session ended. Redirecting…",
  },
  actions: {
    refresh: "Refresh",
    reschedule: "Reschedule",
    cancel: "Cancel",
    saveChanges: "Save changes",
    close: "Close",
    logout: "Log out",
  },
});

export const STATUS_LABELS = Object.freeze({
  queued: "Queued",
  sent: "Sent",
  errored: "Errored",
  cancelled: "Cancelled",
});

export const STATUS_OPTIONS = Object.freeze([
  { value: "all", label: "All statuses" },
  { value: "queued", label: STATUS_LABELS.queued },
  { value: "sent", label: STATUS_LABELS.sent },
  { value: "errored", label: STATUS_LABELS.errored },
  { value: "cancelled", label: STATUS_LABELS.cancelled },
]);
