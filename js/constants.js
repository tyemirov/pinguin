// @ts-check

/** @type {Window & typeof globalThis & { __PINGUIN_CONFIG__?: Record<string, unknown> }} */
const runtimeWindow = window;
const rawConfig = runtimeWindow.__PINGUIN_CONFIG__ ?? {};
const tenantConfig =
  rawConfig && typeof rawConfig.tenant === 'object' ? rawConfig.tenant : null;

const normalizeUrl = (value) => {
  if (!value || typeof value !== "string") {
    return "";
  }
  return value.trim().replace(/\/$/, "");
};

const PLACEHOLDER_GOOGLE_IDS = new Set([
  "YOUR_GOOGLE_WEB_CLIENT_ID",
  "YOUR_GOOGLE_CLIENT_ID",
  "playwright-client",
  "demo-google-client-id",
]);

const deriveDefaultApiBaseUrl = () => {
  try {
    const { protocol, hostname, port } = window.location;
    if (port === "8080") {
      return `${protocol}//${hostname}:8081/api`;
    }
    if (port && port.length > 0) {
      return `${protocol}//${hostname}:${port}/api`;
    }
    return `${protocol}//${hostname}/api`;
  } catch {
    return "/api";
  }
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

const normalizeOptionalString = (value) => {
  if (!value || typeof value !== "string") {
    return "";
  }
  return value.trim();
};

export const RUNTIME_CONFIG = Object.freeze({
  apiBaseUrl: normalizeUrl(rawConfig.apiBaseUrl) || deriveDefaultApiBaseUrl(),
  tauthBaseUrl: normalizeUrl(rawConfig.tauthBaseUrl),
  tauthTenantId: normalizeOptionalString(rawConfig.tauthTenantId),
  googleClientId: normalizeGoogleClientId(rawConfig.googleClientId),
  landingUrl: String(rawConfig.landingUrl || "/index.html"),
  dashboardUrl: String(rawConfig.dashboardUrl || "/dashboard.html"),
  tenant: tenantConfig,
});

export const STRINGS = Object.freeze({
  appName: "Pinguin Notification Service",
  landing: {
    eyebrow: "Pinguin workspace",
    headline: "Notification delivery without the guesswork",
    subheadline:
      "Sign in to review queued, sent, cancelled, and retrying messages from one operational dashboard.",
  },
  dashboard: {
    title: "Scheduled notifications",
    subtitle: "Review delivery status, adjust schedules, or cancel queued jobs in a single view.",
    profileMenuLabel: "User menu",
    emptyState: "No notifications yet. Start by sending one via the CLI or gRPC client.",
    scheduleDialogTitle: "Reschedule notification",
    scheduleDialogDescription: "Select a new delivery time. Notifications can only be edited while queued.",
    scheduleSuccess: "Delivery time updated",
    cancelSuccess: "Notification cancelled",
    cancelConfirm: "Cancel this queued notification?",
    cancelError: "Unable to cancel notification.",
    rescheduleError: "Unable to reschedule notification.",
    loadError: "Unable to load notifications.",
    searchLabel: "Search",
    searchPlaceholder: "Search notifications",
    loadingMore: "Loading more notifications…",
    endOfResults: "All matching notifications loaded.",
    tenantLabel: "Tenant",
    tenantLoading: "Loading tenants…",
    tenantLoadError: "Unable to load tenants.",
    tenantRequired: "Select a tenant to view notifications.",
  },
  smtpIdentities: {
    title: "SMTP identities",
    subtitle: "Create exact sender credentials for Gmail Send mail as.",
    emailLabel: "Sender address",
    emailPlaceholder: "name@example.com",
    actionColumn: "Actions",
    loadingState: "Loading SMTP identities…",
    emptyState: "No SMTP identities yet.",
    loadError: "Unable to load SMTP identities.",
    createError: "Unable to create SMTP identity.",
    rotateError: "Unable to rotate SMTP credentials.",
    deleteError: "Unable to delete SMTP identity.",
    createSuccess: "SMTP identity created",
    rotateSuccess: "SMTP credentials rotated",
    deleteSuccess: "SMTP identity deleted",
    deleteConfirm: "Delete this SMTP identity?",
    rotateConfirm: "Rotate credentials for this SMTP identity?",
    credentialsTitle: "Gmail SMTP settings",
    credentialsDescription: "This password is shown once.",
    hostLabel: "SMTP server",
    portLabel: "Port",
    securityLabel: "Security",
    usernameLabel: "Username",
    passwordLabel: "Password",
    lastUsedLabel: "Last used",
    neverUsed: "Never",
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
    logout: "Sign out",
    create: "Create",
    rotate: "Rotate",
    delete: "Delete",
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
