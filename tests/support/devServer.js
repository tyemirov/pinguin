#!/usr/bin/env node
// @ts-check
const http = require('http');
const fs = require('fs');
const path = require('path');
const crypto = require('crypto');

const HOST = '127.0.0.1';
const PORT = process.env.PLAYWRIGHT_PORT ? Number(process.env.PLAYWRIGHT_PORT) : 4174;
const WEB_ROOT = path.resolve(__dirname, '../../web');
const TAUTH_HELPER_PATH = path.resolve(__dirname, './stubs/auth-client.js');
const runtimeConfig = {
  tauthBaseUrl:
    process.env.PLAYWRIGHT_TAUTH_BASE_URL || `http://${HOST}:${PORT}`,
  tauthTenantId: 'tauth-devserver',
  googleClientId: 'playwright-client',
  apiBaseUrl: `http://${HOST}:${PORT}/api`,
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
    failList: false,
    failReschedule: false,
    failCancel: false,
  };
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
  serverState.failList = Boolean(payload.failList);
  serverState.failReschedule = Boolean(payload.failReschedule);
  serverState.failCancel = Boolean(payload.failCancel);
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
    const filtered = filterNotifications(serverState.notifications, statuses);
    sendJson(res, 200, { notifications: filtered });
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

  if (url.pathname === '/auth/nonce' && req.method === 'POST') {
    const token = issueNonce();
    sendJson(res, 200, { nonce: token });
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
    sendJson(res, 200, {
      profile: {
        user_email: 'playwright@example.com',
        user_display_name: 'Playwright User',
      },
    });
    return;
  }

  if (url.pathname === '/auth/refresh' && req.method === 'POST') {
    sendJson(res, 204, null);
    return;
  }

  if (url.pathname === '/tauth.js') {
    serveStatic(TAUTH_HELPER_PATH, res);
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

function filterNotifications(source, statuses) {
  if (!Array.isArray(source) || source.length === 0) {
    return [];
  }
  if (!statuses || statuses.length === 0) {
    return source;
  }
  const wanted = new Set(statuses.map((value) => String(value).toLowerCase()));
  return source.filter((item) => wanted.has(String(item.status).toLowerCase()));
}

server.listen(PORT, HOST, () => {
  log(`Playwright test server listening on http://${HOST}:${PORT}`);
});
