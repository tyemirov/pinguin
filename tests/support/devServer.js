#!/usr/bin/env node
// @ts-check
const http = require('http');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const HOST = '127.0.0.1';
const PORT = process.env.PLAYWRIGHT_PORT ? Number(process.env.PLAYWRIGHT_PORT) : 4174;
const WEB_ROOT = path.resolve(__dirname, '../../web');
const AUTH_COOKIE = 'pinguin_playwright_auth';
const PLAYWRIGHT_AVATAR_URL =
  'data:image/svg+xml,%3Csvg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 40 40"%3E%3Crect width="40" height="40" rx="20" fill="%232563eb"/%3E%3Ctext x="20" y="25" text-anchor="middle" font-size="16" font-family="Arial" fill="white"%3EP%3C/text%3E%3C/svg%3E';
const AUTH_PROFILE = {
  user_email: 'playwright@example.com',
  user_display_name: 'Playwright User',
  user_avatar_url: PLAYWRIGHT_AVATAR_URL,
  display: 'Playwright User',
  given_name: 'Playwright',
  avatar_url: PLAYWRIGHT_AVATAR_URL,
};
const runtimeConfig = {
  tauthBaseUrl:
    process.env.PLAYWRIGHT_TAUTH_BASE_URL || `http://${HOST}:${PORT}`,
  tauthTenantId: 'tauth-devserver',
  googleClientId: 'playwright-client',
  apiBaseUrl: `http://${HOST}:${PORT}/api`,
  eventLogUrl: '/event-log.html',
  smtpRelayUrl: '/smtp-relay.html',
  tenant: {
    id: 'tenant-devserver',
    displayName: 'Dev Server Tenant',
  },
};

const shouldLog = process.env.PLAYWRIGHT_DEVSERVER_LOGS === '1';
const log = (...args) => {
  if (shouldLog) {
    console.log(...args);
  }
};

const swallowEpipe = (stream) => {
  if (!stream) {
    return;
  }
  stream.on('error', (error) => {
    if (error && error.code === 'EPIPE') {
      return;
    }
    throw error;
  });
};

swallowEpipe(process.stdout);
swallowEpipe(process.stderr);

const handleFatal = (label, error) => {
  if (error instanceof Error) {
    console.error(label, error.stack || error.message);
  } else {
    console.error(label, error);
  }
  process.exit(1);
};

process.on('uncaughtException', (error) => {
  handleFatal('devServer uncaughtException', error);
});

process.on('unhandledRejection', (error) => {
  handleFatal('devServer unhandledRejection', error);
});

let serverState = createDefaultState();
const nonceStore = new Map();
const NONCE_TTL_MS = 2 * 60 * 1000;

function createDefaultState() {
  return {
    notifications: defaultNotifications(),
    tenants: defaultTenants(),
    smtpIdentities: [],
    failList: false,
    failReschedule: false,
    failCancel: false,
    failSMTPList: false,
    failSMTPCreate: false,
    failSMTPForwardingUpdate: false,
    failSMTPRotate: false,
    failSMTPDelete: false,
  };
}

function defaultTenants() {
  return [
    { id: 'tenant-devserver', displayName: 'Dev Server Tenant' },
    { id: 'tenant-archive', displayName: 'Archive Tenant' },
  ];
}

function defaultNotifications() {
  const now = new Date();
  return [
    {
      notification_id: 'notif-1',
      tenant_id: 'tenant-devserver',
      notification_type: 'email',
      recipient: 'user@example.com',
      subject: 'Queued notification',
      message: 'Hello from tests',
      status: 'queued',
      created_at: now.toISOString(),
      updated_at: now.toISOString(),
      scheduled_for: new Date(now.getTime() + 3600 * 1000).toISOString(),
      retry_count: 0,
    },
  ];
}

function applyOverrides(payload) {
  if (Array.isArray(payload.notifications) && payload.notifications.length > 0) {
    serverState.notifications = payload.notifications.map((item) => ({
      ...item,
      tenant_id: item.tenant_id || 'tenant-devserver',
      scheduled_for: item.scheduled_for || item.scheduled_time || null,
    }));
  } else {
    serverState.notifications = defaultNotifications();
  }
  serverState.tenants = Array.isArray(payload.tenants) && payload.tenants.length > 0
    ? payload.tenants.map((item) => ({
        id: item.id || item.tenant_id || 'tenant-devserver',
        displayName: item.displayName || item.display_name || item.id || 'Tenant',
      }))
    : defaultTenants();
  serverState.smtpIdentities = Array.isArray(payload.smtpIdentities)
    ? payload.smtpIdentities.map((item) => ({
        ...item,
        forward_to: Array.isArray(item.forward_to) ? item.forward_to : ['owner@example.com'],
      }))
    : [];
  serverState.failList = Boolean(payload.failList);
  serverState.failReschedule = Boolean(payload.failReschedule);
  serverState.failCancel = Boolean(payload.failCancel);
  serverState.failSMTPList = Boolean(payload.failSMTPList);
  serverState.failSMTPCreate = Boolean(payload.failSMTPCreate);
  serverState.failSMTPForwardingUpdate = Boolean(payload.failSMTPForwardingUpdate);
  serverState.failSMTPRotate = Boolean(payload.failSMTPRotate);
  serverState.failSMTPDelete = Boolean(payload.failSMTPDelete);
  nonceStore.clear();
}

function readJson(req) {
  return new Promise((resolve) => {
    let data = '';
    req.on('data', (chunk) => {
      data += chunk;
    });
    req.on('end', () => {
      try {
        resolve(JSON.parse(data || '{}'));
      } catch (error) {
        resolve({});
      }
    });
  });
}

function sendJson(res, statusCode, body, headers = {}) {
  res.writeHead(statusCode, {
    'Content-Type': 'application/json',
    'Cache-Control': 'no-store',
    ...headers,
  });
  res.end(body ? JSON.stringify(body) : undefined);
}

function hasAuthCookie(req) {
  const cookieHeader = req.headers.cookie || '';
  return cookieHeader.split(';').some((entry) => entry.trim() === `${AUTH_COOKIE}=1`);
}

function authCookieHeader() {
  return `${AUTH_COOKIE}=1; Path=/; SameSite=Lax`;
}

function expiredAuthCookieHeader() {
  return `${AUTH_COOKIE}=; Path=/; Max-Age=0; SameSite=Lax`;
}

function serveStatic(filePath, res) {
  fs.readFile(filePath, (error, data) => {
    if (error) {
      res.writeHead(404);
      res.end('Not found');
      return;
    }
    const ext = path.extname(filePath).toLowerCase();
    const contentType =
      {
        '.html': 'text/html; charset=utf-8',
        '.js': 'text/javascript; charset=utf-8',
        '.css': 'text/css; charset=utf-8',
        '.svg': 'image/svg+xml',
        '.json': 'application/json',
      }[ext] || 'application/octet-stream';
    res.writeHead(200, { 'Content-Type': contentType });
    res.end(data);
  });
}

const server = http.createServer(async (req, res) => {
  const url = new URL(req.url, `http://${req.headers.host}`);
  log('devServer request', req.method, url.pathname);
  if (req.method === 'GET' && url.pathname === '/runtime-config') {
    log('devServer: served runtime config');
    sendJson(res, 200, runtimeConfig);
    return;
  }
  if (req.method === 'POST' && url.pathname === '/testing/reset') {
    const body = await readJson(req);
    applyOverrides(body || {});
    sendJson(res, 204, null);
    return;
  }

  if (req.method === 'GET' && url.pathname === '/api/notifications') {
    if (serverState.failList) {
      sendJson(res, 500, { error: 'list_failed' });
      return;
    }
    const statuses = url.searchParams.getAll('status').filter(Boolean);
    const tenantId = url.searchParams.get('tenant_id') || '';
    if (!tenantId.trim()) {
      sendJson(res, 400, { error: 'tenant_id is required' });
      return;
    }
    const searchQuery = url.searchParams.get('q') || '';
    const filtered = filterNotifications(serverState.notifications, statuses, tenantId, searchQuery);
    const page = paginateNotifications(filtered, url.searchParams);
    sendJson(res, 200, { notifications: page.notifications, next_cursor: page.nextCursor });
    return;
  }

  if (req.method === 'GET' && url.pathname === '/api/tenants') {
    sendJson(res, 200, { tenants: serverState.tenants });
    return;
  }

  const scheduleMatch = url.pathname.match(/^\/api\/notifications\/([^/]+)\/schedule$/);
  if (scheduleMatch && req.method === 'PATCH') {
    const tenantId = url.searchParams.get('tenant_id') || '';
    if (!tenantId.trim()) {
      sendJson(res, 400, { error: 'tenant_id is required' });
      return;
    }
    if (serverState.failReschedule) {
      sendJson(res, 500, { error: 'reschedule_failed' });
      return;
    }
    const body = await readJson(req);
    const scheduled_for = body.scheduled_for || body.scheduled_time || null;
    serverState.notifications = serverState.notifications.map((item) => {
      if (item.notification_id === scheduleMatch[1]) {
        return { ...item, scheduled_for, status: 'queued', updated_at: new Date().toISOString() };
      }
      return item;
    });
    const updated = serverState.notifications.find((item) => item.notification_id === scheduleMatch[1]);
    sendJson(res, 200, updated || {});
    return;
  }

  const cancelMatch = url.pathname.match(/^\/api\/notifications\/([^/]+)\/cancel$/);
  if (cancelMatch && req.method === 'POST') {
    const tenantId = url.searchParams.get('tenant_id') || '';
    if (!tenantId.trim()) {
      sendJson(res, 400, { error: 'tenant_id is required' });
      return;
    }
    if (serverState.failCancel) {
      sendJson(res, 500, { error: 'cancel_failed' });
      return;
    }
    serverState.notifications = serverState.notifications.map((item) => {
      if (item.notification_id === cancelMatch[1]) {
        return { ...item, status: 'cancelled', updated_at: new Date().toISOString() };
      }
      return item;
    });
    const updated = serverState.notifications.find((item) => item.notification_id === cancelMatch[1]);
    sendJson(res, 200, updated || {});
    return;
  }

  if (req.method === 'GET' && url.pathname === '/api/smtp-identities') {
    if (serverState.failSMTPList) {
      sendJson(res, 500, { error: 'smtp_identity_list_failed' });
      return;
    }
    sendJson(res, 200, { identities: serverState.smtpIdentities });
    return;
  }

  if (req.method === 'POST' && url.pathname === '/api/smtp-identities') {
    if (serverState.failSMTPCreate) {
      sendJson(res, 500, { error: 'smtp_identity_create_failed' });
      return;
    }
    const body = await readJson(req);
    const emailAddress = typeof body.email_address === 'string' ? body.email_address.trim() : '';
    const forwardTo = Array.isArray(body.forward_to)
      ? body.forward_to.map((item) => String(item).trim()).filter(Boolean)
      : [];
    if (!emailAddress || forwardTo.length === 0) {
      sendJson(res, 400, { error: 'email_address is invalid' });
      return;
    }
    const identity = newSMTPIdentity(emailAddress, forwardTo);
    serverState.smtpIdentities.push(identity);
    sendJson(res, 201, credentialsForIdentity(identity));
    return;
  }

  const updateSMTPForwardingMatch = url.pathname.match(/^\/api\/smtp-identities\/([^/]+)\/forwarding$/);
  if (updateSMTPForwardingMatch && req.method === 'PATCH') {
    if (serverState.failSMTPForwardingUpdate) {
      sendJson(res, 500, { error: 'smtp_identity_forwarding_update_failed' });
      return;
    }
    const identity = serverState.smtpIdentities.find((item) => item.id === updateSMTPForwardingMatch[1]);
    if (!identity) {
      sendJson(res, 404, { error: 'smtp identity not found' });
      return;
    }
    const body = await readJson(req);
    const forwardTo = Array.isArray(body.forward_to)
      ? body.forward_to.map((item) => String(item).trim()).filter(Boolean)
      : [];
    if (forwardTo.length === 0) {
      sendJson(res, 400, { error: 'forward_to is required' });
      return;
    }
    identity.forward_to = forwardTo;
    identity.updated_at = new Date().toISOString();
    sendJson(res, 200, identity);
    return;
  }

  const rotateSMTPMatch = url.pathname.match(/^\/api\/smtp-identities\/([^/]+)\/rotate$/);
  if (rotateSMTPMatch && req.method === 'POST') {
    if (serverState.failSMTPRotate) {
      sendJson(res, 500, { error: 'smtp_identity_rotate_failed' });
      return;
    }
    const identity = serverState.smtpIdentities.find((item) => item.id === rotateSMTPMatch[1]);
    if (!identity) {
      sendJson(res, 404, { error: 'smtp identity not found' });
      return;
    }
    identity.username = `smtp_rotated_JAYbQkNwQvT-LZI${serverState.smtpIdentities.length}`;
    identity.updated_at = new Date().toISOString();
    sendJson(res, 200, credentialsForIdentity(identity, 'pgsmtp_rotated_UVSZ9mxDW6ZeV-tNwApoddcyCjOM5uA'));
    return;
  }

  const deleteSMTPMatch = url.pathname.match(/^\/api\/smtp-identities\/([^/]+)$/);
  if (deleteSMTPMatch && req.method === 'DELETE') {
    if (serverState.failSMTPDelete) {
      sendJson(res, 500, { error: 'smtp_identity_delete_failed' });
      return;
    }
    serverState.smtpIdentities = serverState.smtpIdentities.filter((item) => item.id !== deleteSMTPMatch[1]);
    sendJson(res, 204, null);
    return;
  }

  if (url.pathname === '/auth/nonce' && req.method === 'POST') {
    const token = issueNonce();
    sendJson(res, 200, { nonce: token });
    return;
  }

  if (url.pathname === '/me' && req.method === 'GET') {
    if (!hasAuthCookie(req)) {
      sendJson(res, 401, { error: 'session_required' });
      return;
    }
    sendJson(res, 200, AUTH_PROFILE);
    return;
  }

  if (url.pathname === '/auth/google' && req.method === 'POST') {
    const body = await readJson(req);
    if (!body || typeof body.google_id_token !== 'string' || !body.google_id_token.trim()) {
      sendJson(res, 400, { error: 'invalid_google_token' });
      return;
    }
    const nonceToken = typeof body.nonce_token === 'string' ? body.nonce_token.trim() : '';
    if (!consumeNonce(nonceToken)) {
      sendJson(res, 401, { error: 'invalid_nonce' });
      return;
    }
    sendJson(res, 200, AUTH_PROFILE, { 'Set-Cookie': authCookieHeader() });
    return;
  }

  if (url.pathname === '/auth/refresh' && req.method === 'POST') {
    if (!hasAuthCookie(req)) {
      sendJson(res, 401, { error: 'session_required' });
      return;
    }
    sendJson(res, 204, null);
    return;
  }

  if (url.pathname === '/auth/logout' && req.method === 'POST') {
    sendJson(res, 204, null, { 'Set-Cookie': expiredAuthCookieHeader() });
    return;
  }

  // Serve static files from /web
  let relativePath = url.pathname;
  if (relativePath === '/' || relativePath === '') {
    relativePath = '/index.html';
  }
  const safePath = path.normalize(relativePath).replace(/^\.\/+/, '');
  const filePath = path.join(WEB_ROOT, safePath);
  serveStatic(filePath, res);
});

server.on('error', (error) => {
  handleFatal('devServer listen error', error);
});

function issueNonce() {
  const token = crypto.randomBytes(16).toString('hex');
  nonceStore.set(token, Date.now() + NONCE_TTL_MS);
  return token;
}

function consumeNonce(token) {
  if (!token) {
    return false;
  }
  const expiry = nonceStore.get(token);
  if (!expiry) {
    purgeNonces();
    return false;
  }
  nonceStore.delete(token);
  if (Date.now() > expiry) {
    purgeNonces();
    return false;
  }
  purgeNonces();
  return true;
}

function purgeNonces() {
  if (!nonceStore.size) {
    return;
  }
  const now = Date.now();
  for (const [token, expiry] of nonceStore.entries()) {
    if (now > expiry) {
      nonceStore.delete(token);
    }
  }
}

function filterNotifications(source, statuses, tenantId, searchQuery = '') {
  if (!Array.isArray(source) || source.length === 0) {
    return [];
  }
  const normalizedTenantId = String(tenantId || '').trim();
  let filtered = normalizedTenantId
    ? source.filter((item) => item.tenant_id === normalizedTenantId)
    : source;
  if (statuses && statuses.length > 0) {
    const wanted = new Set(statuses.map((value) => String(value).toLowerCase()));
    filtered = filtered.filter((item) => wanted.has(String(item.status).toLowerCase()));
  }
  const normalizedQuery = String(searchQuery).trim().toLowerCase();
  if (!normalizedQuery) {
    return filtered;
  }
  return filtered.filter((item) => notificationSearchFields(item).some((value) => value.includes(normalizedQuery)));
}

function notificationSearchFields(item) {
  return [
    item.notification_id,
    item.tenant_id,
    item.notification_type,
    item.status,
    item.recipient,
    item.subject,
    item.message,
  ].map((value) => String(value || '').toLowerCase());
}

function paginateNotifications(source, searchParams) {
  const parsedLimit = Number(searchParams.get('limit') || source.length || 0);
  const limit = Number.isInteger(parsedLimit) && parsedLimit > 0 ? parsedLimit : source.length;
  const parsedCursor = Number(searchParams.get('cursor') || 0);
  const startIndex = Number.isInteger(parsedCursor) && parsedCursor > 0 ? parsedCursor : 0;
  const endIndex = startIndex + limit;
  return {
    notifications: source.slice(startIndex, endIndex),
    nextCursor: endIndex < source.length ? String(endIndex) : '',
  };
}

function newSMTPIdentity(emailAddress, forwardTo = ['owner@example.com']) {
  const now = new Date().toISOString();
  const identityIndex = serverState.smtpIdentities.length + 1;
  return {
    id: `smtp-id-${identityIndex}`,
    email_address: emailAddress,
    username: `smtp_JAYbQkNwQvT-LZI${identityIndex}`,
    forward_to: forwardTo,
    status: 'active',
    last_used_at: null,
    created_at: now,
    updated_at: now,
  };
}

function credentialsForIdentity(identity, password = 'pgsmtp_UVSZ9mxDW6ZeV-tNwApoddcyCjOM5uA') {
  return {
    identity,
    smtp_settings: {
      host: 'smtp.pinguin.test',
      port: 465,
      security_mode: 'ssl',
    },
    username: identity.username,
    password,
  };
}

server.listen(PORT, HOST, () => {
  log(`Playwright test server listening on http://${HOST}:${PORT}`);
});
