# github.com/tyemirov/tauth (v0.9.8)

## index.html

<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <meta
      name="description"
      content="TAuth verifies Google tokens, mints first-party JWT cookies, and keeps refresh rotation server-side for single-origin apps."
    />
    <title>TAuth | Own your Google Sign-In sessions</title>
    <link rel="preconnect" href="https://fonts.googleapis.com" />
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin />
    <link
      href="https://fonts.googleapis.com/css2?family=Space+Mono:wght@400;700&family=Sora:wght@400;500;600;700&display=swap"
      rel="stylesheet"
    />
    <style>
      :root {
        --ink-100: #f3f8ff;
        --ink-70: #c1ccda;
        --ink-40: #8292a6;
        --background: #07090d;
        --surface: #0f141c;
        --surface-strong: #151d28;
        --accent: #5ef6ff;
        --accent-alt: #b6ff6a;
        --accent-warm: #ff985f;
        --grid-line: rgba(255, 255, 255, 0.08);
        --shadow: 0 22px 60px rgba(4, 8, 12, 0.55);
        --font-heading: "Space Mono", ui-monospace, SFMono-Regular, Menlo,
          Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
        --font-body: "Sora", "Segoe UI", "Helvetica Neue", Arial, sans-serif;
      }

      * {
        box-sizing: border-box;
      }

      body {
        margin: 0;
        font-family: var(--font-body);
        color: var(--ink-100);
        background: radial-gradient(
            circle at 12% 18%,
            rgba(94, 246, 255, 0.14),
            transparent 45%
          ),
          radial-gradient(
            circle at 90% 10%,
            rgba(255, 152, 95, 0.12),
            transparent 45%
          ),
          radial-gradient(
            circle at 80% 85%,
            rgba(182, 255, 106, 0.12),
            transparent 50%
          ),
          var(--background);
        min-height: 100vh;
        line-height: 1.6;
        overflow-x: hidden;
      }

      body::before,
      body::after {
        content: "";
        position: fixed;
        inset: 0;
        pointer-events: none;
        z-index: -1;
      }

      body::before {
        background-image: linear-gradient(
            rgba(255, 255, 255, 0.03) 1px,
            transparent 1px
          ),
          linear-gradient(90deg, rgba(255, 255, 255, 0.03) 1px, transparent 1px);
        background-size: 120px 120px;
        opacity: 0.35;
      }

      body::after {
        background: radial-gradient(
          circle at 70% 20%,
          rgba(94, 246, 255, 0.12),
          transparent 60%
        );
        opacity: 0.7;
      }

      a {
        color: inherit;
        text-decoration: none;
      }

      .skip-link {
        position: absolute;
        left: -999px;
        top: 0;
        background: var(--accent);
        color: #07090d;
        padding: 8px 14px;
        border-radius: 999px;
        z-index: 20;
      }

      .skip-link:focus {
        left: 20px;
        top: 20px;
      }

      .page {
        position: relative;
        padding: 28px 6vw 96px;
        max-width: 1200px;
        margin: 0 auto;
      }

      .site-header {
        display: flex;
        align-items: center;
        justify-content: space-between;
        gap: 24px;
        padding: 16px 0 32px;
      }

      .logo {
        font-family: var(--font-heading);
        font-weight: 700;
        font-size: 20px;
        letter-spacing: 0.08em;
        text-transform: uppercase;
      }

      .nav {
        display: flex;
        gap: 18px;
        font-size: 13px;
        text-transform: uppercase;
        letter-spacing: 0.12em;
        color: var(--ink-40);
      }

      .nav a {
        padding-bottom: 4px;
        border-bottom: 1px solid transparent;
      }

      .nav a:hover {
        border-color: var(--accent);
        color: var(--ink-100);
      }

      .cta {
        padding: 12px 20px;
        border-radius: 999px;
        font-weight: 600;
        font-size: 14px;
        display: inline-flex;
        align-items: center;
        gap: 8px;
        transition: transform 0.2s ease, box-shadow 0.2s ease;
      }

      .cta-primary {
        background: var(--accent);
        color: #05070a;
        box-shadow: 0 14px 30px rgba(94, 246, 255, 0.3);
      }

      .cta-secondary {
        border: 1px solid var(--grid-line);
        color: var(--ink-100);
        background: rgba(15, 20, 28, 0.7);
      }

      .cta:hover {
        transform: translateY(-2px);
      }

      .hero {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
        gap: 48px;
        align-items: center;
        padding: 20px 0 64px;
      }

      .eyebrow {
        text-transform: uppercase;
        font-size: 12px;
        letter-spacing: 0.28em;
        color: var(--ink-40);
      }

      h1,
      h2,
      h3 {
        font-family: var(--font-heading);
        font-weight: 700;
        letter-spacing: 0.02em;
        margin: 0 0 16px;
      }

      h1 {
        font-size: clamp(34px, 5vw, 56px);
        line-height: 1.1;
      }

      h2 {
        font-size: clamp(24px, 4vw, 34px);
      }

      h3 {
        font-size: 18px;
      }

      .lead {
        color: var(--ink-70);
        font-size: 18px;
        max-width: 520px;
      }

      .hero-actions {
        display: flex;
        flex-wrap: wrap;
        gap: 12px;
        margin-top: 24px;
      }

      .hero-metrics {
        margin-top: 28px;
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(160px, 1fr));
        gap: 14px;
      }

      .metric {
        padding: 14px 16px;
        border-radius: 14px;
        background: rgba(15, 20, 28, 0.75);
        border: 1px solid rgba(255, 255, 255, 0.08);
      }

      .metric-label {
        display: block;
        font-size: 12px;
        text-transform: uppercase;
        letter-spacing: 0.16em;
        color: var(--ink-40);
      }

      .metric-value {
        display: block;
        font-family: var(--font-heading);
        margin-top: 6px;
        font-size: 14px;
      }

      .panel {
        background: linear-gradient(135deg, rgba(21, 29, 40, 0.95), rgba(9, 12, 18, 0.9));
        border: 1px solid rgba(255, 255, 255, 0.08);
        border-radius: 22px;
        box-shadow: var(--shadow);
      }

      .hero-visual {
        padding: 24px;
      }

      .terminal {
        padding: 20px;
        border-radius: 16px;
        background: rgba(7, 10, 16, 0.9);
        border: 1px solid rgba(255, 255, 255, 0.08);
        font-family: var(--font-heading);
        font-size: 13px;
        color: var(--ink-100);
      }

      .terminal-header {
        display: flex;
        align-items: center;
        gap: 6px;
        margin-bottom: 14px;
        color: var(--ink-40);
        text-transform: uppercase;
        letter-spacing: 0.12em;
        font-size: 11px;
      }

      .terminal-dot {
        width: 9px;
        height: 9px;
        border-radius: 50%;
        background: var(--accent);
      }

      pre {
        margin: 0;
        white-space: pre-wrap;
        word-break: break-word;
        color: var(--ink-70);
        font-family: var(--font-heading);
      }

      code {
        font-family: var(--font-heading);
        color: var(--accent);
      }

      .feature-grid {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
        gap: 24px;
        margin: 40px 0 70px;
      }

      .feature-card {
        padding: 22px;
        border-radius: 18px;
        background: rgba(15, 20, 28, 0.8);
        border: 1px solid rgba(255, 255, 255, 0.06);
      }

      .feature-card svg {
        width: 26px;
        height: 26px;
        color: var(--accent-alt);
        margin-bottom: 14px;
      }

      .section {
        margin-top: 84px;
      }

      .split {
        display: grid;
        grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
        gap: 36px;
        align-items: center;
      }

      .section-copy {
        color: var(--ink-70);
        font-size: 16px;
      }

      .code-card {
        padding: 24px;
        border-radius: 18px;
        background: rgba(12, 16, 23, 0.9);
        border: 1px solid rgba(94, 246, 255, 0.22);
        box-shadow: 0 20px 40px rgba(0, 0, 0, 0.35);
      }

      .palette-grid {
        display: grid;
        gap: 18px;
      }

      .palette-card {
        padding: 20px;
        border-radius: 16px;
        background: rgba(15, 20, 28, 0.8);
        border: 1px solid rgba(255, 255, 255, 0.08);
      }

      .swatch-row {
        display: flex;
        gap: 10px;
        margin-bottom: 12px;
      }

      .swatch {
        width: 22px;
        height: 22px;
        border-radius: 8px;
        background: var(--swatch);
        border: 1px solid rgba(255, 255, 255, 0.2);
      }

      .docs-links {
        display: grid;
        gap: 12px;
        margin-top: 18px;
      }

      .docs-link {
        padding: 14px 18px;
        border-radius: 14px;
        background: rgba(15, 20, 28, 0.7);
        border: 1px solid rgba(255, 255, 255, 0.08);
        display: flex;
        justify-content: space-between;
        align-items: center;
        gap: 12px;
      }

      .site-footer {
        margin-top: 90px;
        padding: 36px 24px 48px;
        border-top: 1px solid var(--grid-line);
        background: rgba(7, 9, 13, 0.85);
      }

      .site-footer__inner {
        max-width: 1180px;
        margin: 0 auto;
        display: flex;
        flex-wrap: wrap;
        align-items: center;
        justify-content: space-between;
        gap: 18px;
      }

      .site-footer__brand {
        color: var(--ink-70);
        font-size: 0.95rem;
      }

      .site-footer__links {
        display: flex;
        flex-wrap: wrap;
        gap: 18px;
      }

      .site-footer__links a {
        color: var(--ink-70);
        text-decoration: none;
        font-weight: 600;
      }

      .site-footer__links a:hover {
        color: var(--ink-100);
      }

      .reveal {
        animation: fadeUp 0.8s ease forwards;
        opacity: 0;
      }

      .reveal.delay-1 {
        animation-delay: 0.1s;
      }

      .reveal.delay-2 {
        animation-delay: 0.2s;
      }

      .reveal.delay-3 {
        animation-delay: 0.3s;
      }

      @keyframes fadeUp {
        from {
          opacity: 0;
          transform: translateY(18px);
        }
        to {
          opacity: 1;
          transform: translateY(0);
        }
      }

      @media (max-width: 720px) {
        .site-header {
          flex-direction: column;
          align-items: flex-start;
        }

        .nav {
          flex-wrap: wrap;
        }
      }

      @media (prefers-reduced-motion: reduce) {
        .reveal {
          animation: none;
          opacity: 1;
        }

        .cta {
          transition: none;
        }
      }
    </style>
  </head>
  <body>
    <a class="skip-link" href="#main">Skip to content</a>
    <div class="page">
      <header class="site-header">
        <div class="logo">TAuth</div>
        <nav class="nav" aria-label="Primary">
          <a href="#features">Features</a>
          <a href="#blueprint">Blueprint</a>
          <a href="#deep-dive">Deep dive</a>
          <a href="#docs">Docs</a>
        </nav>
        <a class="cta cta-primary" href="usage.html">Get started</a>
      </header>

      <main id="main">
        <section class="hero">
          <div class="hero-copy reveal delay-1">
            <div class="eyebrow">Google Identity Services, owned sessions</div>
            <h1>Own the session. Keep tokens off the browser.</h1>
            <p class="lead">
              TAuth verifies Google credentials, mints first-party JWT cookies,
              and rotates refresh tokens server-side. One origin, zero token
              storage, and a multi-tenant config your platform team can trust.
            </p>
            <div class="hero-actions">
              <a class="cta cta-primary" href="usage.html">Get started</a>
              <a class="cta cta-secondary" href="https://github.com/tyemirov/TAuth"
                >View on GitHub</a
              >
            </div>
            <div class="hero-metrics">
              <div class="metric">
                <span class="metric-label">Session model</span>
                <span class="metric-value">JWT cookies</span>
              </div>
              <div class="metric">
                <span class="metric-label">Scope</span>
                <span class="metric-value">Single origin</span>
              </div>
              <div class="metric">
                <span class="metric-label">Mode</span>
                <span class="metric-value">Multi-tenant</span>
              </div>
            </div>
          </div>
          <div class="hero-visual panel reveal delay-2">
            <div class="terminal">
              <div class="terminal-header">
                <span class="terminal-dot"></span>
                <span class="terminal-dot" style="background: var(--accent-warm)"></span>
                <span class="terminal-dot" style="background: #7aa6ff"></span>
                launch
              </div>
              <pre><code>$ tauth --config=config.yaml
/auth/nonce  -> /auth/google
/auth/refresh -> /auth/logout
/me -> profile
</code></pre>
            </div>
          </div>
        </section>

        <section id="features" class="feature-grid">
          <article class="feature-card reveal delay-1">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
              <path d="M4 6h16v12H4z"></path>
              <path d="M8 10h8M8 14h6"></path>
            </svg>
            <h3>First-party cookies</h3>
            <p>
              Access sessions live in HttpOnly cookies, not local storage.
              SameSite rules are derived per tenant.
            </p>
          </article>
          <article class="feature-card reveal delay-2">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
              <path d="M12 3l7 4v10l-7 4-7-4V7z"></path>
              <path d="M12 7v10"></path>
            </svg>
            <h3>Tenant aware</h3>
            <p>
              Host multiple products with tenant-specific cookies, issuers, and
              refresh TTLs in one file.
            </p>
          </article>
          <article class="feature-card reveal delay-3">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
              <path d="M4 12h16"></path>
              <path d="M12 4v16"></path>
            </svg>
            <h3>Preflight validation</h3>
            <p>
              Emit a redacted config report so orchestrators can validate
              secrets and endpoints before launch.
            </p>
          </article>
          <article class="feature-card reveal delay-3">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.6">
              <path d="M6 8h12M6 12h12M6 16h8"></path>
            </svg>
            <h3>Drop-in client</h3>
            <p>
              Use hosted tauth.js for nonce exchange, refresh retries, and logout
              state without custom wiring.
            </p>
          </article>
        </section>

        <section id="blueprint" class="section">
          <div class="split">
            <div class="reveal delay-1">
              <h2>Blueprint the landing story</h2>
              <p class="section-copy">
                The page stacks a bold hero, value props, and deep dives that
                read like a platform spec. Every section supports an operator
                decision in under a minute.
              </p>
            </div>
            <div class="code-card reveal delay-2">
              <pre><code>Structure:
- Hero + CLI snapshot
- Value props grid
- Auth exchange, JWT validation, tenant config
- Palette suggestions
- Get started links
</code></pre>
            </div>
          </div>
        </section>

        <section id="deep-dive" class="section">
          <div class="split">
            <div class="reveal delay-1">
              <h2>Nonce to cookie exchange</h2>
              <p class="section-copy">
                Clients request a nonce, post the Google credential, then rely
                on the signed cookie for everything else. Refresh and logout
                stay server-only.
              </p>
            </div>
            <div class="code-card reveal delay-2">
              <pre><code>POST /auth/nonce
POST /auth/google
POST /auth/refresh
POST /auth/logout
GET  /me
</code></pre>
            </div>
          </div>
        </section>

        <section class="section">
          <div class="split">
            <div class="reveal delay-1">
              <h2>JWT validation for every service</h2>
              <p class="section-copy">
                Downstream Go services validate app_session cookies with the same
                tenant config, so issuers and cookie names stay aligned.
              </p>
            </div>
            <div class="code-card reveal delay-2">
              <pre><code>validator, err := sessionvalidator.New(
  sessionvalidator.Config{
    SigningKey: signingKey,
    Issuer: "tauth",
  },
)
</code></pre>
            </div>
          </div>
        </section>

        <section class="section">
          <div class="split">
            <div class="reveal delay-1">
              <h2>Tenant config as a contract</h2>
              <p class="section-copy">
                One YAML file defines tenant origins, cookie names, and TTLs.
                Resolve by Origin or explicit header when you share an origin.
              </p>
            </div>
            <div class="code-card reveal delay-2">
              <pre><code>tenants:
  - id: "notes"
    tenant_origins: ["https://notes.localhost"]
    google_web_client_id: "..."
    jwt_signing_key: "..."
    session_cookie_name: "app_session_notes"
</code></pre>
            </div>
          </div>
        </section>

        <section id="palette" class="section">
          <div class="split">
            <div class="reveal delay-1">
              <h2>Palette suggestions</h2>
              <p class="section-copy">
                Use a neon accent on deep charcoal for dark mode, or flip to a
                light mist palette for marketing docs and release notes.
              </p>
            </div>
            <div class="palette-grid reveal delay-2">
              <div class="palette-card">
                <h3>Dark baseline</h3>
                <div class="swatch-row">
                  <span class="swatch" style="--swatch: #07090d"></span>
                  <span class="swatch" style="--swatch: #0f141c"></span>
                  <span class="swatch" style="--swatch: #151d28"></span>
                  <span class="swatch" style="--swatch: #5ef6ff"></span>
                  <span class="swatch" style="--swatch: #b6ff6a"></span>
                </div>
                <pre><code>--bg: #07090d
--surface: #0f141c
--ink: #f3f8ff
--accent: #5ef6ff
--accent-2: #b6ff6a
</code></pre>
              </div>
              <div class="palette-card">
                <h3>Light baseline</h3>
                <div class="swatch-row">
                  <span class="swatch" style="--swatch: #f7f8fb"></span>
                  <span class="swatch" style="--swatch: #e6ebf2"></span>
                  <span class="swatch" style="--swatch: #121826"></span>
                  <span class="swatch" style="--swatch: #009bb8"></span>
                  <span class="swatch" style="--swatch: #5a7d2a"></span>
                </div>
                <pre><code>--bg: #f7f8fb
--surface: #e6ebf2
--ink: #121826
--accent: #009bb8
--accent-2: #5a7d2a
</code></pre>
              </div>
            </div>
          </div>
        </section>

        <section id="docs" class="section">
          <div class="split">
            <div class="reveal delay-1">
              <h2>Get started in minutes</h2>
              <p class="section-copy">
                Launch the binary, point it at config.yaml, and let the hosted
                client handle the browser-side exchange.
              </p>
              <div class="docs-links">
                <a class="docs-link" href="usage.html"
                  >Usage guide <span>docs/usage.md</span></a
                >
                <a
                  class="docs-link"
                  href="https://github.com/tyemirov/TAuth/blob/master/ARCHITECTURE.md"
                  >Architecture <span>ARCHITECTURE.md</span></a
                >
                <a
                  class="docs-link"
                  href="https://github.com/tyemirov/TAuth/discussions"
                  >Community <span>GitHub Discussions</span></a
                >
              </div>
            </div>
            <div class="panel hero-visual reveal delay-2">
              <div class="terminal">
                <div class="terminal-header">
                  <span class="terminal-dot"></span>
                  <span class="terminal-dot" style="background: var(--accent-warm)"></span>
                  <span class="terminal-dot" style="background: #7aa6ff"></span>
                  quickstart
                </div>
                <pre><code>$ tauth --config=config.yaml
listen :8443
cookies: app_session / app_refresh
tauth.js: /tauth.js
</code></pre>
              </div>
            </div>
          </div>
        </section>
      </main>

      <footer class="site-footer">
        <div class="site-footer__inner">
          <div class="site-footer__brand">TAuth by Marco Polo Research Lab</div>
          <div class="site-footer__links">
            <a href="https://github.com/tyemirov/TAuth">GitHub</a>
            <a href="https://github.com/tyemirov/TAuth/blob/master/docs/usage.md">Docs</a>
            <a href="https://github.com/tyemirov/TAuth/discussions">Community</a>
          </div>
        </div>
      </footer>
    </div>
  </body>
</html>

## migration.md

# GAuss to TAuth Migration

## Audience and goal
This document is written from the perspective of an engineer migrating an established GAuss integration to the TAuth service in a running application. The goal is to move authentication and session management to TAuth while keeping product behavior stable and minimizing user disruption.

## Current GAuss footprint to inventory
I will start by mapping where GAuss is embedded in the running app so the migration is scoped correctly.

- Authentication flow uses GAuss routes: /login, /auth/google, /auth/google/callback, /logout, and the post-login redirect.
- Session state lives in the gauss_session cookie and is enforced through gauss.AuthMiddleware.
- User identity is stored in session keys for email, name, and picture, plus the OAuth token for downstream Google API usage.
- OAuth scopes may be broader than profile and email; any non-profile scope implies the app depends on Google API access tokens.
- Logout behavior is driven by GAuss redirect configuration.

## Target TAuth model
TAuth is a standalone service that verifies Google Identity Services ID tokens and mints first-party cookies.

- Session cookies are app_session (JWT access token) and app_refresh (opaque refresh token).
- Core endpoints are /auth/nonce, /auth/google, /auth/refresh, /auth/logout, and /me.
- These endpoints are provided by the TAuth server only; consuming apps should call them rather than implement them.
- Tenant configuration is YAML-driven and includes host routing, cookie domain, Google web client ID, signing keys, and TTLs.
- Session validation is performed by verifying the JWT signature, issuer, and time-based claims in app_session.

## Migration path
### 1. Pre-migration assessment
I will confirm the GAuss responsibilities that must move to TAuth and identify any blockers.

- List every route guarded by gauss.AuthMiddleware and any code that reads GAuss session keys.
- Identify where the stored GAuss OAuth token is used to call Google APIs; TAuth only validates Google ID tokens and does not mint OAuth access tokens for Google APIs.
- Define the application user identity and roles used today so TAuth can embed equivalent claims in its JWTs.

### 2. Build the TAuth deployment
I will deploy TAuth as a separate service with production-grade settings.

- Choose the TAuth host and cookie domain so cookies cover the product origin without leaking beyond the intended registrable domain.
- Configure the tenant entry with tenant origins, Google web client ID, JWT signing key, and TTLs that match existing session expectations.
- Use a persistent refresh token store and set database_url to avoid losing refresh tokens between restarts.
- Enable CORS only when the UI and TAuth are on different origins and include accounts.google.com and the product origin in cors_allowed_origins; list accounts.google.com under cors_allowed_origin_exceptions so validation permits the non-tenant origin.
- Keep allow_insecure_http disabled in production and terminate TLS in front of TAuth so Secure cookies are issued.
- Use environment variable expansion for secrets in the YAML to keep signing keys and client IDs out of the file.

### 3. Update Google Identity configuration
I will update Google Cloud OAuth settings to support TAuth.

- Add the TAuth host and the product origin to Authorized JavaScript origins for the Google web client.
- Keep the existing GAuss OAuth client and redirect URIs until cutover so legacy logins remain functional.

### 4. Integrate user store and roles
I will ensure the TAuth user store maps Google identities to the existing application user model.

- Implement a UserStore in the TAuth service that maps Google subject values to existing user IDs and roles.
- Ensure /me returns the fields required by the product UI and that JWT claims align with the current authorization model.

### 5. Frontend integration
I will replace the GAuss redirect login flow with the TAuth browser flow.

- Load /tauth.js from the TAuth host and initialize it on app startup.
- Use the authenticated and unauthenticated callbacks to drive the UI state, replacing the GAuss login page and redirect flow.
- Use `getAuthEndpoints()` to derive `/auth/nonce` and `/auth/google` URLs from the helper (the base URL must be provided explicitly via `initAuthClient`).
- Route all authenticated API calls through a fetch wrapper that can call /auth/refresh when a 401 response is returned.
- Replace GAuss logout with /auth/logout so refresh tokens are revoked and cookies are cleared.

### 6. Backend integration
I will update backend services to validate TAuth sessions instead of GAuss cookies.

- Replace gauss.AuthMiddleware with JWT validation for app_session and return 401 rather than redirecting to /login.
- Use the TAuth sessionvalidator package in Go services, and use equivalent JWT validation in other services with the tenant signing key and issuer.
- Update authorization logic to read roles and tenant_id from the JWT claims.

### 7. Parallel run and cutover
I will switch traffic without downtime and accept that reauthentication is required.

- Run GAuss and TAuth in parallel during rollout; cookies do not collide because their names differ.
- Ship the TAuth UI integration behind a feature flag, then ramp gradually.
- Expect users to reauthenticate to obtain TAuth cookies because GAuss sessions cannot be migrated.
- After adoption is stable, remove GAuss handlers, session storage, and configuration from the application.

### 8. Validation and monitoring
I will validate the flow end to end and monitor for regressions.

- Confirm nonce issuance, Google credential exchange, and session cookie issuance work consistently.
- Confirm /me returns expected user data and that refresh rotation works across browser tabs.
- Monitor logs for nonce mismatches, token validation failures, tenant resolution errors, and CORS rejections.

### 9. Cleanup
I will remove remaining GAuss dependencies once TAuth is the sole authentication path.

- Remove GAuss environment variables, routes, and templates from the codebase.
- Update operational runbooks to document TAuth configuration, signing key rotation, and troubleshooting.

## usage.md

# TAuth Usage Guide

This document is the authoritative guide for operators and front‑end teams integrating against a TAuth deployment. It explains how to run the service, how sessions work, and how to connect a browser application using either the provided helper script or direct HTTP calls.

For a deep dive into internal architecture and implementation details, see `ARCHITECTURE.md`. For confident‑programming and refactor policies, see `POLICY.md` and `docs/refactor-plan.md`.

---

## 1. What TAuth provides

TAuth sits between Google Identity Services (GIS) and your product UI:

- Verifies Google ID tokens issued by a Google OAuth Web client.
- Mints short‑lived access cookies and long‑lived refresh cookies.
- Rotates refresh tokens on every refresh call and revokes them on logout.
- Exposes a small HTTP API and a browser helper (`/tauth.js`) for zero-token-in-JavaScript sessions.

Once TAuth is running for a given registrable domain, any app on that domain (or its subdomains) can rely on the `HttpOnly` session cookies instead of storing tokens in `localStorage` or JavaScript memory.

---

## 2. Running the service

### 2.1 Binary layout

The `tauth` binary lives under `cmd/server` in this repository. You can:

- Build it directly with Go (e.g. `go build ./cmd/server`), or
 - Use the provided Docker setup in `examples/tauth-demo` for a local stack.

The binary reads configuration exclusively from a YAML file (default `config.yaml`). Use `tauth --config=/path/to/config.yaml` or export `TAUTH_CONFIG_FILE` to point at a different file; no other environment variables or CLI flags are required.

### 2.2 Core configuration

`config.yaml` must include the server-level keys below plus at least one tenant:

| Key | Purpose | Example |
| --- | --- | --- |
| `listen_addr` | HTTP listen address | `:8080` |
| `database_url` | Refresh store DSN | `sqlite:///data/tauth.db` |
| `enable_cors` | Enable CORS for cross-origin UIs | `true` / `false` |
| `cors_allowed_origins` | Allowed origins when CORS is enabled (include your UI origins *and* `https://accounts.google.com`) | `["https://app.example.com","https://accounts.google.com"]` |
| `cors_allowed_origin_exceptions` | Allowed non-tenant origins that may appear in `cors_allowed_origins` | `["https://accounts.google.com"]` |
| `enable_tenant_header_override` | Allow `X-TAuth-Tenant` overrides (dev/local only) | `true` / `false` |
| `tenants` | Array of tenant entries (see README §5.1 for schema) | `[...]` |

Key notes:

- **TLS and cookies**: In production, terminate TLS at the load balancer or the service so cookies can be marked `Secure`. Each tenant defines its own `cookie_domain`; use that field (e.g. `.example.com`) to share cookies across subdomains. Leave the field blank to emit host-only cookies during `localhost` development (browsers reject `Domain=localhost`).
- **Database URL**: For SQLite, use triple‑slash absolute paths (`sqlite:///data/tauth.db`). Host‑based forms such as `sqlite://file:/data/tauth.db` are rejected. For Postgres, use a standard DSN (`postgres://user:pass@host:5432/dbname?sslmode=disable`).
- **CORS**: Leave `enable_cors` set to `false` when UI and API share the same origin. Enable it only when your UI is on a different origin (for example, Vite dev server) and set `cors_allowed_origins` explicitly. If you include non-tenant origins (for example `https://accounts.google.com`), also list them under `cors_allowed_origin_exceptions` so validation permits them.
- **Shared origins**: If two tenants intentionally share the same origin (typical for localhost demos), add each frontend origin (`http://localhost:8000`, `http://localhost:4173`, …) to the tenant’s `tenant_origins`. TAuth inspects the request `Origin` header to resolve the tenant automatically. You can still enable `enable_tenant_header_override` and send `X-TAuth-Tenant` when you want to override the origin mapping manually.
- **Per-tenant signing keys**: Each tenant block must declare a `jwt_signing_key`. TAuth uses that HS256 secret exclusively for the tenant’s cookies, so rotate keys per tenant instead of relying on a global fallback.
- **Local HTTP mode**: Setting `allow_insecure_http: true` on a tenant drops the `Secure` flag and downgrades cookies to `SameSite=Lax` so browsers keep them over HTTP even while CORS is enabled. This only works when your dev UI also runs on `http://localhost` (same host, different port); switching hosts such as `127.0.0.1` will make the browser treat the request as cross-site and block the cookies.

### 2.3 Example: hosted deployment

This example mirrors the README but focuses on the minimum you need to host TAuth at `https://auth.example.com` for a product UI at `https://app.example.com`:

```bash
cat > config.yaml <<'YAML'
server:
  listen_addr: ":8443"
  database_url: "sqlite:///data/tauth.db"
  enable_cors: true
  cors_allowed_origins:
    - "https://app.example.com"
    - "https://accounts.google.com"
  cors_allowed_origin_exceptions:
    - "https://accounts.google.com"
  enable_tenant_header_override: false

tenants:
  - id: "prod"
    display_name: "Production Tenant"
    tenant_origins:
      - "https://app.example.com"
    google_web_client_id: "your_web_client_id.apps.googleusercontent.com"
    jwt_signing_key: "replace-with-your-tenant-signing-key"
    cookie_domain: ".example.com"
    session_ttl: "15m"
    refresh_ttl: "1440h"
    nonce_ttl: "5m"
    allow_insecure_http: false
YAML

tauth --config=config.yaml
```

Run this behind TLS so the service issues `Secure` cookies and the browser accepts them.

When migrating an existing tenant that expects the legacy cookie names (`app_session`, `app_refresh`), set the `session_cookie_name` / `refresh_cookie_name` fields inside the tenant block. These fields are always required—choose unique names per tenant to avoid collisions when multiple tenants share `localhost`. Legacy stacks (such as Gravity) can keep `app_session` / `app_refresh`, but doing so means any other tenant using the same names will overwrite those cookies.

### 2.4 Example: local quick‑start (Docker Compose)

For a full local stack (TAuth + demo UI) without installing Go:

1. `cd examples/tauth-demo`
2. Edit `.env.tauth` (set `TAUTH_CONFIG_FILE=/config/config.yaml` and the per-tenant `TAUTH_GOOGLE_WEB_CLIENT_ID` / `TAUTH_JWT_SIGNING_KEY` values).
3. Review `config.yaml` and replace the placeholder Google OAuth client with one registered for `http://localhost:8000` and `http://localhost:8082` (or keep the environment variable references from step 2).
4. Start the stack: `docker compose up --build`
5. Visit `http://localhost:8000` for the demo UI. It talks to TAuth at `http://localhost:8082`.

Stop the stack with `docker compose down`. The `tauth_data` volume holds the SQLite database, and `config.yaml` stays next to the compose file for future edits.

### 2.5 Preflight validation (pre-start)

Use the preflight command to validate configuration and emit a redacted effective-config report before you launch the service:

```bash
tauth preflight --config=config.yaml
```

The report includes effective server settings, per-tenant cookie names and TTLs, derived SameSite modes, and JWT signing key fingerprints (never raw keys). Redacted reports still emit `tenant_origin_hashes` and `jwt_signing_key_fingerprint` so external validators can compare secrets without exposing them. To include the raw `tenant_origins` list, pass `--include-origins`.

The JSON payload is versioned and shaped as:
- `schema_version`, `service` metadata
- `effective_config` (server + tenant settings)
- `dependencies` (preflight checks with readiness status)

The preflight builder is generalized under `github.com/tyemirov/utils/preflight` with a Viper-based adapter (`github.com/tyemirov/utils/preflight/viperconfig`) for services that load YAML configs and bind env vars through Viper.

---

## 3. Sessions and cookies

TAuth works with two cookies:

- `app_session` – short‑lived JWT access token.
  - `HttpOnly`, `Secure`, `SameSite` (strict by default).
  - Sent with all requests under the configured cookie domain.
- `app_refresh` – opaque refresh token.
  - `HttpOnly`, `Secure`, `Path=/auth`.
  - Rotated on `/auth/refresh` and revoked on `/auth/logout`.

Your product should:

- Use `app_session` to protect routes (for example via `pkg/sessionvalidator` in other Go services).
- Never store tokens in JavaScript; rely on these cookies.
- Call `/auth/refresh` when API calls return `401` to keep sessions alive.

---

## 4. Recommended integration: `tauth.js`

The simplest way to use TAuth from the browser is through the helper served at `/tauth.js`. It exports eight globals:

- `initAuthClient(options)` – hydrates the current user and sets up refresh behaviour.
- `apiFetch(url, init)` – wrapper around `fetch` that automatically refreshes sessions on `401`.
- `getCurrentUser()` – returns the current profile object or `null`.
- `getAuthEndpoints()` – returns the resolved URL map for `/me` and `/auth/*`.
- `requestNonce()` – fetches a one-time nonce for Google Identity Services.
- `exchangeGoogleCredential({ credential, nonceToken })` – exchanges the Google credential for cookies and updates the profile.
- `logout()` – revokes the refresh token and clears client state.
- `setAuthTenantId(tenantId)` – sets the tenant override for subsequent requests.

For backend services written in Go, use the `pkg/sessionvalidator` package described in section 6.8 to validate `app_session` cookies.

### 4.1 Loading the helper

On your product site, include the script from wherever you host the asset:

```html
<script
  src="https://tauth.mprlab.com/tauth.js"
  data-tenant-id="tenant-admin"
></script>
```

### 4.2 Initialising on page load

Call `initAuthClient` once during startup, after the script loads. The `baseUrl` option is required and must point at your TAuth API origin:

```html
<script>
  // Optional: override tenant dynamically when the page knows which tenant to use.
  setAuthTenantId("tenant-admin");
  initAuthClient({
    baseUrl: "https://auth.example.com",
    tenantId: "demo", // optional override for shared-origin dev setups
    onAuthenticated(profile) {
      renderDashboard(profile);
    },
    onUnauthenticated() {
      showSignInButton();
    },
  });
</script>
```

Behaviour:

- TAuth calls `GET /me` to check for an existing session.
- If missing or expired, it attempts `POST /auth/refresh`.
- If refresh succeeds, it calls `onAuthenticated(profile)`; otherwise it calls `onUnauthenticated()`.
- The `profile` object matches the `/me` response (see section 6.3).

### 4.3 Calling your own APIs with `apiFetch`

Wrap all authenticated HTTP requests through `apiFetch`:

```js
async function loadProtectedData() {
  const response = await apiFetch("/api/data", { method: "GET" });
  if (!response.ok) {
    throw new Error("request_failed");
  }
  return response.json();
}
```

When a call returns `401`, `apiFetch`:

1. Sends `POST /auth/refresh` with `credentials: "include"`.
2. Retries the original request on success.
3. Broadcasts `"refreshed"` events via `BroadcastChannel` (if available), allowing multiple tabs to stay in sync.

If refresh fails, pending requests reject and callers can treat this as “logged out”.

### 4.4 Logging out

Use `logout()` to terminate the session:

```js
async function handleLogoutClick() {
  await logout();
  redirectToLanding();
}
```

The helper:

- Calls `POST /auth/logout` to revoke the refresh token.
- Clears local profile state.
- Broadcasts `"logged_out"` to other tabs.
- Invokes `onUnauthenticated()` if provided.

### 4.5 Selecting a tenant explicitly

Most deployments rely on the request `Origin` header to resolve tenants. When multiple tenants intentionally share the same origin (for example, several apps pointing at `http://localhost:8080`) or when requests omit `Origin` (non-browser clients), enable the TAuth server’s header override (`--enable_tenant_header_override`). Once enabled, the helper tags `/me` and `/auth/*` calls with either your explicit `tenantId` or, when omitted, the current page origin so shared-origin setups continue to function even if certain requests omit `Origin`. You can still pin a specific tenant explicitly by passing `tenantId` to `initAuthClient`:

```js
initAuthClient({
  baseUrl: "https://auth-dev.example.com",
  tenantId: "team-blue",
  onAuthenticated: hydrateDashboard,
  onUnauthenticated: showGoogleButton,
});
```

The helper automatically attaches `X-TAuth-Tenant: team-blue` (or the current page origin when no ID is supplied) to `/me`, `/auth/nonce`, `/auth/google`, `/auth/refresh`, and logout requests while leaving your own API traffic alone. Switch tenants by reinitialising with a different `tenantId` (or prefer separate origins when possible). The override still resolves against the configured tenant list, so unknown tenant IDs or origins are rejected.

---

## 5. Google Identity Services flow

TAuth assumes a GIS **Web** client using the popup flow. A nonce protects each sign‑in exchange.

### 5.1 Configure GIS

1. Create (or reuse) a Google OAuth Web client.
2. Add all product origins (for example `https://app.example.com`) to **Authorized JavaScript origins**.
3. Load the GIS script:

   ```html
   <script src="https://accounts.google.com/gsi/client" async defer></script>
   <div id="googleSignIn"></div>
   ```

### 5.2 Nonce and credential exchange

The required sequence for custom clients is:

1. **Nonce** – `POST /auth/nonce`
   - Returns `{ "nonce": "<random>" }`.
2. **Initialize GIS** with the nonce:
   - `google.accounts.id.initialize({ client_id, nonce, ux_mode: "popup", callback })`.
3. **Show the button / popup** via GIS APIs.
4. **Exchange credential** – when GIS invokes your callback with `response.credential`:
   - Call `POST /auth/google` with JSON `{ "google_id_token": "<response.credential>", "nonce_token": "<same nonce>" }` and `credentials: "include"`.
5. TAuth:
   - Validates the ID token against the resolved tenant’s `google_web_client_id`.
   - Verifies the nonce (raw or hashed) and the issuer.
   - Issues `app_session` and `app_refresh` cookies.
   - Returns a profile JSON payload.

> You must fetch a fresh nonce for every sign‑in attempt. TAuth invalidates a nonce as soon as it is used.

When using `tauth.js` or the mpr‑ui header component, this flow is handled internally; you only need to surface the Google button and configure your client ID.

---

## 6. HTTP endpoints

This section documents the public HTTP surface from a client’s perspective. See `ARCHITECTURE.md` for a stable contract summary and versioning notes. These endpoints are served exclusively by the TAuth server; consuming applications should call them, not reimplement them.

### 6.1 `POST /auth/nonce`

Issues a one‑time nonce for the next GIS exchange.

- **Request**: empty JSON body. Include `credentials: "include"` if you want to reuse cookies on same origin.
- **Response**: `200 OK` with JSON:

  ```json
  { "nonce": "..." }
  ```

### 6.2 `POST /auth/google`

Verifies a Google ID token and mints cookies.

- **Request body**:

  ```json
  {
    "google_id_token": "<id_token_from_gis>",
    "nonce_token": "<nonce_from_/auth/nonce>"
  }
  ```

- **Response**: `200 OK` with user profile JSON (see `/me` below). Sets `app_session` and `app_refresh` cookies.

Common failure cases:

- Invalid or expired ID token (`401`).
- Mismatched nonce (`401`).
- Audience (`aud`) does not match the resolved tenant’s `google_web_client_id` (`401`).

### 6.3 `GET /me`

Returns the profile associated with the current session.

- **Auth**: requires a valid `app_session` cookie.
- **Response**:

  ```json
  {
    "user_id": "google:12345",
    "user_email": "user@example.com",
    "display": "Example User",
    "avatar_url": "https://lh3.googleusercontent.com/a/...",
    "roles": ["user"],
    "expires": "2024-05-30T12:34:56.000Z"
  }
  ```

- **Errors**: `401` when the access cookie is missing, expired, or invalid.

### 6.4 `POST /auth/refresh`

Rotates the refresh token and mints a new access cookie.

- **Auth**: requires a valid `app_refresh` cookie.
- **Request body**: empty.
- **Response**: `204 No Content` on success. Sets new `app_session` and `app_refresh` cookies.

After a successful refresh, call `/me` again or rely on `tauth.js` to hydrate the profile.

### 6.5 `POST /auth/logout`

Revokes the refresh token and clears cookies.

- **Auth**: best‑effort; succeeds even if no valid refresh token is present.
- **Request body**: empty.
- **Response**: `204 No Content`. Clears `app_session` and `app_refresh`.

Clients should treat this as “signed out” regardless of prior state.

### 6.6 `GET /tauth.js`

Serves the browser helper described in section 4.

- Include it via `<script src="https://your-tauth-origin/tauth.js"></script>`.
- Exposes `initAuthClient`, `apiFetch`, `getCurrentUser`, `getAuthEndpoints`, `requestNonce`, `exchangeGoogleCredential`, `logout`, and `setAuthTenantId` on `window`.
- The TAuth service serves only API endpoints plus `/tauth.js`; demo pages live in `examples/` and are served separately.

## 6.7 Validating sessions from other Go services

Downstream Go services that share the TAuth cookie domain can validate `app_session` cookies directly using the `pkg/sessionvalidator` package. This is the recommended way to enforce authentication and read identity information without duplicating JWT logic.
If your service can read the same `config.yaml` as TAuth, call `LoadTenantAuthConfig` to derive the tenant’s signing key, issuer, and cookie names before constructing a validator.

### 6.7.1 Basic validator setup

Add the module to your Go service and construct a validator at startup:

```go
import (
    "os"

    "github.com/tyemirov/tauth/pkg/sessionvalidator"
)

func newSessionValidator() (*sessionvalidator.Validator, error) {
    signingKey := []byte(os.Getenv("TAUTH_NOTES_JWT_SIGNING_KEY"))
    return sessionvalidator.New(sessionvalidator.Config{
        SigningKey: signingKey,
        Issuer:     "tauth",
        // CookieName: optional; defaults to "app_session".
    })
}
```

The configuration mirrors your TAuth deployment:

- `SigningKey` must match the `jwt_signing_key` configured for the tenant whose cookies you validate.
- `Issuer` must match the issuer configured by the server (typically `"tauth"`; see `ARCHITECTURE.md`).
- `CookieName` defaults to `app_session` and should only be overridden if you have customised the cookie name on the TAuth side.

The constructor validates configuration up front and returns a typed error if required fields are missing.

### 6.7.2 Gin middleware integration

For Gin-based services, use the built-in middleware to protect routes and attach claims to the context:

```go
import (
    "log"

    "github.com/gin-gonic/gin"
    "github.com/tyemirov/tauth/pkg/sessionvalidator"
)

func main() {
    validator, err := newSessionValidator()
    if err != nil {
        log.Fatalf("invalid validator configuration: %v", err)
    }

    router := gin.Default()
    router.Use(validator.GinMiddleware(sessionvalidator.DefaultContextKey))

    router.GET("/me", func(context *gin.Context) {
        claimsValue, exists := context.Get(sessionvalidator.DefaultContextKey)
        if !exists {
            context.AbortWithStatus(http.StatusUnauthorized)
            return
        }
        claims := claimsValue.(*sessionvalidator.Claims)
        context.JSON(http.StatusOK, map[string]interface{}{
            "user_id":    claims.GetUserID(),
            "user_email": claims.GetUserEmail(),
            "display":    claims.GetUserDisplayName(),
            "avatar_url": claims.GetUserAvatarURL(),
            "roles":      claims.GetUserRoles(),
        })
    })

    _ = router.Run()
}
```

Key points:

- The middleware reads the `app_session` cookie from each request, validates it, and aborts with `401` when invalid.
- On success, it stores a `*sessionvalidator.Claims` value in the Gin context under the provided key (default `auth_claims`).
- Handler code can safely cast this value and use the helper methods (`GetUserID`, `GetUserEmail`, `GetUserDisplayName`, `GetUserAvatarURL`, `GetUserRoles`, `GetExpiresAt`) to drive authorization and UI decisions.

### 6.8.3 Manual validation flows

If you are not using Gin, or you need finer-grained control, use the lower-level helpers:

- `ValidateRequest(*http.Request)` – validates the session cookie on an incoming request and returns `*Claims`.
- `ValidateToken(string)` – validates a raw JWT string, for example when the token is forwarded between services.

Example with `net/http`:

```go
func handleProtectedRoute(response http.ResponseWriter, request *http.Request, validator *sessionvalidator.Validator) {
    claims, err := validator.ValidateRequest(request)
    if err != nil {
        http.Error(response, "unauthorized", http.StatusUnauthorized)
        return
    }
    // Use claims.* accessors here.
}
```

Using the shared validator keeps your services aligned with TAuth’s JWT format and validation rules, and avoids duplicating cryptographic or time-based logic across codebases.

---

## 7. Typical flows

### 7.1 First sign‑in

1. User clicks “Sign in with Google”.
2. UI calls `/auth/nonce`, configures GIS with the nonce, and shows the popup.
3. GIS returns a credential; UI posts it to `/auth/google`.
4. TAuth validates the token, issues cookies, returns profile JSON.
5. UI renders signed‑in state and begins using `apiFetch` for protected calls.

### 7.2 Silent refresh

1. An API call via `apiFetch` returns `401`.
2. `apiFetch` sends `POST /auth/refresh` with the refresh cookie.
3. On success, it retries the original request and broadcasts `"refreshed"`.
4. UI continues to operate with renewed session cookies.

### 7.3 Logout

1. User clicks “Sign out”.
2. UI calls `logout()`.
3. TAuth revokes the refresh token and clears cookies.
4. Helper broadcasts `"logged_out"`; all tabs transition to unauthenticated state.

---

## 8. Troubleshooting

Use this checklist when integrating:

- **401 from `/me` but refresh works** – Session cookie expired; ensure your client either uses `tauth.js` or calls `/auth/refresh` before retrying.
- **401 from `/auth/refresh`** – Refresh cookie missing or revoked; treat as “signed out” and prompt the user to sign in again.
- **No cookies set** – Verify:
  - The response comes from HTTPS (in production).
  - The tenant’s `cookie_domain` matches the registrable domain you expect.
  - CORS is configured correctly when using a split origin (`enable_cors` and `cors_allowed_origins` in `config.yaml`).
- **Google rejects the client or TAuth rejects the token** – Confirm:
  - The OAuth client type is **Web**.
  - All relevant origins are in the **Authorized JavaScript origins** list.
  - The `aud` claim in the ID token matches the tenant’s `google_web_client_id`.

For more detailed operational guidance, refer to the troubleshooting section in `ARCHITECTURE.md`.
- When multiple tenants share the same origin, list each frontend origin under `tenant_origins` so TAuth can resolve the tenant from the `Origin` header. You can still override the mapping by adding `data-tenant-id="tenant-id"` to the script tag (see 4.1) or by calling `setAuthTenantId("tenant-id")` before `initAuthClient(...)`. The helper automatically sends `X-TAuth-Tenant` whenever you opt into an explicit override, and now falls back to the page origin when no tenant ID is provided.

## README.md

# TAuth

*Google Sign-In + JWT sessions for single-origin apps*

TAuth lets product teams accept Google Sign-In, mint their own cookies, and keep browsers free of token storage. Ship a secure authentication stack by pairing this Go service with the tiny `tauth.js` module.
TAuth servers are the only place `/auth/*` and `/me` endpoints are implemented; consuming apps call those endpoints rather than hosting their own copies.

---

## Why teams choose TAuth

- **Own the session lifecycle** – verify Google once, then rely on short-lived access cookies and rotating refresh tokens.
- **Zero tokens in JavaScript** – the client handles hydration, silent refresh, and logout notifications without touching `localStorage`.
- **Minutes to value** – a single binary with predictable defaults, powered by Gin and Google’s official identity SDK.
- **Designed for growth** – plug in Postgres or SQLite to persist refresh tokens, and extend the web hook points to fit your product.

---

## Deploy TAuth for a hosted product

### 1. Describe your tenants

Every deployment — even “single tenant” ones — loads configuration from a YAML file. Define your tenants (origins, Google clients, cookie domain, and TTLs) once and pass that file to every TAuth process:

```bash
cat > tenants.yaml <<'YAML'
tenants:
  - id: "prod"
    display_name: "Production tenant"
    tenant_origins:
      - "https://gravity.mprlab.com"
      - "https://pinguin.mprlab.com"
    google_web_client_id: "your_web_client_id.apps.googleusercontent.com"
    jwt_signing_key: "replace-with-your-tenant-signing-key"
    cookie_domain: ".mprlab.com"
    session_ttl: "15m"
    refresh_ttl: "1440h"
    nonce_ttl: "5m"
    allow_insecure_http: false
YAML
```

Tenant files accept shell-style environment placeholders (`${TENANT_COOKIE_DOMAIN}` or `$TENANT_COOKIE_DOMAIN`) in any string field. TAuth expands those variables before validation so you can keep secrets or per-host values in `.env` files; missing variables collapse to empty strings, so keep sensible defaults in the YAML when a field is required.

Each entry defines:

- `id` – stable identifier used inside JWTs and storage (lowercase letters/numbers/underscores/hyphens).
- `display_name` – friendly label surfaced in logs and the demo UI.
- `tenant_origins` – browser origins that should resolve to this tenant. Entries must be full origins (`https://app.example.com`, `http://localhost:8000`); the resolver uses the request `Origin` header (or the `X-TAuth-Tenant` override) to select a tenant, so do not list the TAuth API hostname here.
- `google_web_client_id` – OAuth Web client configured in Google Cloud Console for this tenant’s origins.
- `jwt_signing_key` – HS256 secret unique to this tenant. Every tenant must declare its own signing key so sessions remain isolated.
- `cookie_domain` – registrable domain for cookies (e.g. `.mprlab.com` to share cookies across subdomains). Leave it blank to emit host-only cookies when developing on `localhost`.
- `session_ttl` / `refresh_ttl` / `nonce_ttl` – durations using Go’s `time.ParseDuration` syntax.
- `allow_insecure_http` – `true` only for local development; production tenants must stay `false`. When enabled, cookies drop the `Secure` flag and default to `SameSite=Lax` so browsers keep them over HTTP (even if CORS is on). That setup only works when your dev UI also runs on `http://localhost`, so avoid mixing hosts like `127.0.0.1`.

### 2. Launch the service (e.g. on `https://tauth.mprlab.com`)

```bash
cat > config.yaml <<'YAML'
server:
  listen_addr: ":8443"
  database_url: "sqlite:///data/tauth.db"
  enable_cors: true
  cors_allowed_origins:
    - "https://gravity.mprlab.com"
    - "https://accounts.google.com"
  cors_allowed_origin_exceptions:
    - "https://accounts.google.com"
  enable_tenant_header_override: false

tenants:
  - id: "gravity"
    display_name: "Gravity"
    tenant_origins: ["https://gravity.mprlab.com"]
    google_web_client_id: "gravity-client.apps.googleusercontent.com"
    jwt_signing_key: "replace-with-gravity-signing-key"
    cookie_domain: ".mprlab.com"
    session_ttl: "30m"
    refresh_ttl: "720h"
    nonce_ttl: "10m"
    allow_insecure_http: false
YAML

tauth --config=config.yaml
# or set TAUTH_CONFIG_FILE=/etc/tauth/config.yaml and run `tauth`
```

Before deploying, run `tauth preflight --config=config.yaml` to validate the config and emit a redacted effective-config report (signing keys and tenant origins are reported as fingerprints only so validators can compare without seeing secrets).

> SQLite DSN tip: use three slashes for absolute paths (e.g. `sqlite:///data/tauth.db`). Host-based forms such as `sqlite://file:/data/tauth.db` are invalid and rejected at startup.

When multiple product origins need access, list them under the `cors_allowed_origins` array inside `config.yaml`. If you include non-tenant origins (for example `https://accounts.google.com`), mirror them in `cors_allowed_origin_exceptions` so config validation permits them.

Host the binary behind TLS (or terminate TLS at your load balancer) so responses set `Secure` cookies. Working from the tenants file above, cookies issued by `https://tauth.mprlab.com` will also be sent with requests made by `https://gravity.mprlab.com` because both live under `.mprlab.com`.

### Run the demo with Docker Compose (local quick-start)

We ship a compose example under `examples/tauth-demo` that builds TAuth from the local Dockerfile and pairs it with a simple static web server (`ghcr.io/tyemirov/ghttp:latest`) serving the demo assets on port `8000`. The TAuth service itself serves only API endpoints plus `/tauth.js`.

1. `cd examples/tauth-demo`
2. Update the environment file with your Google OAuth client ID and signing key:

   ```bash
   $EDITOR .env.tauth
   ```

3. Review `config.yaml` to ensure the tenant origins and ports match your local setup.
4. Build and start the stack: `docker compose up --build`
5. Visit `http://localhost:8000` to load the demo UI (it communicates with TAuth at `http://localhost:8082` via CORS).

The sample config now defines **two tenants** so you can exercise origin-based routing without touching `/etc/hosts`. Thanks to RFC 6761, any `*.localhost` name automatically resolves to `127.0.0.1`, so both tenants work out of the box:

- `notes` – resolve via `http://localhost:8082` (or the Gravity UI at `http://localhost:8000`). This matches the default Gravity config and is the tenant you’ve already used.
- `mpr-sites` – the `mpr-frontend` container serves `examples/tauth-demo/index.html` on `http://localhost:8001`. Its browser origin (`http://localhost:4173`) lives under `tenant_origins`, so TAuth can derive the tenant from the request `Origin` header without extra UI wiring.

This setup lets you verify header overrides, cookie isolation, and resolver behavior locally before promoting changes to production.

When two tenants share `localhost`, list each frontend origin (for example `http://localhost:8000` for Gravity and `http://localhost:4173` for the MPR demo) under `tenant_origins`. TAuth inspects the `Origin` header and resolves the tenant automatically, so the UI doesn’t need to set `data-tenant-id` or call `setAuthTenantId` just to distinguish environments.

Stop the stack with `docker compose down`. The compose file persists refresh tokens inside a named `tauth_data` volume mounted at `/data`, so you can inspect or reset the SQLite database between runs. Update `.env.tauth` (or the referenced `config.yaml`) to change ports, database DSNs, origins, cookie domains, or Google credentials before re-running. Re-run `docker compose up --build` whenever you change Go code so the local image picks up your edits.

### 3. Integrate the browser helper from the product site

```html
<script src="https://tauth.mprlab.com/tauth.js"></script>
<script>
  initAuthClient({
    baseUrl: "https://tauth.mprlab.com",
    tenantId: "demo", // optional override when multiple tenants share an origin
    onAuthenticated(profile) {
      renderDashboard(profile);
    },
    onUnauthenticated() {
      showGoogleButton();
    }
  });
</script>

<div id="googleSignIn"></div>
```

The GitHub Pages workflow in `.github/workflows/frontend-deploy.yml` publishes the `web/` directory, so the helper is available at `https://<pages-domain>/tauth.js` when Pages is enabled.

`tauth.js` requires an explicit `baseUrl` in `initAuthClient`; it never infers the API host from the script origin.

### 4. Prepare and exchange Google credentials across origins

`tauth.js` already fetches nonces, initializes Google Identity Services, and exchanges credentials for you. Render the button, provide `onAuthenticated` / `onUnauthenticated` callbacks, and the helper keeps cookies fresh across your origin. When building a custom UI, follow the handshake described in [ARCHITECTURE.md#google-sign-in-exchange](ARCHITECTURE.md#google-sign-in-exchange): fetch a nonce, pass it to Google when initializing the popup, then POST `{ google_id_token, nonce_token }` to `/auth/google`. The minted `app_session` cookie authenticates `/api/me` and any downstream routes on the configured domain (e.g. `.mprlab.com`).

### Configure Google Identity Services (popup flow)

1. **Create or reuse a Google OAuth Web client.** Add every product origin (e.g. `https://gravity.mprlab.com`) to the *Authorized JavaScript origins* list. Redirect URIs are not required for this popup flow.
2. **Load the GIS SDK before you render a button.**

   ```html
   <script src="https://accounts.google.com/gsi/client" async defer></script>
   <div id="googleSignIn"></div>
   ```

3. **Fetch and attach a nonce before prompting Google.** Use `POST /auth/nonce`, call `google.accounts.id.initialize({ nonce, client_id, ux_mode: "popup" })`, and render the button programmatically (see `prepareGoogleSignIn` above or `examples/tauth-demo/index.html`).
4. **Exchange the credential without redirecting.** When GIS invokes your callback, post `{ google_id_token, nonce_token }` to `https://tauth.mprlab.com/auth/google` (or your hosted base URL) with `credentials: "include"` so TAuth can mint cookies.

### Quick verification checklist

- Open the browser console and confirm a nonce request (`POST /auth/nonce`) fires before the GIS popup.
- Click the button; the popup should open and return a credential to `handleCredential`.
- Check the network tab for `POST https://tauth.mprlab.com/auth/google` and ensure it succeeds (`200`).
- Inspect cookies; `app_session` and `app_refresh` should now be scoped to the configured domain (e.g. `.mprlab.com`).
- Call `/api/me` and verify it returns the signed-in profile.

> **Tip:** The Docker demo ships with a placeholder Google OAuth Web client inside `examples/tauth-demo/.env.tauth`. Replace it with your own value before sharing the stack beyond local testing.

### Example `/me` payload

Successful exchanges populate `/me` with a rich profile:

```json
{
  "user_id": "google:12345",
  "user_email": "user@example.com",
  "display": "Example User",
  "avatar_url": "https://lh3.googleusercontent.com/a/AEdFTp7...",
  "roles": ["user"],
  "expires": "2024-05-30T12:34:56.000Z"
}
```

Use the new `avatar_url` field to render signed-in UI chrome in your frontend.

---

## Multi-tenant configuration

TAuth now reads **all** configuration from a single YAML file (`config.yaml` by default). The snippet above shows the server-level keys; the example below highlights the `tenants` section. A “single-tenant deployment” is simply a file with one entry; adding more entries lets you serve multiple products from the same binary without touching CLI flags.

```yaml
tenants:
  - id: "demo"
    display_name: "Demo tenant"
    tenant_origins:
      - "https://demo.localhost"
      - "https://demo.example.com"
    google_web_client_id: "demo-client.apps.googleusercontent.com"
    cookie_domain: "demo.example.com"
    session_ttl: "30m"
    refresh_ttl: "720h"
    nonce_ttl: "10m"
    allow_insecure_http: true
```

Rules enforced by the loader:

- IDs must use lowercase letters, digits, underscores, or hyphens (`demo`, `customer_b`).
- `display_name` is required so operators can distinguish tenants in logs.
- `tenant_origins` entries are validated and normalized as origins (scheme + host + optional port). Add every browser origin that should resolve to this tenant (for example `https://app.example.com`, `http://localhost:8000`). If multiple tenants share the same origin, enable the header override and send `X-TAuth-Tenant`.
- `google_web_client_id` and each TTL must be present and non-empty. Durations use Go’s `time.ParseDuration` syntax (e.g. `15m`, `720h`); zero or negative values are invalid. `cookie_domain` may be blank to issue host-only cookies (recommended locally); when provided it must be a valid registrable domain (e.g. `.example.com`).
- `session_cookie_name` / `refresh_cookie_name` must be specified for every tenant. Choose unique values per tenant to avoid overwriting each other’s cookies when they share a cookie domain (for example `app_session_notes`, `app_refresh_mpr`). Legacy stacks (such as Gravity) can keep `app_session`/`app_refresh` as long as they understand the collision risk.
- `nonce_ttl` defaults to `5m` if omitted; `allow_insecure_http` defaults to `false` and should only be `true` for localhost development. With that flag enabled, cookies downgrade to `SameSite=Lax` and omit the `Secure` bit so browsers accept them over HTTP.
- Values support shell-style environment expansion (`${TENANT_COOKIE_DOMAIN}` or `$TENANT_COOKIE_DOMAIN`) before parsing. Missing variables resolve to empty strings, so leave meaningful defaults in the file to avoid loader validation errors.

The `internal/tenants` package validates the entire file before returning domain objects, so downstream routing relies on trusted tenant definitions. Request routing works as follows:

- The resolver matches tenants by the request’s `Origin` header. Requests without an `Origin` header (or with an unknown origin) are rejected unless you enable the header override.
- For local development, non-browser clients, or shared origins, enable the optional header override (`enable_tenant_header_override: true`). When enabled, TAuth accepts either a tenant ID (`X-TAuth-Tenant: demo`) or a frontend origin (`X-TAuth-Tenant: http://localhost:8000`) as the override hint. Leave it disabled in production when every tenant owns unique origins.
- `internal/tenants.TenantMiddleware` attaches the resolved tenant to `gin.Context`; downstream handlers call `tenants.TenantFromContext` to retrieve the resolved configuration and proceed with tenant-scoped logic.
- Launch the server with `tauth --config=/path/to/config.yaml` (or export `TAUTH_CONFIG_FILE`); no other CLI flags or environment variables are required.
- Front-ends that share a single origin can still opt into an explicit tenant selection by adding `data-tenant-id="tenant-a"` to the `<script src=".../tauth.js">` tag or by calling `setAuthTenantId("tenant-a")` before `initAuthClient(...)` when you need to override the origin mapping (for example, preview builds served from the same origin). `tauth.js` automatically adds the `X-TAuth-Tenant` header to its own `/me`, `/auth/nonce`, `/auth/google`, `/auth/refresh`, and logout calls (falling back to the current page origin whenever you don’t provide a tenant ID) while leaving your product’s API traffic untouched.
- Refresh tokens, nonce pools, and the built-in demo user store are keyed by tenant ID. Session JWTs now embed a `tenant_id` claim, and the middleware rejects cookies presented under the wrong tenant so credentials cannot hop between tenants.

---

### Google nonce handling

Custom clients must follow the nonce exchange documented in [ARCHITECTURE.md#google-sign-in-exchange](ARCHITECTURE.md#google-sign-in-exchange). The README’s quick-start sticks to the happy-path view; dive into the architecture doc for the exact sequencing (nonce issuance, GIS initialization, credential exchange, and `/auth/google` expectations). The default helpers already implement the full set of guardrails.

---

## Deploy with confidence

- Works out of the box for any single registrable domain—host TAuth once and share cookies across subdomains.
- Toggle CORS (and `SameSite=None` automatically) when your UI is served from a different origin during development.
- Set `database_url` to a Postgres or SQLite DSN to store refresh tokens durably.
- Structured zap logging makes it easy to monitor sign-in, refresh, and logout flows wherever you deploy.

---

## Learn more

- Read the authoritative usage guide in [`docs/usage.md`](docs/usage.md) for end-to-end setup and integration details.
- Dive into [ARCHITECTURE.md](ARCHITECTURE.md) for endpoints, request flows, and deployment guidance.
- Read [POLICY.md](POLICY.md) for the confident-programming rules enforced across the codebase.
- Inspect `web/tauth.js` to extend UI hooks or wire additional analytics.
- Validate sessions from other Go services with [`pkg/sessionvalidator`](pkg/sessionvalidator/README.md).

---

## License

MIT (or your preferred license). Add a `LICENSE` file accordingly.

## ARCHITECTURE.md

# ARCHITECTURE

## 1. System Overview

TAuth is a single-origin authentication service that sits between Google Identity Services and your product UI. It verifies Google ID tokens, issues first-party JWT access cookies, and rotates long-lived refresh tokens. The service is written in Go (Gin router) and ships the companion browser helper `web/tauth.js`.

```
Browser ──(Google ID token)──> TAuth ──(verify)──> Google Identity Services
Browser <─(HttpOnly cookies)── TAuth ──(refresh token persistence)──> Database
```

## 2. Top-Level Layout

```
. 
├─ cmd/server/                 # Cobra CLI entrypoint (reads config.yaml, boots Gin server)
├─ internal/
│  ├─ authkit/                 # Domain logic: routes, JWT helpers, refresh stores
│  └─ web/                     # Demo user store, CORS middleware, static file serving
└─ web/                        # Embeddable tauth.js helper
```

All Go packages under `internal/` are private; only the CLI is exported.

## 3. Request and Session Flow

### 3.1 Endpoints

| Method | Path            | Responsibility                                          | Response                                    |
| ------ | --------------- | ------------------------------------------------------- | ------------------------------------------- |
| POST   | `/auth/nonce`   | Issue short-lived single-use nonce for Google exchange | `200` JSON `{ nonce }`                       |
| POST   | `/auth/google`  | Verify Google ID token, issue access + refresh cookies | `200` JSON `{ user_id, user_email, ... }`   |
| POST   | `/auth/refresh` | Rotate refresh token, mint new access cookie           | `204 No Content`                            |
| POST   | `/auth/logout`  | Revoke refresh token, clear cookies                    | `204 No Content`                            |
| GET    | `/me`           | Return profile associated with current access cookie   | `200` JSON or `401` when unauthenticated    |
| GET    | `/tauth.js` | Serve the client helper                        | `200` JavaScript                            |

These endpoints are implemented only by the TAuth server. Consuming applications should call them, not host copies.
TAuth serves no other static assets; demo pages live in the repository under `examples/` and are hosted separately for local development.

### 3.2 Cookies

- `app_session`: short-lived JWT access token (`HttpOnly`, `Secure`, `SameSite` strict by default).
- `app_refresh`: long-lived opaque refresh token (`HttpOnly`, `Secure`, `Path=/auth`).

The access cookie authenticates `/me` and any downstream protected routes. The refresh cookie is rotated on each `/auth/refresh` and revoked on `/auth/logout`.

### 3.3 Google Sign-In exchange

1. Browser obtains a Google ID token from Google Identity Services.
2. Browser requests a nonce from `/auth/nonce`, passes it to Google Identity Services via `google.accounts.id.initialize({ nonce })`, and includes the same value as `nonce_token` when posting `{ "google_id_token": "...", "nonce_token": "..." }` to `/auth/google`.
3. `MountAuthRoutes` enforces HTTPS unless `AllowInsecureHTTP` is explicitly enabled for local development.
4. `idtoken.NewValidator` validates issuer and audience against `ServerConfig.GoogleWebClientID`.
5. `UserStore.UpsertGoogleUser` persists or updates email, display name, and avatar URL, then returns the application user ID plus roles.
6. `MintAppJWT` signs a short-lived access JWT (`HS256`, issuer `ServerConfig.AppJWTIssuer`) embedding `tenant_id`, `user_avatar_url`, and the other profile claims so downstream services can verify both the user and the tenant context.
7. `RefreshTokenStore.Issue` creates a new opaque refresh token (hashed before storage) with `RefreshTTL`.
8. Helper functions set `app_session` (path `/`) and `app_refresh` (path `/auth`) cookies with `HttpOnly`, `Secure`, and configured SameSite attributes.
9. The JSON response mirrors key profile fields (including `avatar_url`) so the browser helper can hydrate UI state.

### 3.4 Browser helper handshake

`web/tauth.js` abstracts the nonce and credential exchange, but custom front-ends can implement the same flow with a small wrapper around Google Identity Services:

```js
let pendingNonce = "";

async function prepareGoogleSignIn(baseUrl, clientId) {
  const response = await fetch(`${baseUrl}/auth/nonce`, {
    method: "POST",
    credentials: "include",
    headers: { "X-Requested-With": "XMLHttpRequest" },
  });
  if (!response.ok) {
    throw new Error("nonce request failed");
  }
  pendingNonce = (await response.json()).nonce;
  google.accounts.id.initialize({
    client_id: clientId,
    callback: handleCredential,
    nonce: pendingNonce,
    ux_mode: "popup",
  });
  google.accounts.id.renderButton(document.getElementById("googleSignIn"), {
    theme: "outline",
    size: "large",
    text: "signin_with",
  });
  google.accounts.id.prompt();
}

async function exchangeGoogleCredential(baseUrl, googleIdToken) {
  await fetch(`${baseUrl}/auth/google`, {
    method: "POST",
    credentials: "include",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      google_id_token: googleIdToken,
      nonce_token: pendingNonce,
    }),
  });
}
```

Nonce handling rules:

- TAuth issues one-time nonces via `POST /auth/nonce`; Google never provides one for you.
- Always supply the nonce to Google Identity Services when calling `google.accounts.id.initialize` (or via `data-nonce` on `g_id_onload`) before prompting the user.
- Echo the same nonce back to `/auth/google` as `nonce_token`. Requests without a matching nonce fail with `auth.login.nonce_mismatch`.
- Google Identity Services may hash the nonce inside the ID token (`base64url(sha256(nonce_token))`). TAuth accepts hashed or raw forms.
- Fetch a fresh nonce for every sign-in attempt. Nonces are invalidated once consumed and cannot be reused.
- The default helper (`tauth.js`) already implements these invariants; custom UIs should mirror the same flow when wiring auth state.

## 4. Components

### 4.1 `cmd/server`

- Cobra CLI with a YAML-backed configuration loader.
- Wires logging (zap), Gin middleware, CORS, routes, and graceful shutdown.
- Selects refresh token store:
  - In-memory (`authkit.NewMemoryRefreshTokenStore`) when `database_url` is empty.
  - Persistent (`authkit.NewDatabaseRefreshTokenStore`) when pointing at Postgres (`postgres://`) or SQLite (`sqlite://`), using GORM.
- Attaches `authkit.RequireSession` to protected route groups (see `/api` group in `cmd/server/main.go`).

### 4.2 `internal/authkit`

- `ServerConfig`: cookie + session settings.
- `MountAuthRoutes`: installs `/auth/*` handlers and binds stores.
- JWT helpers: signing, validation, claims modeling.
- Refresh token stores:
  - Memory implementation for tests/dev.
  - GORM-backed implementation (`DatabaseRefreshTokenStore`) that performs migrations and issues hashed refresh tokens.
- `RequireSession`: Gin middleware backed by the shared session validator; confirms issuer and injects `JwtCustomClaims` into the request context (`auth_claims`).
- Shared helpers (`refresh_token_helpers.go`) generate token IDs and opaque values consistently across store implementations.

### 4.3 `internal/web`

- `NewInMemoryUsers`: placeholder application user store (maps Google `sub` to a profile).
- `PermissiveCORS`: development-only CORS middleware.
- `ServeEmbeddedStaticJS`: serves `tauth.js` from the embedded FS.
- `HandleWhoAmI`: returns profile data for `/api/me`.

### 4.4 `web/tauth.js`

- Initializes session state via `/me`.
- Dispatches events on authentication changes.
- Attempts silent refresh on 401 using `/auth/refresh`.
- Provides hooks for UI callbacks (`onAuthenticated`, `onUnauthenticated`).
- Accepts an optional `tenantId` when calling `initAuthClient`; when present the helper attaches `X-TAuth-Tenant` to `/me`, `/auth/*`, and logout requests so multiple tenants can share an origin in development. When you omit `tenantId`, the helper now falls back to the current page origin so header overrides remain accurate even when browsers omit `Origin` on certain requests.
- Emits DOM events (`auth:authenticated`, `auth:unauthenticated`) to coordinate UI without global state.

### 4.5 Interfaces and extension points

```go
type UserStore interface {
    UpsertGoogleUser(ctx context.Context, tenantID string, googleSub string, userEmail string, userDisplayName string, userAvatarURL string) (applicationUserID string, userRoles []string, err error)
    GetUserProfile(ctx context.Context, tenantID string, applicationUserID string) (userEmail string, userDisplayName string, userAvatarURL string, userRoles []string, err error)
}

type RefreshTokenStore interface {
    Issue(ctx context.Context, tenantID string, applicationUserID string, expiresUnix int64, previousTokenID string) (tokenID string, tokenOpaque string, err error)
    Validate(ctx context.Context, tenantID string, tokenOpaque string) (applicationUserID string, tokenID string, expiresUnix int64, err error)
    Revoke(ctx context.Context, tenantID string, tokenID string) error
}
```

- Swap `UserStore` for a production datastore (e.g., Postgres) while keeping the auth kit isolated from application models.
- Implement a custom `RefreshTokenStore` (e.g., Redis, DynamoDB) by reusing the hashing helpers to maintain compatibility.
- Downstream services can read `auth_claims` and rely on `JwtCustomClaims` to authorize domain-specific operations.

### 4.6 `pkg/sessionvalidator`

- Reusable library for downstream Go services to validate the `app_session` cookie.
- Smart constructor enforces signing key and issuer configuration, with optional cookie name overrides.
- Provides `ValidateToken`, `ValidateRequest`, and a Gin middleware adapter to populate typed `Claims`.
- Shares the same claim shape (`user_id`, `user_email`, `display`, `avatar_url`, `roles`, `expires`) used by the server.
- Includes `LoadTenantAuthConfig` to derive tenant signing keys, issuer, and cookie names from the same `config.yaml` used by TAuth.

## 5. Configuration Surface

| Variable / Flag            | Purpose                                             | Example                                             |
| -------------------------- | --------------------------------------------------- | --------------------------------------------------- |
| `listen_addr`          | HTTP listen address                                 | `:8080`                                             |
| `database_url`         | Refresh store DSN (`postgres://` or `sqlite://`)    | `sqlite:///auth.db`                                 |
| `enable_cors`          | Enable permissive CORS (cross-origin dev only)      | `true` / `false`                                    |
| `cors_allowed_origins` | List of allowed origins when CORS is enabled (include GIS) | `["https://app.example.com","https://accounts.google.com"]` |
| `cors_allowed_origin_exceptions` | Non-tenant origins that may appear in `cors_allowed_origins` | `["https://accounts.google.com"]` |
| `enable_tenant_header_override` | Allow `X-TAuth-Tenant` overrides (dev/testing) | `true`                                     |
| `tenants`              | Array of tenant entries (id, tenant_origins, client IDs, TTLs) | See README §5 |

Configuration is loaded from a single YAML file (`config.yaml` by default, override via `tauth --config=/path/to/file` or `TAUTH_CONFIG_FILE`).

### 5.1 Multi-tenant configuration file

Every deployment relies on the declarative config file parsed by `internal/tenants`. The YAML document describes each tenant’s identity, origins, Google Web client, and cookie/scheduling knobs:

```yaml
tenants:
  - id: "demo"
    display_name: "Demo tenant"
    tenant_origins:
      - "https://demo.localhost"
      - "https://app.example.com"
    google_web_client_id: "demo-client.apps.googleusercontent.com"
    jwt_signing_key: "demo-signing-key"
    cookie_domain: "demo.example.com"
    session_ttl: "30m"
    refresh_ttl: "720h"
    nonce_ttl: "10m"
    allow_insecure_http: true
```

Validation rules baked into the loader:

- IDs use lowercase letters/digits/underscores/hyphens; duplicates are rejected.
- `display_name` is required so operators can identify tenants in logs.
- Origins are normalized to lowercase and deduplicated within each tenant definition. Entries must be full origins (scheme + host + optional port). When multiple tenants share the same origin, the runtime requires `X-TAuth-Tenant` to be enabled so requests can declare their tenant explicitly.
- `google_web_client_id` must be present for every tenant. Each tenant also requires its own `jwt_signing_key`; the server rejects definitions that omit it. TTLs follow Go’s `time.ParseDuration` syntax. `cookie_domain` may be blank to emit host-only cookies (required for `localhost`); otherwise provide a registrable domain (e.g. `.example.com`). `session_cookie_name` / `refresh_cookie_name` are mandatory; set them explicitly per tenant (for example `app_session_notes`, `app_refresh_notes`). Reuse the legacy `app_session`/`app_refresh` names only when you intentionally want multiple tenants to share the same cookies.
- `nonce_ttl` defaults to `5m` when omitted; `allow_insecure_http` defaults to `false`.
- Before decoding, the loader expands environment variables (`$VAR` / `${VAR}`) inside the YAML so operator templates can stay DRY. Unset variables resolve to empty strings, triggering the same validation rules as blank values.

Tenant resolution & runtime:

- `internal/tenants.NewResolver` consumes the validated config and maps HTTP requests to tenants. Origins are matched case-insensitively, and unknown origins are rejected with a 404 response before hitting auth routes. When multiple tenants intentionally share the same origin, enable the header override and send `X-TAuth-Tenant` to disambiguate.
- Local and development tooling can opt into the `X-TAuth-Tenant` override header (configurable via `WithHeaderOverride`/`--enable_tenant_header_override`) when requests lack `Origin` headers or when multiple tenants share a single origin. The override accepts either tenant IDs or frontend origins. Leave it disabled in production where origins stay unique.
- `internal/tenants.TenantMiddleware` injects the resolved tenant into `gin.Context` so auth routes and stores can look up per-tenant keys (`tenants.TenantFromContext`) without touching global state.
- Multi-tenant mode is always enabled via the `tenants` array inside `config.yaml`. Launch TAuth with `tauth --config=/path/to/config.yaml` (or set `TAUTH_CONFIG_FILE`). Use `enable_tenant_header_override: true` in local/testing environments when you need to override tenants via headers instead of origins.
- Front-ends pass `tenantId` to `initAuthClient` when they need to pin a tenant explicitly; the helper automatically sets the `X-TAuth-Tenant` header on its own `/me`, `/auth/*`, and logout requests to line up with the override flow above while leaving product APIs untouched. When no tenant ID is supplied, the helper falls back to the page origin so shared-origin setups work without extra wiring.
- All per-tenant server configs live inside `authkit.TenantRegistry`, which backs `MountAuthRoutes` and `RequireSession` so cookies, TTLs, and SameSite/AllowInsecure decisions reflect the resolved tenant.
- Refresh token stores, nonce pools, and in-memory user stores are keyed by tenant ID, and JWT sessions embed a `tenant_id` claim that `RequireSession` verifies against the resolved tenant to prevent cross-tenant cookie replay. Front-end clients normally rely on origins, but when multiple tenants share the same origin (local dev boxes, automation rigs) you can enable the header override and pass `tenantId` to `initAuthClient`. The helper adds `X-TAuth-Tenant` to `/me`, `/auth/*`, and logout requests without touching product APIs so you can switch tenants without DNS changes.

## 6. Persistence Model

The persistent refresh token store manages the `refresh_tokens` table (automigrated via GORM):

```sql
CREATE TABLE IF NOT EXISTS refresh_tokens (
    token_id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_unix BIGINT NOT NULL,
    revoked_at_unix BIGINT NOT NULL DEFAULT 0,
    previous_token_id TEXT NOT NULL DEFAULT '',
    issued_at_unix BIGINT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_hash ON refresh_tokens (token_hash);
CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user ON refresh_tokens (user_id);
```

Opaque refresh tokens are hashed (`SHA-256`, Base64 URL) before storage. Each refresh rotation inserts the new token, links it to the previous ID, and marks older tokens revoked.

`DatabaseRefreshTokenStore` parses the database URL to select a GORM dialector (`postgres` or the CGO-free `github.com/glebarez/sqlite`), silences default logging, auto-migrates the schema, and tags errors with context (`refresh_store.*`) for observability. For SQLite, only triple-slash absolute paths (`sqlite:///data/tauth.db`) or opaque memory URLs (`sqlite://file::memory:?cache=shared`) are accepted; host-prefixed forms such as `sqlite://file:/data/tauth.db` are rejected. Shared helpers ensure memory and persistent stores derive token IDs and hashes identically.

## 7. Security Considerations

- Always run behind HTTPS in production; set a tenant’s `allow_insecure_http` to `true` only for local development.
- Access cookies are short-lived; refresh cookies survive longer but are `HttpOnly` and scoped to `/auth`.
- Validate Google tokens strictly: issuer, audience, expiry, issued-at.
- Rate limit `/auth/google` and `/auth/refresh` and monitor failures via zap logs.
- Require nonce tokens from `/auth/nonce` for every Google Sign-In exchange and treat missing or mismatched nonces as unauthorized.
- Rotate each tenant's `jwt_signing_key` using standard secrets management practices.
- Only hashed refresh tokens are stored—never persist the raw opaque value.
- Serve browser code through `/tauth.js` and avoid inline scripts to keep CSP-friendly deployments.

## 8. Local Development Modes

### 8.1 Same Origin (recommended)

- Serve UI and API from the same host; keep `enable_cors` set to `false`.
- Cookies remain `SameSite=Strict`.

### 8.2 Split Origin (local labs)

- UI: `http://localhost:5173`, API: `http://localhost:8080`.
- Set `enable_cors: true` and mark the tenant’s `allow_insecure_http` as `true`.
- Browser will require HTTPS + `SameSite=None` in production for cross-origin cookies.

## 9. CLI and Server Lifecycle

- Cobra command `tauth` reads configuration from a single YAML file (`--config=/path/to/config.yaml` or `TAUTH_CONFIG_FILE`).
- `tauth preflight --config=...` validates configuration and emits a versioned, redacted effective-config report (with dependency readiness) for external validators before launch, built on the shared `github.com/tyemirov/utils/preflight` builder.
- Graceful shutdown listens for `SIGINT`/`SIGTERM`, allowing 10s for in-flight requests.
- zap middleware logs method, path, status, IP, and latency for each request.
- Integration tests use the exported CLI wiring to spin up in-memory servers (`go test ./...`).

## 10. Dependency Highlights

- **Web framework**: `github.com/gin-gonic/gin` for routing/middleware.
- **Configuration**: `spf13/viper` + `spf13/cobra` for flags and environment merging.
- **Google verification**: `google.golang.org/api/idtoken`.
- **JWT**: `github.com/golang-jwt/jwt/v5` with HS256 signatures.
- **Persistence**: `gorm.io/gorm` with `gorm.io/driver/postgres` and the CGO-free `github.com/glebarez/sqlite`.
- **Logging**: `go.uber.org/zap` (production configuration).
- **Testing**: standard library `httptest` plus the memory refresh store for fast integration coverage.

## 11. Troubleshooting Playbook

- **401 on `/me` but refresh succeeds** – Access cookie expired; the client will refresh on next call.
- **401 on `/auth/refresh`** – Refresh cookie missing/expired/revoked; prompt user to sign in again.
- **Cookies missing** – Verify the tenant’s `cookie_domain`, HTTPS usage, and CORS settings.
- **Google token rejection** – Confirm OAuth client type (Web) and that `aud` matches configured client ID.

## 12. Versioning Contract

The following surface area is considered stable across releases:

- Endpoints: `/auth/nonce`, `/auth/google`, `/auth/refresh`, `/auth/logout`, `/me`.
- Cookie names: `app_session`, `app_refresh`.
- JSON payload fields returned to the client (`user_id`, `user_email`, `display`, `roles`, `expires`).

Update the embedded client and bump the service version together when changing these contracts.

## usage.md

# TAuth Usage Guide

This document is the authoritative guide for operators and front‑end teams integrating against a TAuth deployment. It explains how to run the service, how sessions work, and how to connect a browser application using either the provided helper script or direct HTTP calls.

For a deep dive into internal architecture and implementation details, see `ARCHITECTURE.md`. For confident‑programming and refactor policies, see `POLICY.md` and `docs/refactor-plan.md`.

---

## 1. What TAuth provides

TAuth sits between Google Identity Services (GIS) and your product UI:

- Verifies Google ID tokens issued by a Google OAuth Web client.
- Mints short‑lived access cookies and long‑lived refresh cookies.
- Rotates refresh tokens on every refresh call and revokes them on logout.
- Exposes a small HTTP API and a browser helper (`/tauth.js`) for zero-token-in-JavaScript sessions.

Once TAuth is running for a given registrable domain, any app on that domain (or its subdomains) can rely on the `HttpOnly` session cookies instead of storing tokens in `localStorage` or JavaScript memory.

---

## 2. Running the service

### 2.1 Binary layout

The `tauth` binary lives under `cmd/server` in this repository. You can:

- Build it directly with Go (e.g. `go build ./cmd/server`), or
 - Use the provided Docker setup in `examples/tauth-demo` for a local stack.

The binary reads configuration exclusively from a YAML file (default `config.yaml`). Use `tauth --config=/path/to/config.yaml` or export `TAUTH_CONFIG_FILE` to point at a different file; no other environment variables or CLI flags are required.

### 2.2 Core configuration

`config.yaml` must include the server-level keys below plus at least one tenant:

| Key | Purpose | Example |
| --- | --- | --- |
| `listen_addr` | HTTP listen address | `:8080` |
| `database_url` | Refresh store DSN | `sqlite:///data/tauth.db` |
| `enable_cors` | Enable CORS for cross-origin UIs | `true` / `false` |
| `cors_allowed_origins` | Allowed origins when CORS is enabled (include your UI origins *and* `https://accounts.google.com`) | `["https://app.example.com","https://accounts.google.com"]` |
| `cors_allowed_origin_exceptions` | Allowed non-tenant origins that may appear in `cors_allowed_origins` | `["https://accounts.google.com"]` |
| `enable_tenant_header_override` | Allow `X-TAuth-Tenant` overrides (dev/local only) | `true` / `false` |
| `tenants` | Array of tenant entries (see README §5.1 for schema) | `[...]` |

Key notes:

- **TLS and cookies**: In production, terminate TLS at the load balancer or the service so cookies can be marked `Secure`. Each tenant defines its own `cookie_domain`; use that field (e.g. `.example.com`) to share cookies across subdomains. Leave the field blank to emit host-only cookies during `localhost` development (browsers reject `Domain=localhost`).
- **Database URL**: For SQLite, use triple‑slash absolute paths (`sqlite:///data/tauth.db`). Host‑based forms such as `sqlite://file:/data/tauth.db` are rejected. For Postgres, use a standard DSN (`postgres://user:pass@host:5432/dbname?sslmode=disable`).
- **CORS**: Leave `enable_cors` set to `false` when UI and API share the same origin. Enable it only when your UI is on a different origin (for example, Vite dev server) and set `cors_allowed_origins` explicitly. If you include non-tenant origins (for example `https://accounts.google.com`), also list them under `cors_allowed_origin_exceptions` so validation permits them.
- **Shared origins**: If two tenants intentionally share the same origin (typical for localhost demos), add each frontend origin (`http://localhost:8000`, `http://localhost:4173`, …) to the tenant’s `tenant_origins`. TAuth inspects the request `Origin` header to resolve the tenant automatically. You can still enable `enable_tenant_header_override` and send `X-TAuth-Tenant` when you want to override the origin mapping manually.
- **Per-tenant signing keys**: Each tenant block must declare a `jwt_signing_key`. TAuth uses that HS256 secret exclusively for the tenant’s cookies, so rotate keys per tenant instead of relying on a global fallback.
- **Local HTTP mode**: Setting `allow_insecure_http: true` on a tenant drops the `Secure` flag and downgrades cookies to `SameSite=Lax` so browsers keep them over HTTP even while CORS is enabled. This only works when your dev UI also runs on `http://localhost` (same host, different port); switching hosts such as `127.0.0.1` will make the browser treat the request as cross-site and block the cookies.

### 2.3 Example: hosted deployment

This example mirrors the README but focuses on the minimum you need to host TAuth at `https://auth.example.com` for a product UI at `https://app.example.com`:

```bash
cat > config.yaml <<'YAML'
server:
  listen_addr: ":8443"
  database_url: "sqlite:///data/tauth.db"
  enable_cors: true
  cors_allowed_origins:
    - "https://app.example.com"
    - "https://accounts.google.com"
  cors_allowed_origin_exceptions:
    - "https://accounts.google.com"
  enable_tenant_header_override: false

tenants:
  - id: "prod"
    display_name: "Production Tenant"
    tenant_origins:
      - "https://app.example.com"
    google_web_client_id: "your_web_client_id.apps.googleusercontent.com"
    jwt_signing_key: "replace-with-your-tenant-signing-key"
    cookie_domain: ".example.com"
    session_ttl: "15m"
    refresh_ttl: "1440h"
    nonce_ttl: "5m"
    allow_insecure_http: false
YAML

tauth --config=config.yaml
```

Run this behind TLS so the service issues `Secure` cookies and the browser accepts them.

When migrating an existing tenant that expects the legacy cookie names (`app_session`, `app_refresh`), set the `session_cookie_name` / `refresh_cookie_name` fields inside the tenant block. These fields are always required—choose unique names per tenant to avoid collisions when multiple tenants share `localhost`. Legacy stacks (such as Gravity) can keep `app_session` / `app_refresh`, but doing so means any other tenant using the same names will overwrite those cookies.

### 2.4 Example: local quick‑start (Docker Compose)

For a full local stack (TAuth + demo UI) without installing Go:

1. `cd examples/tauth-demo`
2. Edit `.env.tauth` (set `TAUTH_CONFIG_FILE=/config/config.yaml` and the per-tenant `TAUTH_GOOGLE_WEB_CLIENT_ID` / `TAUTH_JWT_SIGNING_KEY` values).
3. Review `config.yaml` and replace the placeholder Google OAuth client with one registered for `http://localhost:8000` and `http://localhost:8082` (or keep the environment variable references from step 2).
4. Start the stack: `docker compose up --build`
5. Visit `http://localhost:8000` for the demo UI. It talks to TAuth at `http://localhost:8082`.

Stop the stack with `docker compose down`. The `tauth_data` volume holds the SQLite database, and `config.yaml` stays next to the compose file for future edits.

### 2.5 Preflight validation (pre-start)

Use the preflight command to validate configuration and emit a redacted effective-config report before you launch the service:

```bash
tauth preflight --config=config.yaml
```

The report includes effective server settings, per-tenant cookie names and TTLs, derived SameSite modes, and JWT signing key fingerprints (never raw keys). Redacted reports still emit `tenant_origin_hashes` and `jwt_signing_key_fingerprint` so external validators can compare secrets without exposing them. To include the raw `tenant_origins` list, pass `--include-origins`.

The JSON payload is versioned and shaped as:
- `schema_version`, `service` metadata
- `effective_config` (server + tenant settings)
- `dependencies` (preflight checks with readiness status)

The preflight builder is generalized under `github.com/tyemirov/utils/preflight` with a Viper-based adapter (`github.com/tyemirov/utils/preflight/viperconfig`) for services that load YAML configs and bind env vars through Viper.

---

## 3. Sessions and cookies

TAuth works with two cookies:

- `app_session` – short‑lived JWT access token.
  - `HttpOnly`, `Secure`, `SameSite` (strict by default).
  - Sent with all requests under the configured cookie domain.
- `app_refresh` – opaque refresh token.
  - `HttpOnly`, `Secure`, `Path=/auth`.
  - Rotated on `/auth/refresh` and revoked on `/auth/logout`.

Your product should:

- Use `app_session` to protect routes (for example via `pkg/sessionvalidator` in other Go services).
- Never store tokens in JavaScript; rely on these cookies.
- Call `/auth/refresh` when API calls return `401` to keep sessions alive.

---

## 4. Recommended integration: `tauth.js`

The simplest way to use TAuth from the browser is through the helper served at `/tauth.js`. It exports eight globals:

- `initAuthClient(options)` – hydrates the current user and sets up refresh behaviour.
- `apiFetch(url, init)` – wrapper around `fetch` that automatically refreshes sessions on `401`.
- `getCurrentUser()` – returns the current profile object or `null`.
- `getAuthEndpoints()` – returns the resolved URL map for `/me` and `/auth/*`.
- `requestNonce()` – fetches a one-time nonce for Google Identity Services.
- `exchangeGoogleCredential({ credential, nonceToken })` – exchanges the Google credential for cookies and updates the profile.
- `logout()` – revokes the refresh token and clears client state.
- `setAuthTenantId(tenantId)` – sets the tenant override for subsequent requests.

For backend services written in Go, use the `pkg/sessionvalidator` package described in section 6.8 to validate `app_session` cookies.

### 4.1 Loading the helper

On your product site, include the script from wherever you host the asset:

```html
<script
  src="https://tauth.mprlab.com/tauth.js"
  data-tenant-id="tenant-admin"
></script>
```

### 4.2 Initialising on page load

Call `initAuthClient` once during startup, after the script loads. The `baseUrl` option is required and must point at your TAuth API origin:

```html
<script>
  // Optional: override tenant dynamically when the page knows which tenant to use.
  setAuthTenantId("tenant-admin");
  initAuthClient({
    baseUrl: "https://auth.example.com",
    tenantId: "demo", // optional override for shared-origin dev setups
    onAuthenticated(profile) {
      renderDashboard(profile);
    },
    onUnauthenticated() {
      showSignInButton();
    },
  });
</script>
```

Behaviour:

- TAuth calls `GET /me` to check for an existing session.
- If missing or expired, it attempts `POST /auth/refresh`.
- If refresh succeeds, it calls `onAuthenticated(profile)`; otherwise it calls `onUnauthenticated()`.
- The `profile` object matches the `/me` response (see section 6.3).

### 4.3 Calling your own APIs with `apiFetch`

Wrap all authenticated HTTP requests through `apiFetch`:

```js
async function loadProtectedData() {
  const response = await apiFetch("/api/data", { method: "GET" });
  if (!response.ok) {
    throw new Error("request_failed");
  }
  return response.json();
}
```

When a call returns `401`, `apiFetch`:

1. Sends `POST /auth/refresh` with `credentials: "include"`.
2. Retries the original request on success.
3. Broadcasts `"refreshed"` events via `BroadcastChannel` (if available), allowing multiple tabs to stay in sync.

If refresh fails, pending requests reject and callers can treat this as “logged out”.

### 4.4 Logging out

Use `logout()` to terminate the session:

```js
async function handleLogoutClick() {
  await logout();
  redirectToLanding();
}
```

The helper:

- Calls `POST /auth/logout` to revoke the refresh token.
- Clears local profile state.
- Broadcasts `"logged_out"` to other tabs.
- Invokes `onUnauthenticated()` if provided.

### 4.5 Selecting a tenant explicitly

Most deployments rely on the request `Origin` header to resolve tenants. When multiple tenants intentionally share the same origin (for example, several apps pointing at `http://localhost:8080`) or when requests omit `Origin` (non-browser clients), enable the TAuth server’s header override (`--enable_tenant_header_override`). Once enabled, the helper tags `/me` and `/auth/*` calls with either your explicit `tenantId` or, when omitted, the current page origin so shared-origin setups continue to function even if certain requests omit `Origin`. You can still pin a specific tenant explicitly by passing `tenantId` to `initAuthClient`:

```js
initAuthClient({
  baseUrl: "https://auth-dev.example.com",
  tenantId: "team-blue",
  onAuthenticated: hydrateDashboard,
  onUnauthenticated: showGoogleButton,
});
```

The helper automatically attaches `X-TAuth-Tenant: team-blue` (or the current page origin when no ID is supplied) to `/me`, `/auth/nonce`, `/auth/google`, `/auth/refresh`, and logout requests while leaving your own API traffic alone. Switch tenants by reinitialising with a different `tenantId` (or prefer separate origins when possible). The override still resolves against the configured tenant list, so unknown tenant IDs or origins are rejected.

---

## 5. Google Identity Services flow

TAuth assumes a GIS **Web** client using the popup flow. A nonce protects each sign‑in exchange.

### 5.1 Configure GIS

1. Create (or reuse) a Google OAuth Web client.
2. Add all product origins (for example `https://app.example.com`) to **Authorized JavaScript origins**.
3. Load the GIS script:

   ```html
   <script src="https://accounts.google.com/gsi/client" async defer></script>
   <div id="googleSignIn"></div>
   ```

### 5.2 Nonce and credential exchange

The required sequence for custom clients is:

1. **Nonce** – `POST /auth/nonce`
   - Returns `{ "nonce": "<random>" }`.
2. **Initialize GIS** with the nonce:
   - `google.accounts.id.initialize({ client_id, nonce, ux_mode: "popup", callback })`.
3. **Show the button / popup** via GIS APIs.
4. **Exchange credential** – when GIS invokes your callback with `response.credential`:
   - Call `POST /auth/google` with JSON `{ "google_id_token": "<response.credential>", "nonce_token": "<same nonce>" }` and `credentials: "include"`.
5. TAuth:
   - Validates the ID token against the resolved tenant’s `google_web_client_id`.
   - Verifies the nonce (raw or hashed) and the issuer.
   - Issues `app_session` and `app_refresh` cookies.
   - Returns a profile JSON payload.

> You must fetch a fresh nonce for every sign‑in attempt. TAuth invalidates a nonce as soon as it is used.

When using `tauth.js` or the mpr‑ui header component, this flow is handled internally; you only need to surface the Google button and configure your client ID.

---

## 6. HTTP endpoints

This section documents the public HTTP surface from a client’s perspective. See `ARCHITECTURE.md` for a stable contract summary and versioning notes. These endpoints are served exclusively by the TAuth server; consuming applications should call them, not reimplement them.

### 6.1 `POST /auth/nonce`

Issues a one‑time nonce for the next GIS exchange.

- **Request**: empty JSON body. Include `credentials: "include"` if you want to reuse cookies on same origin.
- **Response**: `200 OK` with JSON:

  ```json
  { "nonce": "..." }
  ```

### 6.2 `POST /auth/google`

Verifies a Google ID token and mints cookies.

- **Request body**:

  ```json
  {
    "google_id_token": "<id_token_from_gis>",
    "nonce_token": "<nonce_from_/auth/nonce>"
  }
  ```

- **Response**: `200 OK` with user profile JSON (see `/me` below). Sets `app_session` and `app_refresh` cookies.

Common failure cases:

- Invalid or expired ID token (`401`).
- Mismatched nonce (`401`).
- Audience (`aud`) does not match the resolved tenant’s `google_web_client_id` (`401`).

### 6.3 `GET /me`

Returns the profile associated with the current session.

- **Auth**: requires a valid `app_session` cookie.
- **Response**:

  ```json
  {
    "user_id": "google:12345",
    "user_email": "user@example.com",
    "display": "Example User",
    "avatar_url": "https://lh3.googleusercontent.com/a/...",
    "roles": ["user"],
    "expires": "2024-05-30T12:34:56.000Z"
  }
  ```

- **Errors**: `401` when the access cookie is missing, expired, or invalid.

### 6.4 `POST /auth/refresh`

Rotates the refresh token and mints a new access cookie.

- **Auth**: requires a valid `app_refresh` cookie.
- **Request body**: empty.
- **Response**: `204 No Content` on success. Sets new `app_session` and `app_refresh` cookies.

After a successful refresh, call `/me` again or rely on `tauth.js` to hydrate the profile.

### 6.5 `POST /auth/logout`

Revokes the refresh token and clears cookies.

- **Auth**: best‑effort; succeeds even if no valid refresh token is present.
- **Request body**: empty.
- **Response**: `204 No Content`. Clears `app_session` and `app_refresh`.

Clients should treat this as “signed out” regardless of prior state.

### 6.6 `GET /tauth.js`

Serves the browser helper described in section 4.

- Include it via `<script src="https://your-tauth-origin/tauth.js"></script>`.
- Exposes `initAuthClient`, `apiFetch`, `getCurrentUser`, `getAuthEndpoints`, `requestNonce`, `exchangeGoogleCredential`, `logout`, and `setAuthTenantId` on `window`.
- The TAuth service serves only API endpoints plus `/tauth.js`; demo pages live in `examples/` and are served separately.

## 6.7 Validating sessions from other Go services

Downstream Go services that share the TAuth cookie domain can validate `app_session` cookies directly using the `pkg/sessionvalidator` package. This is the recommended way to enforce authentication and read identity information without duplicating JWT logic.
If your service can read the same `config.yaml` as TAuth, call `LoadTenantAuthConfig` to derive the tenant’s signing key, issuer, and cookie names before constructing a validator.

### 6.7.1 Basic validator setup

Add the module to your Go service and construct a validator at startup:

```go
import (
    "os"

    "github.com/tyemirov/tauth/pkg/sessionvalidator"
)

func newSessionValidator() (*sessionvalidator.Validator, error) {
    signingKey := []byte(os.Getenv("TAUTH_NOTES_JWT_SIGNING_KEY"))
    return sessionvalidator.New(sessionvalidator.Config{
        SigningKey: signingKey,
        Issuer:     "tauth",
        // CookieName: optional; defaults to "app_session".
    })
}
```

The configuration mirrors your TAuth deployment:

- `SigningKey` must match the `jwt_signing_key` configured for the tenant whose cookies you validate.
- `Issuer` must match the issuer configured by the server (typically `"tauth"`; see `ARCHITECTURE.md`).
- `CookieName` defaults to `app_session` and should only be overridden if you have customised the cookie name on the TAuth side.

The constructor validates configuration up front and returns a typed error if required fields are missing.

### 6.7.2 Gin middleware integration

For Gin-based services, use the built-in middleware to protect routes and attach claims to the context:

```go
import (
    "log"

    "github.com/gin-gonic/gin"
    "github.com/tyemirov/tauth/pkg/sessionvalidator"
)

func main() {
    validator, err := newSessionValidator()
    if err != nil {
        log.Fatalf("invalid validator configuration: %v", err)
    }

    router := gin.Default()
    router.Use(validator.GinMiddleware(sessionvalidator.DefaultContextKey))

    router.GET("/me", func(context *gin.Context) {
        claimsValue, exists := context.Get(sessionvalidator.DefaultContextKey)
        if !exists {
            context.AbortWithStatus(http.StatusUnauthorized)
            return
        }
        claims := claimsValue.(*sessionvalidator.Claims)
        context.JSON(http.StatusOK, map[string]interface{}{
            "user_id":    claims.GetUserID(),
            "user_email": claims.GetUserEmail(),
            "display":    claims.GetUserDisplayName(),
            "avatar_url": claims.GetUserAvatarURL(),
            "roles":      claims.GetUserRoles(),
        })
    })

    _ = router.Run()
}
```

Key points:

- The middleware reads the `app_session` cookie from each request, validates it, and aborts with `401` when invalid.
- On success, it stores a `*sessionvalidator.Claims` value in the Gin context under the provided key (default `auth_claims`).
- Handler code can safely cast this value and use the helper methods (`GetUserID`, `GetUserEmail`, `GetUserDisplayName`, `GetUserAvatarURL`, `GetUserRoles`, `GetExpiresAt`) to drive authorization and UI decisions.

### 6.8.3 Manual validation flows

If you are not using Gin, or you need finer-grained control, use the lower-level helpers:

- `ValidateRequest(*http.Request)` – validates the session cookie on an incoming request and returns `*Claims`.
- `ValidateToken(string)` – validates a raw JWT string, for example when the token is forwarded between services.

Example with `net/http`:

```go
func handleProtectedRoute(response http.ResponseWriter, request *http.Request, validator *sessionvalidator.Validator) {
    claims, err := validator.ValidateRequest(request)
    if err != nil {
        http.Error(response, "unauthorized", http.StatusUnauthorized)
        return
    }
    // Use claims.* accessors here.
}
```

Using the shared validator keeps your services aligned with TAuth’s JWT format and validation rules, and avoids duplicating cryptographic or time-based logic across codebases.

---

## 7. Typical flows

### 7.1 First sign‑in

1. User clicks “Sign in with Google”.
2. UI calls `/auth/nonce`, configures GIS with the nonce, and shows the popup.
3. GIS returns a credential; UI posts it to `/auth/google`.
4. TAuth validates the token, issues cookies, returns profile JSON.
5. UI renders signed‑in state and begins using `apiFetch` for protected calls.

### 7.2 Silent refresh

1. An API call via `apiFetch` returns `401`.
2. `apiFetch` sends `POST /auth/refresh` with the refresh cookie.
3. On success, it retries the original request and broadcasts `"refreshed"`.
4. UI continues to operate with renewed session cookies.

### 7.3 Logout

1. User clicks “Sign out”.
2. UI calls `logout()`.
3. TAuth revokes the refresh token and clears cookies.
4. Helper broadcasts `"logged_out"`; all tabs transition to unauthenticated state.

---

## 8. Troubleshooting

Use this checklist when integrating:

- **401 from `/me` but refresh works** – Session cookie expired; ensure your client either uses `tauth.js` or calls `/auth/refresh` before retrying.
- **401 from `/auth/refresh`** – Refresh cookie missing or revoked; treat as “signed out” and prompt the user to sign in again.
- **No cookies set** – Verify:
  - The response comes from HTTPS (in production).
  - The tenant’s `cookie_domain` matches the registrable domain you expect.
  - CORS is configured correctly when using a split origin (`enable_cors` and `cors_allowed_origins` in `config.yaml`).
- **Google rejects the client or TAuth rejects the token** – Confirm:
  - The OAuth client type is **Web**.
  - All relevant origins are in the **Authorized JavaScript origins** list.
  - The `aud` claim in the ID token matches the tenant’s `google_web_client_id`.

For more detailed operational guidance, refer to the troubleshooting section in `ARCHITECTURE.md`.
- When multiple tenants share the same origin, list each frontend origin under `tenant_origins` so TAuth can resolve the tenant from the `Origin` header. You can still override the mapping by adding `data-tenant-id="tenant-id"` to the script tag (see 4.1) or by calling `setAuthTenantId("tenant-id")` before `initAuthClient(...)`. The helper automatically sends `X-TAuth-Tenant` whenever you opt into an explicit override, and now falls back to the page origin when no tenant ID is provided.

## README.md

# Session Validator

The `sessionvalidator` package lets downstream Go services consume the session
cookie issued by TAuth. It verifies the HS256 signature, issuer, and time-based
claims, and can be wrapped as Gin middleware for easy route protection.

```go
package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"github.com/tyemirov/tauth/pkg/sessionvalidator"
)

func main() {
	validator, err := sessionvalidator.New(sessionvalidator.Config{
		SigningKey: []byte(os.Getenv("TAUTH_NOTES_JWT_SIGNING_KEY")),
		// CookieName defaults to app_session.
	})
	if err != nil {
		log.Fatalf("invalid validator configuration: %v", err)
	}

	router := gin.Default()
	router.Use(validator.GinMiddleware("claims"))
	router.GET("/me", func(context *gin.Context) {
		claimsValue, _ := context.Get("claims")
		claims := claimsValue.(*sessionvalidator.Claims)
		context.JSON(200, gin.H{
			"user_id": claims.GetUserID(),
			"email":   claims.GetUserEmail(),
		})
	})
	_ = router.Run()
}
```

Consumers should never set an issuer; the validator uses the TAuth issuer
automatically, so pass only the signing key and cookie name (if you override it).

If you already have access to the same `config.yaml` used by TAuth, call
`LoadTenantAuthConfig` to derive the signing key and cookie names for a given
tenant ID, then pass `ValidatorConfig()` into `New`.

## Features

- Smart constructor validates configuration up front.
- `ValidateToken` and `ValidateRequest` helpers for manual flows.
- Gin middleware adapter with configurable context key.
- Exposes typed claims struct matching TAuth’s JWT payload (user id, email,
  display name, avatar URL, roles, expiry metadata).
- Helper to load tenant-specific signing keys and cookie names from the TAuth
  config file.

## Testing

```bash
go test ./pkg/sessionvalidator/...
```

