// @ts-check
import { RUNTIME_CONFIG } from '../constants.js';

function getFetcher() {
  if (typeof window !== 'undefined' && typeof window.apiFetch === 'function') {
    return window.apiFetch;
  }
  return (input, init = {}) => fetch(input, { credentials: 'include', ...init });
}

function toJson(response) {
  if (response.status === 204) {
    return Promise.resolve(null);
  }
  return response.clone().json().catch(() => null);
}

function mapNotification(raw) {
  if (!raw) return null;
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

function mapTenant(raw) {
  if (!raw || typeof raw.id !== 'string' || !raw.id.trim()) return null;
  return {
    id: raw.id.trim(),
    displayName: String(raw.display_name || raw.displayName || raw.id).trim(),
    supportEmail: String(raw.support_email || ''),
    version: Number(raw.version || 0),
    createdAt: raw.created_at || '',
    updatedAt: raw.updated_at || '',
    emailProfile: raw.email_profile || null,
    smsProfile: raw.sms_profile || null,
    apiCredential: raw.api_credential || null,
  };
}

function mapSMTPIdentity(raw) {
  if (!raw) return null;
  return {
    id: raw.id,
    emailAddress: raw.email_address,
    username: raw.username,
    forwardTo: Array.isArray(raw.forward_to) ? raw.forward_to.filter((value) => typeof value === 'string') : [],
    status: raw.status,
    lastUsedAt: raw.last_used_at || null,
    createdAt: raw.created_at,
    updatedAt: raw.updated_at,
  };
}

function mapSMTPCredentials(raw) {
  const identity = mapSMTPIdentity(raw?.identity);
  if (!identity) return null;
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

function mapSMTPSenderDomain(raw) {
  if (!raw) return null;
  const id = Number(raw.id || 0);
  if (!Number.isInteger(id) || id <= 0) return null;
  const dnsRecords = Array.isArray(raw.dns_records)
    ? raw.dns_records.map((record) => ({
        type: String(record?.type || ''), host: String(record?.host || ''),
        value: String(record?.value || ''), purpose: String(record?.purpose || ''),
      })).filter((record) => record.type && record.host && record.value)
    : [];
  const dnsChecks = Array.isArray(raw.dns_checks)
    ? raw.dns_checks.map((check) => ({
        type: String(check?.type || ''), host: String(check?.host || ''),
        expected: String(check?.expected || ''), passed: Boolean(check?.passed),
        message: String(check?.message || ''),
      })).filter((check) => check.type && check.host)
    : [];
  return {
    id, domain: String(raw.domain || ''), status: String(raw.status || 'pending'),
    dnsRecords, dnsChecks, lastCheckedAt: raw.last_checked_at || null,
    createdAt: raw.created_at || '', updatedAt: raw.updated_at || '',
  };
}

function tenantPath(tenantId, suffix = '') {
  const normalized = typeof tenantId === 'string' ? tenantId.trim() : '';
  if (!normalized) throw new Error('tenant_required');
  return `/tenants/${encodeURIComponent(normalized)}${suffix}`;
}

export function createApiClient(baseUrl = RUNTIME_CONFIG.apiBaseUrl) {
  const normalizedBase = baseUrl.replace(/\/$/, '') || '/api';

  async function request(path, init = {}) {
    const mergedInit = { ...init };
    if (init.body) {
      mergedInit.headers = { 'Content-Type': 'application/json', ...(init.headers || {}) };
    }
    const response = await getFetcher()(`${normalizedBase}${path}`, mergedInit);
    const payload = await toJson(response);
    if (!response.ok) {
      const message = payload?.error?.message || payload?.error || `request_failed_${response.status}`;
      const error = new Error(message);
      error.name = 'ApiError';
      error.statusCode = response.status;
      error.code = payload?.error?.code || '';
      throw error;
    }
    return payload;
  }

  return {
    async listTenants() {
      const payload = await request('/tenants', { method: 'GET' });
      return (Array.isArray(payload?.tenants) ? payload.tenants : []).map(mapTenant).filter(Boolean);
    },
    async createTenant(payload, idempotencyKey) {
      return mapTenant(await request('/tenants', {
        method: 'POST', headers: { 'Idempotency-Key': idempotencyKey }, body: JSON.stringify(payload),
      }));
    },
    async updateTenant(tenantId, version, payload) {
      return mapTenant(await request(tenantPath(tenantId), {
        method: 'PUT', headers: { 'If-Match': `"${version}"` }, body: JSON.stringify(payload),
      }));
    },
    async deleteTenant(tenantId, version) {
      await request(tenantPath(tenantId), { method: 'DELETE', headers: { 'If-Match': `"${version}"` } });
    },
    async patchEmailProfile(tenantId, version, payload) {
      return request(tenantPath(tenantId, '/email-profile'), {
        method: 'PATCH', headers: { 'If-Match': `"${version}"` }, body: JSON.stringify(payload),
      });
    },
    async putSMSProfile(tenantId, version, payload) {
      return request(tenantPath(tenantId, '/sms-profile'), {
        method: 'PUT', headers: { 'If-Match': `"${version}"` }, body: JSON.stringify(payload),
      });
    },
    async patchSMSProfile(tenantId, version, payload) {
      return request(tenantPath(tenantId, '/sms-profile'), {
        method: 'PATCH', headers: { 'If-Match': `"${version}"` }, body: JSON.stringify(payload),
      });
    },
    async deleteSMSProfile(tenantId, version) {
      await request(tenantPath(tenantId, '/sms-profile'), {
        method: 'DELETE', headers: { 'If-Match': `"${version}"` },
      });
    },
    async rotateTenantCredential(tenantId, version, payload) {
      return request(tenantPath(tenantId, '/api-credential'), {
        method: 'PUT', headers: { 'If-Match': `"${version}"` }, body: JSON.stringify(payload),
      });
    },
    async listNotifications(statuses = [], tenantId = '', options = {}) {
      const query = new URLSearchParams();
      statuses.filter(Boolean).forEach((status) => query.append('status', String(status)));
      const search = typeof options.query === 'string' ? options.query.trim() : '';
      const cursor = typeof options.cursor === 'string' ? options.cursor.trim() : '';
      const limit = Number(options.limit || 0);
      if (search) query.set('q', search);
      if (cursor) query.set('cursor', cursor);
      if (Number.isInteger(limit) && limit > 0) query.set('limit', String(limit));
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const payload = await request(`${tenantPath(tenantId, '/notifications')}${suffix}`, { method: 'GET' });
      return {
        notifications: (Array.isArray(payload?.notifications) ? payload.notifications : []).map(mapNotification).filter(Boolean),
        nextCursor: typeof payload?.next_cursor === 'string' ? payload.next_cursor : '',
      };
    },
    async rescheduleNotification(notificationId, scheduledIsoString, tenantId) {
      return mapNotification(await request(tenantPath(tenantId, `/notifications/${encodeURIComponent(notificationId)}`), {
        method: 'PATCH', body: JSON.stringify({ scheduled_time: scheduledIsoString }),
      }));
    },
    async cancelNotification(notificationId, tenantId) {
      return mapNotification(await request(tenantPath(tenantId, `/notifications/${encodeURIComponent(notificationId)}`), {
        method: 'PATCH', body: JSON.stringify({ status: 'cancelled' }),
      }));
    },
    async listSMTPIdentities(tenantId) {
      const payload = await request(tenantPath(tenantId, '/smtp-identities'), { method: 'GET' });
      return (Array.isArray(payload?.identities) ? payload.identities : []).map(mapSMTPIdentity).filter(Boolean);
    },
    async listSMTPDomains(tenantId) {
      const payload = await request(tenantPath(tenantId, '/smtp-domains'), { method: 'GET' });
      return (Array.isArray(payload?.domains) ? payload.domains : []).map(mapSMTPSenderDomain).filter(Boolean);
    },
    async createSMTPDomain(tenantId, domain) {
      return mapSMTPSenderDomain(await request(tenantPath(tenantId, '/smtp-domains'), { method: 'POST', body: JSON.stringify({ domain }) }));
    },
    async checkSMTPDomainDNS(tenantId, domainId) {
      return mapSMTPSenderDomain(await request(tenantPath(tenantId, `/smtp-domains/${encodeURIComponent(domainId)}/dns-checks`), { method: 'POST' }));
    },
    async createSMTPIdentity(tenantId, emailAddress, forwardTo) {
      return mapSMTPCredentials(await request(tenantPath(tenantId, '/smtp-identities'), { method: 'POST', body: JSON.stringify({ email_address: emailAddress, forward_to: forwardTo }) }));
    },
    async updateSMTPIdentityForwarding(tenantId, identityId, forwardTo) {
      return mapSMTPIdentity(await request(tenantPath(tenantId, `/smtp-identities/${encodeURIComponent(identityId)}`), { method: 'PATCH', body: JSON.stringify({ forward_to: forwardTo }) }));
    },
    async getSMTPIdentityCredentials(tenantId, identityId) {
      return mapSMTPCredentials(await request(tenantPath(tenantId, `/smtp-identities/${encodeURIComponent(identityId)}/credential`), { method: 'GET' }));
    },
    async rotateSMTPIdentity(tenantId, identityId) {
      return mapSMTPCredentials(await request(tenantPath(tenantId, `/smtp-identities/${encodeURIComponent(identityId)}/credential`), { method: 'PUT' }));
    },
    async deleteSMTPIdentity(tenantId, identityId) {
      await request(tenantPath(tenantId, `/smtp-identities/${encodeURIComponent(identityId)}`), { method: 'DELETE' });
    },
  };
}
