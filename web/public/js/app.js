// web/public/js/app.js
// =============================================================================
// FMCG Wallet Dashboard — single-file SPA (api + auth + views + app + utils)
// Vanilla JS, no framework, no build step. All requests go to /v1/*
// which the dev server (web/server.js) proxies to API_BASE_URL.
// =============================================================================

(function() {
  'use strict';

  // ===========================================================================
  // 0) Utilities — formatters, toasts, modal, DOM helpers
  // ===========================================================================

  // Money formatter: minor units (BigInt-safe) -> "Rp 1.234.567,00"
  // Supports IDR (no decimals), USD (2 decimals), generic.
  const fmtMinor = (minor, currency) => {
    if (minor == null) return '—';
    const m = typeof minor === 'string' ? BigInt(minor) : BigInt(minor);
    const negative = m < 0n;
    const abs = negative ? -m : m;
    const code = (currency || 'IDR').toUpperCase();
    const decimals = code === 'IDR' ? 0 : 2;
    const factor = decimals === 0 ? 1n : 100n;
    const major = abs / factor;
    const minor2 = decimals === 0 ? 0n : abs % factor;
    const majorStr = major.toString().replace(/\B(?=(\d{3})+(?!\d))/g, '.');
    const minorStr = decimals === 0 ? '' : ',' + minor2.toString().padStart(decimals, '0');
    const prefix = code === 'IDR' ? 'Rp ' : code + ' ';
    return (negative ? '−' : '') + prefix + majorStr + minorStr;
  };

  const fmtIDR = (minor) => fmtMinor(minor, 'IDR');

  const fmtDate = (iso) => {
    if (!iso) return '—';
    const d = new Date(iso);
    if (isNaN(d)) return iso;
    return d.toLocaleDateString('id-ID', { year: 'numeric', month: 'short', day: '2-digit' });
  };

  const fmtDateTime = (iso) => {
    if (!iso) return '—';
    const d = new Date(iso);
    if (isNaN(d)) return iso;
    return d.toLocaleString('id-ID', { year: 'numeric', month: 'short', day: '2-digit', hour: '2-digit', minute: '2-digit' });
  };

  const fmtAgo = (iso) => {
    if (!iso) return '—';
    const d = new Date(iso);
    const ms = Date.now() - d.getTime();
    const mins = Math.floor(ms / 60000);
    if (mins < 1) return 'just now';
    if (mins < 60) return `${mins}m ago`;
    const hrs = Math.floor(mins / 60);
    if (hrs < 24) return `${hrs}h ago`;
    const days = Math.floor(hrs / 24);
    if (days < 30) return `${days}d ago`;
    return fmtDate(iso);
  };

  const truncate = (s, n) => {
    if (!s) return '—';
    s = String(s);
    return s.length > n ? s.slice(0, n - 1) + '…' : s;
  };

  const badge = (status) => {
    if (!status) return '<span class="badge">—</span>';
    return `<span class="badge badge-${status}">${status}</span>`;
  };

  const escapeHTML = (s) => {
    if (s == null) return '';
    return String(s)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;')
      .replace(/'/g, '&#39;');
  };

  // Toast notifications
  const toast = (msg, type = 'info', duration = 4000) => {
    const container = document.getElementById('toast-container');
    if (!container) return;
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    el.innerHTML = `<span class="toast-icon">${
      type === 'success' ? '✅' :
      type === 'error' ? '❌' :
      type === 'warning' ? '⚠️' : 'ℹ️'
    }</span><span class="toast-msg">${escapeHTML(msg)}</span>`;
    container.appendChild(el);
    setTimeout(() => el.classList.add('toast-out'), duration - 200);
    setTimeout(() => el.remove(), duration);
    el.addEventListener('click', () => el.remove());
  };

  // Modal
  const modal = {
    show(title, html) {
      document.getElementById('modal-title').textContent = title;
      document.getElementById('modal-body').innerHTML = html;
      document.getElementById('modal-backdrop').hidden = false;
    },
    close() {
      document.getElementById('modal-backdrop').hidden = true;
      document.getElementById('modal-body').innerHTML = '';
    }
  };
  document.addEventListener('DOMContentLoaded', () => {
    document.getElementById('modal-close').addEventListener('click', () => modal.close());
    document.getElementById('modal-backdrop').addEventListener('click', (e) => {
      if (e.target.id === 'modal-backdrop') modal.close();
    });
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape') modal.close(); });
  });

  // Spinner
  const spinner = (text = 'Loading…') => `<div class="spinner-wrap"><div class="spinner"></div><span>${escapeHTML(text)}</span></div>`;
  const emptyState = (icon, title, hint = '') => `
    <div class="empty-state">
      <div class="empty-icon">${icon}</div>
      <div class="empty-title">${escapeHTML(title)}</div>
      ${hint ? `<div class="empty-hint">${escapeHTML(hint)}</div>` : ''}
    </div>`;

  // ===========================================================================
  // 1) API client
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
      const user = this.getUser();
      if (user && user.tenant_id && !opts.skipTenant) headers['X-Tenant-ID'] = user.tenant_id;
      if (opts.idempotencyKey) headers['Idempotency-Key'] = opts.idempotencyKey;

      const fetchOpts = {
        method: opts.method || 'GET',
        headers,
      };
      if (opts.body !== undefined && opts.body !== null) {
        fetchOpts.body = typeof opts.body === 'string' ? opts.body : JSON.stringify(opts.body);
      }
      const res = await fetch(path, fetchOpts);
      const ct = res.headers.get('content-type') || '';
      const body = ct.includes('json') ? await res.json() : await res.text();
      if (!res.ok) {
        const err = new Error((body && body.error && body.error.message) || `HTTP ${res.status}`);
        err.status = res.status;
        err.code = body && body.error && body.error.code;
        err.details = body && body.error && body.error.details;
        err.body = body;
        throw err;
      }
      return body;
    },
    get(path, opts) { return this.request(path, { ...opts, method: 'GET' }); },
    post(path, body, opts) { return this.request(path, { ...opts, method: 'POST', body }); },
    patch(path, body, opts) { return this.request(path, { ...opts, method: 'PATCH', body }); },
    del(path, opts) { return this.request(path, { ...opts, method: 'DELETE' }); },

    async login(tenant_id, username, password) {
      const res = await this.post('/v1/auth/login', { tenant_id, username, password }, { skipAuth: true, skipTenant: true });
      if (res.data && res.data.access_token) {
        this.setToken(res.data.access_token);
        this.setRefresh(res.data.refresh_token);
        try {
          const payload = JSON.parse(atob(res.data.access_token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
          this.setUser({
            username,
            tenant_id,
            user_id: res.data.user_id,
            role: payload.role || 'unknown',
          });
        } catch (e) {
          this.setUser({ username, tenant_id });
        }
      }
      return res;
    },

    async logout() {
      const refresh = this.getRefresh();
      try { if (refresh) await this.post('/v1/auth/logout', { refresh_token: refresh }, { skipAuth: true, skipTenant: true }); }
      catch (e) { /* ignore */ }
      this.setToken(null);
      this.setRefresh(null);
      this.setUser(null);
    },

    async ping() {
      try {
        const res = await fetch('/healthz');
        return res.ok ? '🟢 Connected' : '🔴 Disconnected';
      } catch (e) { return '🔴 Unreachable'; }
    },
  };

  // ===========================================================================
  // 2) Auth flow
  // ===========================================================================
  const auth = {
    showLogin() {
      document.getElementById('topbar').hidden = true;
      document.getElementById('nav').innerHTML = '';
      document.getElementById('view-login').hidden = false;
      ['view-dashboard','view-accounts','view-transfers','view-invoices','view-aging',
       'view-periods','view-reconciler','view-currencies','view-audit'].forEach(id => {
        const el = document.getElementById(id); if (el) el.hidden = true;
      });
      document.getElementById('user-status').textContent = '';
    },
    showApp(user) {
      document.getElementById('topbar').hidden = false;
      document.getElementById('view-login').hidden = true;
      const roleBadge = user.role && user.role !== 'unknown' ? ` <span class="badge badge-active">${user.role}</span>` : '';
      document.getElementById('user-info').innerHTML =
        `<strong>${escapeHTML(user.username)}</strong>${roleBadge}`;
      document.getElementById('user-status').textContent = `tenant: ${truncate(user.tenant_id, 13)}…`;
      this.buildNav();
    },
    buildNav() {
      const nav = [
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
      const el = document.getElementById('nav');
      el.innerHTML = nav.map(([v, l]) => `<a href="#${v}" data-view="${v}">${l}</a>`).join('');
      el.querySelectorAll('a').forEach(a => a.addEventListener('click', (e) => {
        e.preventDefault();
        app.showView(a.dataset.view);
      }));
    },
  };

  // ===========================================================================
  // 3) View renderers
  // ===========================================================================
  const views = {

    // ----- Dashboard -----
    async dashboard() {
      const cards = document.getElementById('dashboard-cards');
      const oi = document.getElementById('open-invoices');
      const ag = document.getElementById('aging-overview');
      cards.innerHTML = spinner('Loading stats…');
      oi.innerHTML = spinner();
      ag.innerHTML = spinner();

      try {
        const [accounts, openInv, partialInv, periods] = await Promise.all([
          api.get('/v1/accounts'),
          api.get('/v1/invoices?status=open&limit=10'),
          api.get('/v1/invoices?status=partial&limit=10'),
          api.get('/v1/periods'),
        ]);

        const acctList = accounts.data || [];
        const totalBal = acctList.reduce((s, a) => s + Number(a.cached_balance_minor || a.balance_minor || 0), 0);
        const cashAccounts = acctList.filter(a => a.type === 'cash');
        const cashBal = cashAccounts.reduce((s, a) => s + Number(a.cached_balance_minor || a.balance_minor || 0), 0);

        const openList = openInv.data || [];
        const partialList = partialInv.data || [];
        const openTotal = openList.reduce((s, i) => s + Number(i.amount_minor - i.paid_amount_minor), 0);
        const overdueList = acctList.filter(a => a.status === 'frozen').length;

        cards.innerHTML = `
          <div class="stat stat-primary">
            <div class="value">${fmtIDR(totalBal)}</div>
            <div class="label">Total Balance (${acctList.length} accounts)</div>
          </div>
          <div class="stat">
            <div class="value">${fmtIDR(cashBal)}</div>
            <div class="label">Cash & Bank (${cashAccounts.length})</div>
          </div>
          <div class="stat stat-warn">
            <div class="value">${fmtIDR(openTotal)}</div>
            <div class="label">Outstanding Receivables</div>
          </div>
          <div class="stat">
            <div class="value">${openList.length}</div>
            <div class="label">Open Invoices</div>
          </div>
          <div class="stat">
            <div class="value">${partialList.length}</div>
            <div class="label">Partial Invoices</div>
          </div>
          <div class="stat">
            <div class="value">${periods.data ? periods.data.length : 0}</div>
            <div class="label">Accounting Periods</div>
          </div>
        `;

        oi.innerHTML = openList.length ? `
          <table class="data-table compact">
            <thead><tr><th>Code</th><th class="num">Amount</th><th class="num">Outstanding</th><th>Due</th></tr></thead>
            <tbody>${openList.map(inv => {
              const out = inv.amount_minor - inv.paid_amount_minor;
              return `<tr>
                <td><code>${inv.code}</code></td>
                <td class="num">${fmtIDR(inv.amount_minor)}</td>
                <td class="num">${fmtIDR(out)}</td>
                <td>${fmtShortDate(inv.due_date)}</td>
              </tr>`;
            }).join('')}</tbody>
          </table>` : emptyState('📭', 'No open invoices', 'All invoices have been paid or partially paid.');

        ag.innerHTML = emptyState('📅', 'Aging data', 'Select Aging tab to view per-customer breakdown.');
      } catch (e) {
        cards.innerHTML = `<p class="error">Failed: ${escapeHTML(e.message)}</p>`;
        oi.innerHTML = '';
        ag.innerHTML = '';
      }
    },

    // ----- Accounts -----
    _allAccounts: [],
    async accounts() {
      const tbody = document.querySelector('#accounts-table tbody');
      const typeFilter = document.getElementById('filter-account-type').value;
      const searchFilter = document.getElementById('filter-account-search').value.toLowerCase();
      tbody.innerHTML = '<tr><td colspan="7" colspan="7">' + spinner('Loading accounts…') + '</td></tr>';
      try {
        const res = await api.get('/v1/accounts');
        let list = res.data || [];
        this._allAccounts = list;
        if (typeFilter) list = list.filter(a => a.type === typeFilter);
        if (searchFilter) list = list.filter(a =>
          (a.code || '').toLowerCase().includes(searchFilter) ||
          (a.name || '').toLowerCase().includes(searchFilter)
        );
        if (list.length === 0) {
          tbody.innerHTML = `<tr><td colspan="7">${emptyState('🏦', 'No accounts found', 'Try changing the filters or click "+ New Account".')}</td></tr>`;
          return;
        }
        tbody.innerHTML = list.map(a => `
          <tr>
            <td><code>${escapeHTML(a.code)}</code></td>
            <td>${escapeHTML(a.name)}</td>
            <td>${badge(a.type)}</td>
            <td>${escapeHTML(a.currency)}</td>
            <td class="num">${fmtMinor(a.cached_balance_minor || a.balance_minor, a.currency)}</td>
            <td>${badge(a.status)}</td>
            <td>${fmtAgo(a.created_at)}</td>
          </tr>`).join('');
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('❌', 'Error loading accounts', e.message)}</td></tr>`;
      }
    },

    // ----- Transfers -----
    async transfers() {
      await this.loadAccountOptions();
      document.getElementById('transfer-success').hidden = true;
      document.getElementById('transfer-error').hidden = true;
      const tbody = document.querySelector('#transfers-table tbody');
      tbody.innerHTML = `<tr><td colspan="7">${emptyState('💸', 'Create your first transfer', 'Use the form above. Transfer history appears here after creation.')}</td></tr>`;
    },

    async loadAccountOptions() {
      try {
        const res = await api.get('/v1/accounts');
        const opts = (res.data || [])
          .filter(a => a.status === 'active' && a.type === 'cash')
          .map(a => `<option value="${a.id}">${escapeHTML(a.code)} — ${escapeHTML(a.name)} (${fmtIDR(a.cached_balance_minor || 0)})</option>`)
          .join('');
        document.querySelector('select[name="from_account_id"]').innerHTML = opts || '<option value="">(no cash accounts)</option>';
        document.querySelector('select[name="to_account_id"]').innerHTML = opts || '<option value="">(no cash accounts)</option>';
      } catch (e) { console.error('load accounts', e); }
    },

    async submitTransfer(form) {
      const fd = new FormData(form);
      const amountRupiah = parseInt(fd.get('amount_rupiah'), 10);
      if (!amountRupiah || amountRupiah <= 0) {
        toast('Amount must be > 0', 'error');
        return;
      }
      const body = {
        from_account_id: fd.get('from_account_id'),
        to_account_id: fd.get('to_account_id'),
        amount_minor: amountRupiah * 100,
        currency: 'IDR',
        description: fd.get('description') || undefined,
      };
      if (body.from_account_id === body.to_account_id) {
        toast('From and To accounts must differ', 'error');
        return;
      }
      const errEl = document.getElementById('transfer-error');
      const okEl = document.getElementById('transfer-success');
      errEl.hidden = okEl.hidden = true;
      const submitBtn = form.querySelector('button[type="submit"]');
      submitBtn.disabled = true;
      submitBtn.textContent = 'Creating…';
      try {
        const idem = crypto.randomUUID ? crypto.randomUUID() : `${Date.now()}-${Math.random()}`;
        const res = await api.post('/v1/transfers', body, { idempotencyKey: idem });
        const txId = res.data?.transaction_id || res.data?.id || '?';
        okEl.innerHTML = `✅ Transfer <code>${escapeHTML(truncate(txId, 12))}</code> created: ${fmtIDR(body.amount_minor)}`;
        okEl.hidden = false;
        toast(`Transfer ${truncate(txId, 8)} created`, 'success');
        form.reset();
        await this.loadAccountOptions();
      } catch (err) {
        const msg = err.details ? JSON.stringify(err.details) : err.message;
        errEl.textContent = `${err.code || 'ERROR'}: ${msg}`;
        errEl.hidden = false;
        toast(`${err.code || 'Error'}: ${msg}`, 'error', 6000);
      } finally {
        submitBtn.disabled = false;
        submitBtn.textContent = 'Create Transfer';
      }
    },

    // ----- Invoices -----
    async invoices() {
      const tbody = document.querySelector('#invoices-table tbody');
      const statusFilter = document.getElementById('filter-invoice-status').value;
      tbody.innerHTML = `<tr><td colspan="7">${spinner()}</td></tr>`;
      try {
        const qs = statusFilter ? `?status=${statusFilter}` : '';
        const res = await api.get(`/v1/invoices${qs}`);
        const list = res.data || [];
        if (list.length === 0) {
          tbody.innerHTML = `<tr><td colspan="7">${emptyState('📄', 'No invoices', 'Click "+ New Invoice" to create one.')}</td></tr>`;
          return;
        }
        // Fetch accounts for customer names lookup
        const acctRes = await api.get('/v1/accounts');
        const acctMap = {};
        (acctRes.data || []).forEach(a => { acctMap[a.id] = a; });

        tbody.innerHTML = list.map(inv => {
          const cust = acctMap[inv.customer_id];
          const outstanding = Number(inv.amount_minor || 0) - Number(inv.paid_amount_minor || 0);
          return `<tr>
            <td><code>${escapeHTML(inv.code)}</code></td>
            <td>${cust ? escapeHTML(cust.name) : '<span class="muted">' + truncate(inv.customer_id, 12) + '</span>'}</td>
            <td class="num">${fmtIDR(inv.amount_minor)}</td>
            <td class="num">${fmtIDR(inv.paid_amount_minor)}</td>
            <td class="num">${fmtIDR(outstanding)}</td>
            <td>${badge(inv.status)}</td>
            <td>${fmtShortDate(inv.due_date)}</td>
          </tr>`;
        }).join('');
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('❌', 'Error', e.message)}</td></tr>`;
      }
    },

    async showNewInvoiceModal() {
      // Need list of customers (type='customer')
      const acctRes = await api.get('/v1/accounts');
      const customers = (acctRes.data || []).filter(a => a.type === 'customer' && a.status === 'active');
      if (customers.length === 0) {
        toast('No customers found. Seed demo data first.', 'warning');
        return;
      }
      const html = `
        <form id="new-invoice-form" class="form-stack">
          <label>Invoice code
            <input type="text" name="code" required placeholder="INV-2026-0099"
                   pattern="INV-[0-9]+-[0-9]+" title="Format: INV-YYYY-NNNN">
          </label>
          <label>Customer
            <select name="customer_id" required>
              ${customers.map(c => `<option value="${c.id}">${escapeHTML(c.code)} — ${escapeHTML(c.name)}</option>`).join('')}
            </select>
          </label>
          <div class="form-row">
            <label>Amount (Rp)
              <input type="number" name="amount_rupiah" min="1" required placeholder="500000">
            </label>
            <label>Due date
              <input type="date" name="due_date" required>
            </label>
          </div>
          <label>Description
            <input type="text" name="description" placeholder="Optional">
          </label>
          <div class="form-actions">
            <button type="button" class="btn-secondary" id="modal-cancel">Cancel</button>
            <button type="submit" class="btn-primary">Create Invoice</button>
          </div>
          <p id="new-invoice-error" class="error" hidden></p>
        </form>`;
      modal.show('New Invoice', html);
      document.getElementById('modal-cancel').onclick = () => modal.close();
      document.getElementById('new-invoice-form').onsubmit = async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const amountRupiah = parseInt(fd.get('amount_rupiah'), 10);
        const body = {
          code: fd.get('code'),
          customer_id: fd.get('customer_id'),
          amount_minor: amountRupiah * 100,
          currency: 'IDR',
          due_date: fd.get('due_date'),
          description: fd.get('description') || undefined,
        };
        const errEl = document.getElementById('new-invoice-error');
        errEl.hidden = true;
        try {
          await api.post('/v1/invoices', body);
          toast(`Invoice ${body.code} created`, 'success');
          modal.close();
          await this.invoices();
        } catch (err) {
          errEl.textContent = `${err.code || 'ERROR'}: ${err.message}`;
          errEl.hidden = false;
        }
      };
    },

    // ----- Aging -----
    _customers: [],
    _currentAgingCustomer: null,
    async aging() {
      const select = document.getElementById('aging-customer-select');
      const tbody = document.querySelector('#aging-table tbody');
      const cards = document.getElementById('aging-summary-cards');

      if (this._customers.length === 0) {
        select.innerHTML = '<option>Loading customers…</option>';
        try {
          const res = await api.get('/v1/accounts');
          this._customers = (res.data || []).filter(a => a.type === 'customer' && a.status === 'active');
          if (this._customers.length === 0) {
            select.innerHTML = '<option>No customers found</option>';
            tbody.innerHTML = `<tr><td colspan="4">${emptyState('👥', 'No customers yet', 'Seed demo data or create customer accounts.')}</td></tr>`;
            cards.innerHTML = '';
            return;
          }
          select.innerHTML = '<option value="">— Select customer —</option>' +
            this._customers.map(c => `<option value="${c.id}">${escapeHTML(c.code)} — ${escapeHTML(c.name)}</option>`).join('');
          select.onchange = () => this.loadAgingFor(select.value);
        } catch (e) {
          select.innerHTML = `<option>Error: ${e.message}</option>`;
        }
      }
      if (this._currentAgingCustomer) {
        await this.loadAgingFor(this._currentAgingCustomer);
      } else {
        tbody.innerHTML = `<tr><td colspan="4">${emptyState('📅', 'Select a customer', 'Choose from the dropdown above to view aging buckets.')}</td></tr>`;
        cards.innerHTML = '';
      }
    },

    async loadAgingFor(customerID) {
      if (!customerID) {
        document.querySelector('#aging-table tbody').innerHTML = `<tr><td colspan="4">${emptyState('📅', 'Select a customer')}</td></tr>`;
        document.getElementById('aging-summary-cards').innerHTML = '';
        return;
      }
      this._currentAgingCustomer = customerID;
      const tbody = document.querySelector('#aging-table tbody');
      const cards = document.getElementById('aging-summary-cards');
      tbody.innerHTML = `<tr><td colspan="4">${spinner()}</td></tr>`;
      cards.innerHTML = spinner();
      try {
        const res = await api.get(`/v1/customers/${customerID}/aging`);
        const data = res.data || [];
        const bucketDefs = [
          { code: 'current',   desc: 'Not yet due' },
          { code: 'd_1_7',     desc: '1-7 days overdue' },
          { code: 'd_8_30',    desc: '8-30 days overdue' },
          { code: 'd_31_60',   desc: '31-60 days overdue' },
          { code: 'd_61_90',   desc: '61-90 days overdue' },
          { code: 'd_90_plus', desc: '90+ days overdue' },
        ];
        const lookup = {};
        data.forEach(d => { lookup[d.bucket] = d; });
        const totalOutstanding = data.reduce((s, d) => s + Number(d.outstanding_minor || 0), 0);
        const totalInvoices = data.reduce((s, d) => s + Number(d.count || 0), 0);

        cards.innerHTML = `
          <div class="stat stat-primary">
            <div class="value">${fmtIDR(totalOutstanding)}</div>
            <div class="label">Total Outstanding</div>
          </div>
          <div class="stat">
            <div class="value">${totalInvoices}</div>
            <div class="label">Open Invoices</div>
          </div>
          <div class="stat ${lookup.d_90_plus && Number(lookup.d_90_plus.outstanding_minor) > 0 ? 'stat-danger' : ''}">
            <div class="value">${lookup.d_90_plus ? fmtIDR(lookup.d_90_plus.outstanding_minor) : '—'}</div>
            <div class="label">90+ days overdue</div>
          </div>`;

        tbody.innerHTML = bucketDefs.map(b => {
          const row = lookup[b.code] || { count: 0, outstanding_minor: 0 };
          const overdueClass = (b.code !== 'current' && Number(row.outstanding_minor) > 0) ? 'class="row-overdue"' : '';
          return `<tr ${overdueClass}>
            <td><strong>${b.code}</strong></td>
            <td>${escapeHTML(b.desc)}</td>
            <td class="num">${row.count}</td>
            <td class="num">${fmtIDR(row.outstanding_minor)}</td>
          </tr>`;
        }).join('');
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="4">${emptyState('❌', 'Error', e.message)}</td></tr>`;
        cards.innerHTML = '';
      }
    },

    // ----- Periods -----
    async periods() {
      const tbody = document.querySelector('#periods-table tbody');
      tbody.innerHTML = `<tr><td colspan="6">${spinner()}</td></tr>`;
      try {
        const res = await api.get('/v1/periods');
        const list = res.data || [];
        if (list.length === 0) {
          tbody.innerHTML = `<tr><td colspan="6">${emptyState('📅', 'No accounting periods', 'Seed demo data or create a period.')}</td></tr>`;
          return;
        }
        tbody.innerHTML = list.map(p => {
          const actions = [];
          if (p.status === 'open') actions.push(`<button class="btn-link" data-action="request-close" data-period-id="${p.id}">Request close</button>`);
          if (p.status === 'closing') actions.push(`<button class="btn-link" data-action="approve-close" data-period-id="${p.id}">Approve</button><button class="btn-link" data-action="reject-close" data-period-id="${p.id}">Reject</button>`);
          return `<tr>
            <td><code>${truncate(p.id, 12)}</code></td>
            <td>${fmtShortDate(p.period_start || p.start_date)}</td>
            <td>${fmtShortDate(p.period_end || p.end_date)}</td>
            <td>${badge(p.status)}</td>
            <td>${fmtAgo(p.created_at)}</td>
            <td>${actions.length ? actions.join(' / ') : '—'}</td>
          </tr>`;
        }).join('');
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="6">${emptyState('❌', 'Error', e.message)}</td></tr>`;
      }
    },

    async periodsAction(action, periodId) {
      try {
        let endpoint, body;
        if (action === 'request-close') { endpoint = `/v1/periods/${periodId}/close-requests`; body = {}; }
        else if (action === 'approve-close') { endpoint = `/v1/periods/close-requests/${periodId}/approve`; body = { approver_id: api.getUser()?.user_id }; }
        else if (action === 'reject-close') { endpoint = `/v1/periods/close-requests/${periodId}/reject`; body = { approver_id: api.getUser()?.user_id, reason: 'manual reject' }; }
        await api.post(endpoint, body);
        toast(`Period ${action} successful`, 'success');
        await this.periods();
      } catch (e) {
        toast(`${e.code || 'Error'}: ${e.message}`, 'error', 6000);
      }
    },

    // ----- Reconciler -----
    async reconciler() {
      const tbody = document.querySelector('#runs-table tbody');
      const periodSelect = document.getElementById('reconciler-period-select');
      tbody.innerHTML = `<tr><td colspan="8">${spinner()}</td></tr>`;
      try {
        const [runsRes, periodsRes] = await Promise.all([
          api.get('/v1/reconciler/runs?limit=20'),
          api.get('/v1/periods'),
        ]);
        const periods = periodsRes.data || [];
        periodSelect.innerHTML = periods.length
          ? periods.map(p => `<option value="${p.id}">${truncate(p.id, 8)}… (${p.status})</option>`).join('')
          : '<option value="">(no periods)</option>';

        const runs = runsRes.data || [];
        if (runs.length === 0) {
          tbody.innerHTML = `<tr><td colspan="8">${emptyState('🔍', 'No reconciliation runs yet', 'Trigger one using the form above.')}</td></tr>`;
          return;
        }
        tbody.innerHTML = runs.map(r => {
          const dur = r.duration_ms ? `${(r.duration_ms / 1000).toFixed(2)}s` : '—';
          const imbalance = Number(r.imbalance_minor || 0);
          const rowClass = imbalance !== 0 ? 'class="row-danger"' : '';
          return `<tr ${rowClass}>
            <td><code>${truncate(r.id, 8)}</code></td>
            <td><code>${truncate(r.period_id, 8)}</code></td>
            <td>${badge(r.status)}</td>
            <td class="num">${fmtIDR(r.total_debit_minor || 0)}</td>
            <td class="num">${fmtIDR(r.total_credit_minor || 0)}</td>
            <td class="num">${fmtIDR(imbalance)}</td>
            <td>${fmtAgo(r.started_at)}</td>
            <td>${dur}</td>
          </tr>`;
        }).join('');
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="8">${emptyState('❌', 'Error', e.message)}</td></tr>`;
      }
    },

    async submitReconciler(form) {
      const fd = new FormData(form);
      const body = {
        tenant_id: api.getUser()?.tenant_id,
        period_id: fd.get('period_id'),
        run_hash_check: fd.get('run_hash_check') === 'on',
      };
      try {
        const res = await api.post('/v1/reconciler/run', body);
        toast(`Reconciliation started: ${truncate(res.data?.run_id || '?', 8)}`, 'success');
        await this.reconciler();
      } catch (e) {
        toast(`${e.code || 'Error'}: ${e.message}`, 'error', 6000);
      }
    },

    // ----- Currencies -----
    _currencies: [],
    async currencies() {
      const tbody = document.querySelector('#currencies-table tbody');
      const fromSel = document.getElementById('convert-from');
      const toSel = document.getElementById('convert-to');
      tbody.innerHTML = `<tr><td colspan="4">${spinner()}</td></tr>`;
      try {
        const res = await api.get('/v1/currencies');
        this._currencies = res.data || [];
        if (this._currencies.length === 0) {
          tbody.innerHTML = `<tr><td colspan="4">${emptyState('💱', 'No currencies', 'Seed demo data first.')}</td></tr>`;
          return;
        }
        tbody.innerHTML = this._currencies.map(c => `
          <tr>
            <td><code>${escapeHTML(c.code)}</code></td>
            <td>${escapeHTML(c.name)}</td>
            <td class="num">${c.decimal_places ?? '—'}</td>
            <td>${c.is_active ? '✅' : '❌'}</td>
          </tr>`).join('');
        const opts = this._currencies.map(c => `<option value="${escapeHTML(c.code)}">${escapeHTML(c.code)} — ${escapeHTML(c.name)}</option>`).join('');
        fromSel.innerHTML = opts;
        toSel.innerHTML = opts;
        if (!fromSel.value) fromSel.value = 'USD';
        if (!toSel.value)   toSel.value = 'IDR';
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="4">${emptyState('❌', 'Error', e.message)}</td></tr>`;
      }
    },

    async submitConvert(form) {
      const fd = new FormData(form);
      const body = {
        tenant_id: api.getUser()?.tenant_id,
        from_currency: fd.get('from_currency'),
        to_currency: fd.get('to_currency'),
        amount_minor: parseInt(fd.get('amount_minor'), 10),
      };
      const out = document.getElementById('convert-result');
      out.innerHTML = spinner('Converting…');
      try {
        const res = await api.post('/v1/currencies/convert', body);
        const r = res.data || res;
        out.innerHTML = `
          <div class="result-row"><span>From:</span><strong>${body.amount_minor.toLocaleString()} ${escapeHTML(body.from_currency)}</strong></div>
          <div class="result-row"><span>To:</span><strong>${r.converted_amount_minor?.toLocaleString() || '?'} ${escapeHTML(body.to_currency)}</strong></div>
          <div class="result-row"><span>Rate:</span><strong>${r.rate || '?'}</strong></div>
          <div class="result-row"><span>FX Rate ID:</span><code>${truncate(r.fx_rate_id, 12)}</code></div>`;
      } catch (e) {
        out.innerHTML = `<p class="error">${escapeHTML(e.message)}</p>`;
      }
    },

    // ----- Audit -----
    async audit() {
      const tbody = document.querySelector('#audit-table tbody');
      const resourceFilter = document.getElementById('filter-audit-resource').value;
      const searchFilter = document.getElementById('filter-audit-search').value.toLowerCase();
      tbody.innerHTML = `<tr><td colspan="6">${spinner()}</td></tr>`;
      try {
        const qs = resourceFilter ? `?resource_type=${resourceFilter}&limit=100` : '?limit=100';
        const res = await api.get(`/v1/audit${qs}`);
        let list = res.data || [];
        if (searchFilter) list = list.filter(a =>
          (a.action || '').toLowerCase().includes(searchFilter) ||
          (a.actor_id || '').toLowerCase().includes(searchFilter)
        );
        if (list.length === 0) {
          tbody.innerHTML = `<tr><td colspan="6">${emptyState('📋', 'No audit entries', 'Audit log fills up as you use the app.')}</td></tr>`;
          return;
        }
        tbody.innerHTML = list.slice(0, 100).map(a => `
          <tr>
            <td title="${escapeHTML(a.occurred_at || a.created_at)}">${fmtAgo(a.occurred_at || a.created_at)}</td>
            <td><code>${truncate(a.actor_id, 8)}</code></td>
            <td>${escapeHTML(a.action || '—')}</td>
            <td>${escapeHTML(a.resource_type || '')} <code>${truncate(a.resource_id, 8)}</code></td>
            <td>${a.status_code || ''}</td>
            <td>${escapeHTML(a.ip_address || '—')}</td>
          </tr>`).join('');
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="6">${emptyState('❌', 'Error', e.message)}</td></tr>`;
      }
    },
  };

  // ===========================================================================
  // 4) App orchestration
  // ===========================================================================
  const app = {
    async init() {
      // Password visibility toggle
      document.getElementById('toggle-pw').addEventListener('click', () => {
        const pw = document.getElementById('password');
        pw.type = pw.type === 'password' ? 'text' : 'password';
      });

      // API status indicator
      const updateStatus = async () => {
        const el = document.getElementById('api-status');
        if (el) el.textContent = await api.ping();
      };
      await updateStatus();
      setInterval(updateStatus, 30_000);

      // Check existing session
      const token = api.getToken();
      const user = api.getUser();
      if (token && user && user.tenant_id) {
        auth.showApp(user);
        this.showView('dashboard');
      } else {
        auth.showLogin();
      }

      // Login form
      document.getElementById('login-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const tenant_id = fd.get('tenant_id').trim();
        const username = fd.get('username').trim();
        const password = fd.get('password');
        const errEl = document.getElementById('login-error');
        const submitBtn = document.getElementById('login-submit');
        errEl.hidden = true;
        submitBtn.disabled = true;
        submitBtn.textContent = 'Signing in…';
        try {
          await api.login(tenant_id, username, password);
          auth.showApp(api.getUser());
          this.showView('dashboard');
          toast(`Welcome, ${username.slice(0, 8)}…`, 'success');
        } catch (err) {
          errEl.textContent = `${err.code || 'ERROR'}: ${err.message}`;
          errEl.hidden = false;
        } finally {
          submitBtn.disabled = false;
          submitBtn.textContent = 'Sign in';
        }
      });

      // Logout
      document.getElementById('logout-btn').addEventListener('click', async () => {
        await api.logout();
        auth.showLogin();
        toast('Logged out', 'info');
      });

      // Transfer form
      document.getElementById('transfer-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        await views.submitTransfer(e.target);
      });

      // Reconciler form
      document.getElementById('reconciler-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        await views.submitReconciler(e.target);
      });

      // Convert form
      document.getElementById('convert-form').addEventListener('submit', async (e) => {
        e.preventDefault();
        await views.submitConvert(e.target);
      });

      // New Account button
      document.getElementById('new-account-btn').addEventListener('click', () => showNewAccountModal());

      // New Invoice button
      document.getElementById('new-invoice-btn').addEventListener('click', () => views.showNewInvoiceModal());

      // Account filters
      document.getElementById('filter-account-type').addEventListener('change', () => views.accounts());
      document.getElementById('filter-account-search').addEventListener('input', debounce(() => views.accounts(), 300));

      // Invoice filter
      document.getElementById('filter-invoice-status').addEventListener('change', () => views.invoices());

      // Aging refresh
      document.getElementById('aging-refresh-btn').addEventListener('click', () => {
        const sel = document.getElementById('aging-customer-select');
        if (sel.value) views.loadAgingFor(sel.value);
      });

      // Audit filters
      document.getElementById('filter-audit-resource').addEventListener('change', () => views.audit());
      document.getElementById('filter-audit-search').addEventListener('input', debounce(() => views.audit(), 300));

      // Periods action delegation
      document.getElementById('periods-table').addEventListener('click', (e) => {
        const btn = e.target.closest('button[data-action]');
        if (!btn) return;
        views.periodsAction(btn.dataset.action, btn.dataset.periodId);
      });

      // Hash-based routing
      window.addEventListener('hashchange', () => {
        const view = window.location.hash.slice(1);
        if (view && document.getElementById('view-' + view)) this.showView(view);
      });
    },

    async showView(name) {
      const viewNames = ['dashboard','accounts','transfers','invoices','aging','periods','reconciler','currencies','audit'];
      viewNames.forEach(v => {
        const el = document.getElementById('view-' + v);
        if (el) el.hidden = (v !== name);
      });
      document.querySelectorAll('#nav a').forEach(a => {
        a.classList.toggle('active', a.dataset.view === name);
      });
      window.location.hash = name;
      const loader = views[name];
      if (loader) try { await loader.call(views); } catch (e) { console.error(`view ${name}`, e); toast(`View error: ${e.message}`, 'error'); }
    },
  };

  // ----- Helpers -----
  const fmtShortDate = (iso) => {
    if (!iso) return '—';
    const d = new Date(iso);
    if (isNaN(d)) return iso;
    return d.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
  };
  const debounce = (fn, ms) => {
    let t;
    return (...args) => { clearTimeout(t); t = setTimeout(() => fn(...args), ms); };
  };

  // ----- New Account modal -----
  const showNewAccountModal = async () => {
    const html = `
      <form id="new-account-form" class="form-stack">
        <label>Account code
          <input type="text" name="code" required placeholder="ACC-001"
                 pattern="[A-Z0-9-]{3,30}" title="Uppercase letters, digits, dashes">
        </label>
        <label>Name
          <input type="text" name="name" required placeholder="My Account">
        </label>
        <div class="form-row">
          <label>Type
            <select name="type" required>
              <option value="cash">Cash</option>
              <option value="receivable">Receivable</option>
              <option value="payable">Payable</option>
              <option value="revenue">Revenue</option>
              <option value="hq">HQ</option>
              <option value="customer">Customer</option>
              <option value="sales_rep">Sales Rep</option>
              <option value="outlet">Outlet</option>
              <option value="suspense">Suspense</option>
            </select>
          </label>
          <label>Currency
            <input type="text" name="currency" value="IDR" required maxlength="3">
          </label>
        </div>
        <label>Starting balance (Rp)
          <input type="number" name="balance_rupiah" value="0" min="0">
        </label>
        <div class="form-actions">
          <button type="button" class="btn-secondary" id="modal-cancel">Cancel</button>
          <button type="submit" class="btn-primary">Create</button>
        </div>
        <p id="new-account-error" class="error" hidden></p>
      </form>`;
    modal.show('New Account', html);
    document.getElementById('modal-cancel').onclick = () => modal.close();
    document.getElementById('new-account-form').onsubmit = async (e) => {
      e.preventDefault();
      const fd = new FormData(e.target);
      const balanceRupiah = parseInt(fd.get('balance_rupiah') || '0', 10);
      const body = {
        code: fd.get('code'),
        name: fd.get('name'),
        type: fd.get('type'),
        currency: fd.get('currency'),
        cached_balance_minor: balanceRupiah * 100,
      };
      const errEl = document.getElementById('new-account-error');
      errEl.hidden = true;
      try {
        await api.post('/v1/accounts', body);
        toast(`Account ${body.code} created`, 'success');
        modal.close();
        await views.accounts();
      } catch (err) {
        errEl.textContent = `${err.code || 'ERROR'}: ${err.message}`;
        errEl.hidden = false;
      }
    };
  };

  // Boot
  document.addEventListener('DOMContentLoaded', () => app.init());
})();
