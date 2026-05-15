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

export const RUNTIME_CONFIG = Object.freeze({
  apiBaseUrl: normalizeUrl(rawConfig.apiBaseUrl) || deriveDefaultApiBaseUrl(),
  landingUrl: String(rawConfig.landingUrl || "/index.html"),
  eventLogUrl: String(rawConfig.eventLogUrl || "/event-log.html"),
  smtpRelayUrl: String(rawConfig.smtpRelayUrl || "/smtp-relay.html"),
  tenant: tenantConfig,
});

export const STRINGS = Object.freeze({
  appName: "Pinguin Notification Service",
  landing: {
    eyebrow: "Pinguin workspace",
    headline: "Notification delivery without the guesswork",
    subheadline:
      "Sign in to review queued, sent, cancelled, and retrying messages from one operational workspace.",
  },
  eventLog: {
    title: "Event log",
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
    title: "SMTP relay",
    subtitle: "Create shared sender credentials and reply forwarding.",
    domainSetupTitle: "Sender domain DNS",
    domainLabel: "Sending domain",
    domainPlaceholder: "example.com",
    domainRecordsLabel: "DNS records",
    domainHostColumn: "Host",
    domainTypeColumn: "Type",
    domainValueColumn: "Value",
    domainPurposeColumn: "Purpose",
    domainStatusColumn: "Status",
    domainPendingLabel: "Pending DNS",
    domainVerifiedLabel: "Verified",
    domainCheckLabel: "Check DNS",
    domainAddLabel: "Add domain",
    domainLoadingState: "Loading sender domains…",
    domainEmptyState: "Add a sender domain before creating SMTP credentials.",
    domainCreateSuccess: "Sender domain added",
    domainCheckSuccess: "DNS check completed",
    domainCreateError: "Unable to add sender domain.",
    domainLoadError: "Unable to load sender domains.",
    domainCheckError: "Unable to check DNS.",
    domainRequiredNotice: "Verify this sender domain before creating SMTP credentials.",
    outboundTitle: "Outbound credentials",
    inboundTitle: "Inbound fanout",
    emailLabel: "Sender address",
    emailPlaceholder: "name@example.com",
    forwardToLabel: "Forward copies to",
    forwardToPlaceholder: "owner@example.com\nmaria@example.com",
    forwardToDisplayLabel: "Forwarding owners",
    actionColumn: "Actions",
    loadingState: "Loading SMTP identities…",
    emptyState: "No SMTP identities yet.",
    loadError: "Unable to load SMTP identities.",
    credentialsLoadError: "Unable to load SMTP credentials.",
    createError: "Unable to create SMTP identity.",
    updateForwardingError: "Unable to update forwarding owners.",
    rotateError: "Unable to rotate SMTP credentials.",
    deleteError: "Unable to delete SMTP identity.",
    createSuccess: "SMTP identity created",
    credentialsLoadSuccess: "SMTP credentials loaded",
    updateForwardingSuccess: "Forwarding owners updated",
    rotateSuccess: "SMTP credentials rotated",
    deleteSuccess: "SMTP identity deleted",
    deleteConfirm: "Delete this SMTP identity?",
    editForwardingLabel: "Edit forwarding owners",
    viewPasswordLabel: "View password",
    cancelEditLabel: "Cancel edit",
    rotateConfirm: "Rotate credentials for this SMTP identity?",
    rotateCredentialsLabel: "Rotate credentials",
    credentialsTitle: "Gmail SMTP settings",
    credentialsDescription: "Use these current settings for Gmail Send-As.",
    credentialsCloseLabel: "Close Gmail SMTP settings",
    hostLabel: "SMTP server",
    hostCopyLabel: "Copy SMTP server",
    hostCopySuccess: "SMTP server copied",
    portLabel: "Port",
    securityLabel: "Security",
    usernameLabel: "Username",
    usernameCopyLabel: "Copy username",
    usernameCopySuccess: "Username copied",
    passwordLabel: "Password",
    passwordCopyLabel: "Copy password",
    passwordCopySuccess: "Password copied",
    copyError: "Unable to copy value.",
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
    copy: "Copy",
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
