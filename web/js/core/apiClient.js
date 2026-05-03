// @ts-check
import { RUNTIME_CONFIG } from '../constants.js';

/** @typedef {import('../types.d.js').NotificationItem} NotificationItem */
/** @typedef {import('../types.d.js').SMTPIdentity} SMTPIdentity */
/** @typedef {import('../types.d.js').SMTPCredentials} SMTPCredentials */

function getFetcher() {
  if (typeof window !== 'undefined' && typeof window.apiFetch === 'function') {
    return window.apiFetch;
  }
  return (input, init = {}) => fetch(input, { credentials: 'include', ...init });
}

function toJson(response) {
  return response
    .clone()
    .json()
    .catch(() => null);
}

function mapNotification(raw) {
  if (!raw) {
    return null;
  }
  return {
    id: raw.notification_id,
    tenantId: raw.tenant_id,
    type: raw.notification_type,
    recipient: raw.recipient,
    subject: raw.subject,
    message: raw.message,
    status: raw.status,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
    scheduledFor: raw.scheduled_for || raw.scheduled_time || null,
    retryCount: raw.retry_count ?? 0,
  };
}

function mapSMTPIdentity(raw) {
  if (!raw) {
    return null;
  }
  return {
    id: raw.id,
    emailAddress: raw.email_address,
    username: raw.username,
    status: raw.status,
    lastUsedAt: raw.last_used_at || null,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  };
}

function mapSMTPCredentials(raw) {
  const identity = mapSMTPIdentity(raw?.identity);
  if (!identity) {
    return null;
  }
  return {
    identity,
    smtpSettings: {
      host: raw?.smtp_settings?.host || '',
      port: Number(raw?.smtp_settings?.port || 0),
      securityMode: raw?.smtp_settings?.security_mode || '',
    },
    username: raw?.username || '',
    password: raw?.password || '',
  };
}

function buildTenantQuery(tenantId) {
  if (!tenantId || typeof tenantId !== 'string') {
    return '';
  }
  const normalized = tenantId.trim();
  if (!normalized) {
    return '';
  }
  return `?tenant_id=${encodeURIComponent(normalized)}`;
}

export function createApiClient(baseUrl = RUNTIME_CONFIG.apiBaseUrl) {
  const normalizedBase = baseUrl.replace(/\/$/, '') || '/api';

  async function request(path, init = {}) {
    const url = `${normalizedBase}${path}`;
    const mergedInit = { ...init };
    mergedInit.headers = {
      'Content-Type': 'application/json',
      ...(init.headers || {}),
    };
    const fetcher = getFetcher();
    const response = await fetcher(url, mergedInit);
    const payload = await toJson(response);
    if (!response.ok) {
      const error = new Error(payload?.error || `request_failed_${response.status}`);
      error.name = 'ApiError';
      // @ts-expect-error augment error object for downstream logic
      error.statusCode = response.status;
      throw error;
    }
    return payload;
  }

  return {
    async listNotifications(statuses = []) {
      const query = new URLSearchParams();
      statuses.filter(Boolean).forEach((status) => {
        query.append('status', String(status));
      });
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const payload = await request(`/notifications${suffix}`, { method: 'GET', headers: {} });
      const items = Array.isArray(payload?.notifications) ? payload.notifications : [];
      return /** @type {NotificationItem[]} */ (
        items.map(mapNotification).filter(Boolean)
      );
    },
    async rescheduleNotification(notificationId, scheduledIsoString, tenantId) {
      const payload = await request(
        `/notifications/${encodeURIComponent(notificationId)}/schedule${buildTenantQuery(tenantId)}`,
        {
          method: 'PATCH',
          body: JSON.stringify({ scheduled_time: scheduledIsoString }),
        },
      );
      return mapNotification(payload);
    },
    async cancelNotification(notificationId, tenantId) {
      const payload = await request(
        `/notifications/${encodeURIComponent(notificationId)}/cancel${buildTenantQuery(tenantId)}`,
        {
          method: 'POST',
        },
      );
      return mapNotification(payload);
    },
    async listSMTPIdentities() {
      const payload = await request('/smtp-identities', { method: 'GET', headers: {} });
      const identities = Array.isArray(payload?.identities) ? payload.identities : [];
      return /** @type {SMTPIdentity[]} */ (
        identities.map(mapSMTPIdentity).filter(Boolean)
      );
    },
    async createSMTPIdentity(emailAddress) {
      const payload = await request('/smtp-identities', {
        method: 'POST',
        body: JSON.stringify({ email_address: emailAddress }),
      });
      return /** @type {SMTPCredentials | null} */ (mapSMTPCredentials(payload));
    },
    async rotateSMTPIdentity(identityId) {
      const payload = await request(`/smtp-identities/${encodeURIComponent(identityId)}/rotate`, {
        method: 'POST',
      });
      return /** @type {SMTPCredentials | null} */ (mapSMTPCredentials(payload));
    },
    async deleteSMTPIdentity(identityId) {
      await request(`/smtp-identities/${encodeURIComponent(identityId)}`, {
        method: 'DELETE',
      });
    },
  };
}
