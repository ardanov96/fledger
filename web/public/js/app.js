// web/public/js/app.js
// =============================================================================
// FMCG Wallet Dashboard — combined JS (api + auth + views + app)
// Single-file to minimize HTTP roundtrips and keep demo self-contained.
// All requests go to /v1/* which the dev server (web/server.js) proxies to API_BASE_URL.
// =============================================================================

(function() {
  'use strict';

  // ===========================================================================
  // 1) API client (api.js)
  // ===========================================================================
  const TOKEN_KEY = 'fmcg.access_token';
  const REFRESH_KEY = 'fmcg.refresh_token';
  const USER_KEY = 'fmcg.user';

  const api = {
    getToken() { return localStorage.getItem(TOKEN_KEY); },
    setToken(t) { t ? localStorage.setItem(TOKEN_KEY, t) : localStorage.removeItem(TOKEN_KEY); },
    getRefresh() { return localStorage.getItem(REFRESH_KEY); },
    setRefresh(r) { r ? localStorage.setItem(REFRESH_KEY, r) : localStorage.removeItem(REFRESH_KEY); },
    getUser() { try { return JSON.parse(localStorage.getItem(USER_KEY)); } catch { return null; } },
    setUser(u) { u ? localStorage.setItem(USER_KEY, JSON.stringify(u)) : localStorage.removeItem(USER_KEY); },

    async request(path, opts = {}) {
      const headers = { 'Content-Type': 'application/json', ...(opts.headers || {}) };
      const token = this.getToken();
      if (token && !opts.skipAuth) headers['Authorization'] = `Bearer ${token}`;
      if (opts.idempotencyKey) headers['Idempotency-Key'] = opts.idempotencyKey;
      const res = await fetch(path, { ...opts, headers });
      const ct = res.headers.get('content-type') || '';
      const body = ct.includes('json') ? await res.json() : await res.text();
      if (!res.ok) {
        const err = new Error((body && body.error && body.error.message) || `HTTP ${res.status}`);
        err.status = res.status;
        err.code = body && body.error && body.error.code;
        err.body = body;
        throw err;
      }
      return body;
    },
    get(path, opts) { return this.request(path, { ...opts, method: 'GET' }); },
    post(path, body, opts) { return this.request(path, { ...opts, method: 'POST', body: body ? JSON.stringify(body) : undefined }); },
    patch(path, body, opts) { return this.request(path, { ...opts, method: 'PATCH', body: body ? JSON.stringify(body) : undefined }); },
    del(path, opts) { return this.request(path, { ...opts, method: 'DELETE' }); },

    async login(username, password) {
      const res = await this.post('/v1/auth/login', { username, password }, { skipAuth: true });
      if (res.data && res.data.type === 'tokens') {
        this.setToken(res.data.access_token);
        this.setRefresh(res.data.refresh_token);
        // Decode JWT payload (base64url) to get role + tenant_id
        try {
          const payload = JSON.parse(atob(res.data.access_token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
          this.setUser({ username, role: payload.role, tenant_id: payload.tenant_id, user_id: payload.sub });
        } catch (e) { this.setUser({ username }); }
      }
      return res;
    },
    async logout() {
      const refresh = this.getRefresh();
      try { if (refresh) await this.post('/v1/auth/logout', { refresh_token: refresh }, { skipAuth: true }); } catch (e) {}
      this.setToken(null); this.setRefresh(null); this.setUser(null);
    },
    async ping() {
      try {
        const res = await fetch('/healthz');
        return res.ok ? '🟢 Connected' : '🔴 Disconnected';
      } catch (e) { return '🔴 Unreachable'; }
    },
  };

  // ===========================================================================
  // 2) Auth flow (auth.js)
  // ===========================================================================
  const auth = {
    showLogin() {
      document.getElementById('topbar').hidden = true;
      document.getElementById('nav').innerHTML = '';
      ['view-login'].forEach(id => document.getElementById(id).hidden = false);
      ['view-dashboard','view-accounts','view-transfers','view-invoices','view-aging',
       'view-periods','view-reconciler','view-currencies','view-audit'].forEach(id => {
        const el = document.getElementById(id); if (el) el.hidden = true;
      });
      document.getElementById('user-status').textContent = '';
      api.setUser(null);
    },
    showApp(user) {
      document.getElementById('topbar').hidden = false;
      document.getElementById('view-login').hidden = true;
      document.getElementById('user-info').innerHTML =
        `<strong>${user.username}</strong> · <span class="badge">${user.role}</span>` +
        (user.tenant_id ? ` · tenant <code>${user.tenant_id.slice(0,8)}</code>` : '');
      document.getElementById('user-status').textContent = `Logged in as ${user.username}`;
      this.buildNav(user.role);
    },
    buildNav(role) {
      const all = [
        ['dashboard', '📊 Dashboard'],
        ['accounts', '🏦 Accounts'],
        ['transfers', '💸 Transfers'],
        ['invoices', '📄 Invoices'],
        ['aging', '📅 Aging'],
        ['periods', '🔄 Periods'],
        ['reconciler', '🔍 Reconciler'],
        ['currencies', '💱 Currencies'],
        ['audit', '📋 Audit'],
      ];
      const nav = document.getElementById('nav');
      nav.innerHTML = all.map(([v, l]) =>
        `<a href="#${v}" data-view="${v}">${l}</a>`
      ).join('');
      nav.querySelectorAll('a').forEach(a => a.addEventListener('click', (e) => {
        e.preventDefault();
        app.showView(a.dataset.view);
      }));
    },
  };

  // ===========================================================================
  // 3) View renderers (views.js)
  // ===========================================================================
  const views = {
    fmtMinor(minor, currency) {
      const v = (minor / 100).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
      return `${currency || ''} ${v}`.trim();
    },
    fmtDate(iso) { return iso ? new Date(iso).toLocaleString() : '—'; },
    fmtShort(iso) { return iso ? new Date(iso).toLocaleDateString() : '—'; },
    badge(status) { return `<span class="badge ${status}">${status}</span>`; },
    empty(msg) { return `<tr><td colspan="6" class="empty">${msg}</td></tr>`; },

    async dashboard() {
      const cards = document.getElementById('dashboard-cards');
      const rt = document.getElementById('recent-transfers');
      const oi = document.getElementById('open-invoices');
      try {
        const [accounts, invoices, transfers] = await Promise.all([
          api.get('/v1/accounts'),
          api.get('/v1/invoices?status=open&limit=5'),
          api.get('/v1/invoices?status=paid&limit=5'),
        ]);
        const totalBal = (accounts.data || []).reduce((s, a) => s + (a.cached_balance_minor || a.balance_minor || 0), 0);
        cards.innerHTML = `
          <div class="stat"><div class="value">${accounts.data ? accounts.data.length : 0}</div><div class="label">Accounts</div></div>
          <div class="stat"><div class="value">${invoices.data ? invoices.data.length : 0}</div><div class="label">Open Invoices</div></div>
          <div class="stat"><div class="value">${transfers.data ? transfers.data.length : 0}</div><div class="label">Paid Invoices</div></div>
        `;
        oi.innerHTML = (invoices.data && invoices.data.length)
          ? '<table class="data-table">' + this.invoicesRows(invoices.data) + '</table>'
          : '<p class="muted">No open invoices</p>';
        rt.innerHTML = '<p class="muted">See Transfers tab for full history</p>';
      } catch (e) {
        cards.innerHTML = `<p class="error">Failed to load dashboard: ${e.message}</p>`;
      }
    },

    async accounts() {
      const tbody = document.querySelector('#accounts-table tbody');
      try {
        const res = await api.get('/v1/accounts');
        if (!res.data || res.data.length === 0) {
          tbody.innerHTML = this.empty('No accounts yet — create one!');
          return;
        }
        tbody.innerHTML = res.data.map(a => `
          <tr>
            <td><code>${a.code}</code></td>
            <td>${a.name}</td>
            <td>${a.type}</td>
            <td>${a.currency}</td>
            <td class="num">${this.fmtMinor(a.cached_balance_minor || a.balance_minor || 0, a.currency)}</td>
            <td>${this.badge(a.status)}</td>
          </tr>
        `).join('');
      } catch (e) { tbody.innerHTML = this.empty(`Error: ${e.message}`); }
    },

    async transfers() {
      await this.loadAccountOptions();
      await this.refreshTransfersTable();
    },
    async loadAccountOptions() {
      try {
        const res = await api.get('/v1/accounts');
        const opts = (res.data || []).map(a => `<option value="${a.id}">${a.code} — ${a.name} (${this.fmtMinor(a.cached_balance_minor || 0, a.currency)})</option>`).join('');
        document.querySelector('select[name="from_account_id"]').innerHTML = opts;
        document.querySelector('select[name="to_account_id"]').innerHTML = opts;
      } catch (e) { console.error('load accounts', e); }
    },
    async refreshTransfersTable() {
      const tbody = document.querySelector('#transfers-table tbody');
      try {
        // No GET /transfers endpoint — show recent ones from invoices instead, or just empty
        tbody.innerHTML = this.empty('Create your first transfer using the form above. (No list endpoint in API; check audit log for full history.)');
      } catch (e) { tbody.innerHTML = this.empty(`Error: ${e.message}`); }
    },

    async invoices() {
      const tbody = document.querySelector('#invoices-table tbody');
      try {
        const res = await api.get('/v1/invoices?limit=50');
        if (!res.data || res.data.length === 0) {
          tbody.innerHTML = this.empty('No invoices yet');
          return;
        }
        tbody.innerHTML = res.data.map(inv => `
          <tr>
            <td><code>${inv.code}</code></td>
            <td>${inv.customer_id ? inv.customer_id.slice(0,8) + '…' : '—'}</td>
            <td class="num">${this.fmtMinor(inv.amount_minor, inv.currency)}</td>
            <td class="num">${this.fmtMinor(inv.paid_amount_minor, inv.currency)}</td>
            <td>${this.badge(inv.status)}</td>
            <td>${this.fmtShort(inv.due_date)}</td>
          </tr>
        `).join('');
      } catch (e) { tbody.innerHTML = this.empty(`Error: ${e.message}`); }
    },
    invoicesRows(list) {
      return list.map(inv => `
        <tr>
          <td><code>${inv.code}</code></td>
          <td class="num">${this.fmtMinor(inv.amount_minor, inv.currency)}</td>
          <td class="num">${this.fmtMinor(inv.paid_amount_minor, inv.currency)}</td>
          <td>${this.badge(inv.status)}</td>
          <td>${this.fmtShort(inv.due_date)}</td>
        </tr>
      `).join('');
    },

    async aging() {
      const tbody = document.querySelector('#aging-table tbody');
      try {
        // Pick first customer, then fetch their aging. (No list-customers endpoint.)
        const inv = await api.get('/v1/invoices?limit=1');
        if (!inv.data || inv.data.length === 0) {
          tbody.innerHTML = this.empty('No invoices available for aging analysis');
          return;
        }
        const customerID = inv.data[0].customer_id;
        const res = await api.get(`/v1/customers/${customerID}/aging`);
        const buckets = ['current','d_1_7','d_8_30','d_31_60','d_61_90','d_90_plus'];
        const data = res.data || [];
        tbody.innerHTML = buckets.map(b => {
          const row = data.find(d => d.bucket === b) || { count: 0, outstanding_minor: 0 };
          return `<tr><td>${b.replace(/_/g, ' ').replace('d ', 'Day ')}</td><td class="num">${row.count}</td><td class="num">${this.fmtMinor(row.outstanding_minor)}</td></tr>`;
        }).join('');
      } catch (e) { tbody.innerHTML = this.empty(`Error: ${e.message}`); }
    },

    async periods() {
      const el = document.getElementById('periods-list');
      el.innerHTML = '<p class="muted">Open API: <code>POST /v1/periods/{id}/close-requests</code> to request close. (Full UI workflow in next iteration.)</p>';
    },

    async reconciler() {
      const tbody = document.querySelector('#runs-table tbody');
      try {
        const res = await api.get('/v1/reconciler/runs?limit=20');
        if (!res.data || res.data.length === 0) {
          tbody.innerHTML = this.empty('No reconciler runs yet. Trigger one using the form above.');
          return;
        }
        tbody.innerHTML = res.data.map(r => `
          <tr>
            <td><code>${r.id ? r.id.slice(0,8) : '—'}</code></td>
            <td><code>${r.period_id ? r.period_id.slice(0,8) : '—'}</code></td>
            <td>${this.badge(r.status)}</td>
            <td class="num">${this.fmtMinor(r.total_debit_minor || 0)}</td>
            <td class="num">${this.fmtMinor(r.total_credit_minor || 0)}</td>
            <td class="num">${this.fmtMinor(r.imbalance_minor || 0)}</td>
            <td>${this.fmtDate(r.started_at)}</td>
          </tr>
        `).join('');
      } catch (e) { tbody.innerHTML = this.empty(`Error: ${e.message}`); }
    },

    async currencies() {
      const tbody = document.querySelector('#currencies-table tbody');
      try {
        const res = await api.get('/v1/currencies');
        if (!res.data || res.data.length === 0) {
          tbody.innerHTML = this.empty('No currencies registered');
          return;
        }
        tbody.innerHTML = res.data.map(c => `
          <tr>
            <td><code>${c.code}</code></td>
            <td>${c.name}</td>
            <td class="num">${c.decimal_places}</td>
            <td>${c.is_active ? '✅' : '❌'}</td>
          </tr>
        `).join('');
      } catch (e) { tbody.innerHTML = this.empty(`Error: ${e.message}`); }
    },

    async audit() {
      const tbody = document.querySelector('#audit-table tbody');
      try {
        const res = await api.get('/v1/audit?limit=50');
        if (!res.data || res.data.length === 0) {
          tbody.innerHTML = this.empty('No audit entries yet');
          return;
        }
        tbody.innerHTML = res.data.map(a => `
          <tr>
            <td>${this.fmtDate(a.occurred_at)}</td>
            <td>${a.actor_id ? a.actor_id.slice(0,8) : '—'}</td>
            <td>${a.action}</td>
            <td>${a.resource_type || ''} ${a.resource_id ? a.resource_id.slice(0,8) : ''}</td>
            <td>${a.status_code || ''}</td>
            <td>${a.ip_address || '—'}</td>
          </tr>
        `).join('');
      } catch (e) { tbody.innerHTML = this.empty(`Error: ${e.message}`); }
    },
  };

  // ===========================================================================
  // 4) App orchestration (app.js)
  // ===========================================================================
  const app = {
    async init() {
      // Status bar ping
      const updateStatus = async () => {
        const el = document.getElementById('api-status');
        if (el) el.textContent = await api.ping();
      };
      await updateStatus();
      setInterval(updateStatus, 30_000);

      // Check existing session
      const token = api.getToken();
      const user = api.getUser();
      if (token && user) {
        auth.showApp(user);
        this.showView('dashboard');
      } else {
        auth.showLogin();
      }

      // Login form
      document.getElementById('login-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const errEl = document.getElementById('login-error');
        errEl.hidden = true;
        try {
          await api.login(fd.get('username'), fd.get('password'));
          auth.showApp(api.getUser());
          this.showView('dashboard');
        } catch (err) {
          errEl.textContent = err.message || 'Login failed';
          errEl.hidden = false;
        }
      });

      // Logout
      document.getElementById('logout-btn').addEventListener('click', async () => {
        await api.logout();
        auth.showLogin();
      });

      // Transfer form
      document.getElementById('transfer-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const body = {
          from_account_id: fd.get('from_account_id'),
          to_account_id: fd.get('to_account_id'),
          amount_minor: parseInt(fd.get('amount_minor'), 10),
          currency: fd.get('currency'),
          description: fd.get('description') || undefined,
        };
        const errEl = document.getElementById('transfer-error');
        const okEl = document.getElementById('transfer-success');
        errEl.hidden = okEl.hidden = true;
        try {
          const idem = crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`;
          const res = await api.post('/v1/transfers', body, { idempotencyKey: idem });
          okEl.textContent = `✅ Transfer ${res.data.transaction_id} created (${this.formatAmount(body.amount_minor, body.currency)})`;
          okEl.hidden = false;
          await views.refreshTransfersTable();
        } catch (err) {
          errEl.textContent = `${err.code || 'ERROR'}: ${err.message}`;
          errEl.hidden = false;
        }
      });

      // Convert form
      document.getElementById('convert-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const body = {
          tenant_id: api.getUser()?.tenant_id,
          from_currency: fd.get('from_currency'),
          to_currency: fd.get('to_currency'),
          amount_minor: parseInt(fd.get('amount_minor'), 10),
        };
        const out = document.getElementById('convert-result');
        try {
          const res = await api.post('/v1/currencies/convert', body);
          out.textContent = JSON.stringify(res, null, 2);
        } catch (err) {
          out.textContent = `Error: ${err.message}`;
        }
      });

      // Reconciler form
      document.getElementById('reconciler-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const body = {
          tenant_id: api.getUser()?.tenant_id,
          period_id: fd.get('period_id'),
          run_hash_check: fd.get('run_hash_check') === 'on',
        };
        try {
          await api.post('/v1/reconciler/run', body);
          await views.reconciler();
        } catch (err) {
          alert(`Reconciler error: ${err.message}`);
        }
      });

      // Hash-based routing
      window.addEventListener('hashchange', () => {
        const view = window.location.hash.slice(1);
        if (view && document.getElementById('view-' + view)) this.showView(view);
      });
    },
    formatAmount(minor, currency) {
      return `${currency} ${(minor / 100).toLocaleString('en-US', { minimumFractionDigits: 2 })}`;
    },
    async showView(name) {
      const viewNames = ['dashboard','accounts','transfers','invoices','aging','periods','reconciler','currencies','audit'];
      viewNames.forEach(v => {
        const el = document.getElementById('view-' + v);
        if (el) el.hidden = (v !== name);
      });
      document.querySelectorAll('nav a').forEach(a => {
        a.classList.toggle('active', a.dataset.view === name);
      });
      window.location.hash = name;
      // Lazy-load data
      const loader = views[name];
      if (loader) try { await loader.call(views); } catch (e) { console.error(`view ${name}`, e); }
    },
  };

  // Boot
  document.addEventListener('DOMContentLoaded', () => app.init());
})();
