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
    const n = Number(minor);
    if (isNaN(n)) return '—';
    const m = BigInt(Math.round(n));
    const negative = m < 0n;
    const abs = negative ? -m : m;
    const code = (currency || 'IDR').toUpperCase();
    const factor = 100n; // all currencies in this system (IDR, USD, SGD) use 2 decimal places in minor units (cents/sen)
    const major = abs / factor;
    const minor2 = abs % factor;
    const majorStr = major.toString().replace(/\B(?=(\d{3})+(?!\d))/g, '.');
    const minorStr = code === 'IDR'
      ? (minor2 === 0n ? '' : ',' + minor2.toString().padStart(2, '0'))
      : ',' + minor2.toString().padStart(2, '0');
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

  const fmtShortDate = (iso) => {
    if (!iso) return '—';
    const d = new Date(iso);
    if (isNaN(d)) return iso;
    return d.toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
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
    const s = String(status).toLowerCase();
    return `<span class="badge badge-${s} ${s}">${escapeHTML(status)}</span>`;
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

  const getUserMeta = (user) => {
    if (!user) return { name: 'Guest', role: 'guest', roleDisplay: 'Guest', initials: 'G', tenantName: 'Default Tenant', userId: '', tenantId: '' };

    const uid = String(user.user_id || user.username || '');
    const tid = String(user.tenant_id || '');
    let name = user.display_name || user.name;
    let role = user.role || 'user';
    let initials = 'U';

    if (uid === '33333333-3333-3333-3333-333333333333' || user.username === '33333333-3333-3333-3333-333333333333' || user.username === 'admin') {
      name = name || 'Admin HQ';
      role = (role === 'unknown' || !role) ? 'hq_admin' : role;
      initials = 'HQ';
    } else if (uid === '44444444-4444-4444-4444-444444444444' || user.username === '44444444-4444-4444-4444-444444444444' || user.username === 'sales') {
      name = name || 'Sales Joni';
      role = (role === 'unknown' || !role) ? 'sales_rep' : role;
      initials = 'SJ';
    } else if (user.username && !user.username.includes('-')) {
      name = user.username.charAt(0).toUpperCase() + user.username.slice(1);
      initials = user.username.slice(0, 2).toUpperCase();
    } else {
      name = name || `User (${truncate(uid, 8)})`;
      initials = uid.slice(0, 2).toUpperCase();
    }

    const roleLabels = {
      'hq_admin': 'HQ Admin',
      'hq_finance': 'HQ Finance',
      'auditor': 'Auditor',
      'sales_rep': 'Sales Rep',
      'outlet_manager': 'Outlet Mgr',
      'admin': 'Admin',
    };
    const roleDisplay = roleLabels[role] || (role ? role.replace(/_/g, ' ').toUpperCase() : 'User');

    let tenantName = 'FMCG Tenant';
    if (tid.startsWith('00000000-0000-0000-0000') || tid === 'hq' || tid === '00000000-0000-0000-0000-000000000001') {
      tenantName = 'Global FMCG HQ';
    } else if (tid) {
      tenantName = `Tenant ${truncate(tid, 8)}`;
    }

    return {
      name,
      role,
      roleDisplay,
      initials,
      tenantName,
      userId: uid,
      tenantId: tid,
    };
  };

  // Toast notifications
  const toast = (msg, type = 'info', duration = 4000) => {
    const container = document.getElementById('toast-container');
    if (!container) return;
    const el = document.createElement('div');
    el.className = `toast toast-${type}`;
    el.innerHTML = `<span class="toast-icon">${
      type === 'success' ? '✓' :
      type === 'error' ? '✕' :
      type === 'warning' ? '!' : 'ℹ'
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
  // Paginator — Reusable table pagination controller (client-side & server-side)
  // ===========================================================================
  class Paginator {
    constructor({ containerId, defaultPageSize = 10, pageSizes = [10, 25, 50, 100], onChange }) {
      this.containerId = containerId;
      this.pageSize = defaultPageSize;
      this.pageSizes = pageSizes;
      this.page = 1;
      this.total = 0;
      this.onChange = onChange;
    }

    get container() {
      return document.getElementById(this.containerId);
    }

    reset() {
      this.page = 1;
    }

    update(totalItems) {
      this.total = totalItems;
      const totalPages = Math.max(1, Math.ceil(this.total / this.pageSize));
      if (this.page > totalPages) {
        this.page = totalPages;
      }
      this.render();
    }

    getCurrentSlice(items) {
      if (!items || !items.length) return [];
      const start = (this.page - 1) * this.pageSize;
      return items.slice(start, start + this.pageSize);
    }

    render() {
      const el = this.container;
      if (!el) return;
      if (this.total === 0) {
        el.innerHTML = '';
        el.hidden = true;
        return;
      }
      el.hidden = false;

      const totalPages = Math.max(1, Math.ceil(this.total / this.pageSize));
      const startItem = (this.page - 1) * this.pageSize + 1;
      const endItem = Math.min(this.page * this.pageSize, this.total);

      // Smart page numbers array
      const pages = [];
      if (totalPages <= 7) {
        for (let p = 1; p <= totalPages; p++) pages.push(p);
      } else {
        pages.push(1);
        if (this.page > 3) pages.push('...');
        const pStart = Math.max(2, this.page - 1);
        const pEnd = Math.min(totalPages - 1, this.page + 1);
        for (let p = pStart; p <= pEnd; p++) {
          if (!pages.includes(p)) pages.push(p);
        }
        if (this.page < totalPages - 2) pages.push('...');
        if (!pages.includes(totalPages)) pages.push(totalPages);
      }

      el.innerHTML = `
        <div class="pagination-left">
          <div class="pagination-info">
            Menampilkan <span class="highlight">${startItem}–${endItem}</span> dari <span class="highlight">${this.total}</span> data
          </div>
          <div class="pagination-size">
            <span>Baris per halaman:</span>
            <select class="page-size-select" aria-label="Jumlah baris per halaman">
              ${this.pageSizes.map(s => `<option value="${s}" ${s === this.pageSize ? 'selected' : ''}>${s}</option>`).join('')}
            </select>
          </div>
        </div>
        <div class="pagination-nav">
          <button type="button" class="pagination-btn btn-prev" ${this.page <= 1 ? 'disabled' : ''} title="Halaman Sebelumnya">« Sebelumnya</button>
          ${pages.map(p => {
            if (p === '...') return '<span class="pagination-ellipsis">…</span>';
            return `<button type="button" class="pagination-btn btn-page ${p === this.page ? 'active' : ''}" data-page="${p}">${p}</button>`;
          }).join('')}
          <button type="button" class="pagination-btn btn-next" ${this.page >= totalPages ? 'disabled' : ''} title="Halaman Berikutnya">Berikutnya »</button>
        </div>
      `;

      // Event handlers
      const sizeSelect = el.querySelector('.page-size-select');
      sizeSelect?.addEventListener('change', (e) => {
        this.pageSize = parseInt(e.target.value, 10);
        this.page = 1;
        if (this.onChange) this.onChange();
      });

      const prevBtn = el.querySelector('.btn-prev');
      prevBtn?.addEventListener('click', () => {
        if (this.page > 1) {
          this.page--;
          if (this.onChange) this.onChange();
        }
      });

      const nextBtn = el.querySelector('.btn-next');
      nextBtn?.addEventListener('click', () => {
        if (this.page < totalPages) {
          this.page++;
          if (this.onChange) this.onChange();
        }
      });

      el.querySelectorAll('.btn-page').forEach(btn => {
        btn.addEventListener('click', () => {
          const targetPage = parseInt(btn.dataset.page, 10);
          if (targetPage !== this.page) {
            this.page = targetPage;
            if (this.onChange) this.onChange();
          }
        });
      });
    }
  }

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
      const subpath = location.pathname.startsWith('/core') ? '/core' : '';
      const url = subpath + path;
      const res = await fetch(url, fetchOpts);
      const ct = res.headers.get('content-type') || '';
      const body = ct.includes('json') ? await res.json() : await res.text();
      if (!res.ok) {
        if (res.status === 401 && !opts._retry && !opts.skipAuth && path !== '/v1/auth/login' && path !== '/v1/auth/refresh') {
          const refreshToken = this.getRefresh();
          if (refreshToken) {
            try {
              const refRes = await this.post('/v1/auth/refresh', { refresh_token: refreshToken }, { skipAuth: true, skipTenant: true, _retry: true });
              if (refRes.data && refRes.data.access_token) {
                this.setToken(refRes.data.access_token);
                this.setRefresh(refRes.data.refresh_token);
                return this.request(path, { ...opts, _retry: true });
              }
            } catch (refErr) {
              this.setToken(null);
              this.setRefresh(null);
              this.setUser(null);
              auth.showLogin();
              toast('Sesi telah kedaluwarsa. Silakan sign in kembali.', 'warning');
            }
          } else {
            this.setToken(null);
            this.setRefresh(null);
            this.setUser(null);
            auth.showLogin();
            toast('Sesi telah kedaluwarsa. Silakan sign in kembali.', 'warning');
          }
        }
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
        const subpath = location.pathname.startsWith('/core') ? '/core' : '';
        const res = await fetch(subpath + '/healthz');
        return res.ok ? 'Connected' : 'Disconnected';
      } catch (e) { return 'Unreachable'; }
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
      document.getElementById('user-dropdown-card')?.setAttribute('hidden', '');
      document.getElementById('user-profile-btn')?.classList.remove('active');
      document.getElementById('user-profile-btn')?.setAttribute('aria-expanded', 'false');
      ['view-dashboard','view-accounts','view-transfers','view-invoices','view-aging',
       'view-periods','view-reconciler','view-currencies','view-audit'].forEach(id => {
        const el = document.getElementById(id); if (el) el.hidden = true;
      });
      document.getElementById('user-status').textContent = '';
    },
    showApp(user) {
      document.getElementById('topbar').hidden = false;
      document.getElementById('view-login').hidden = true;

      const meta = getUserMeta(user);

      // Topbar capsule elements
      const avatarEl = document.getElementById('user-avatar');
      if (avatarEl) {
        avatarEl.innerHTML = `<span class="avatar-text">${meta.initials}</span><span class="avatar-status-dot" title="Active Session"></span>`;
      }
      const displayNameEl = document.getElementById('user-display-name');
      if (displayNameEl) displayNameEl.textContent = meta.name;
      const rolePillEl = document.getElementById('user-role-pill');
      if (rolePillEl) rolePillEl.textContent = meta.roleDisplay;
      const tenantLabelEl = document.getElementById('user-tenant-label');
      if (tenantLabelEl) tenantLabelEl.textContent = meta.tenantName;

      // Dropdown card elements
      const ddAvatar = document.getElementById('dropdown-avatar-lg');
      if (ddAvatar) ddAvatar.textContent = meta.initials;
      const ddFullName = document.getElementById('dropdown-full-name');
      if (ddFullName) ddFullName.textContent = meta.name;
      const ddRoleBadge = document.getElementById('dropdown-role-badge');
      if (ddRoleBadge) ddRoleBadge.textContent = meta.roleDisplay;
      const ddUserId = document.getElementById('dropdown-user-id');
      if (ddUserId) ddUserId.textContent = meta.userId || '—';
      const ddTenantId = document.getElementById('dropdown-tenant-id');
      if (ddTenantId) ddTenantId.textContent = meta.tenantId || '—';

      // Status bar tenant text
      const userStatusEl = document.getElementById('user-status');
      if (userStatusEl) {
        userStatusEl.textContent = `${meta.name} (${meta.roleDisplay}) | ${meta.tenantName}`;
      }

      this.buildNav();
    },
    buildNav() {
      const nav = [
        ['dashboard', 'Dashboard'],
        ['accounts', 'Accounts'],
        ['transfers', 'Transfers'],
        ['invoices', 'Invoices'],
        ['aging', 'Aging'],
        ['periods', 'Periods'],
        ['reconciler', 'Reconciler'],
        ['currencies', 'Currencies'],
        ['audit', 'Audit'],
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
          api.get('/v1/accounts').catch(err => { console.warn('accounts fetch failed:', err); return { data: [] }; }),
          api.get('/v1/invoices?status=open&limit=10').catch(err => { console.warn('open invoices fetch failed:', err); return { data: [] }; }),
          api.get('/v1/invoices?status=partial&limit=10').catch(err => { console.warn('partial invoices fetch failed:', err); return { data: [] }; }),
          api.get('/v1/periods').catch(err => { console.warn('periods fetch failed:', err); return { data: [] }; }),
        ]);

        const acctList = accounts.data || [];
        const totalBal = acctList.reduce((s, a) => s + Number(a.cached_balance_minor || a.balance_minor || 0), 0);
        const cashAccounts = acctList.filter(a => a.type === 'cash');
        const cashBal = cashAccounts.reduce((s, a) => s + Number(a.cached_balance_minor || a.balance_minor || 0), 0);

        const openList = openInv.data || [];
        const partialList = partialInv.data || [];
        const openTotal = openList.reduce((s, i) => {
          const out = i.outstanding_minor ?? (Number(i.amount_minor || 0) - Number(i.paid_minor || i.paid_amount_minor || 0));
          return s + Number(out || 0);
        }, 0);
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
              const out = inv.outstanding_minor ?? (Number(inv.amount_minor || 0) - Number(inv.paid_minor || inv.paid_amount_minor || 0));
              return `<tr>
                <td><code>${escapeHTML(inv.code)}</code></td>
                <td class="num">${fmtIDR(inv.amount_minor)}</td>
                <td class="num">${fmtIDR(out)}</td>
                <td>${fmtShortDate(inv.due_date)}</td>
              </tr>`;
            }).join('')}</tbody>
          </table>` : emptyState('📭', 'No open invoices', 'All invoices have been paid or partially paid.');

        try {
          const agingRes = await api.get('/v1/accounts?type=customer');
          const customers = agingRes.data || [];
          if (customers.length > 0) {
            const firstCust = customers[0];
            const custAging = await api.get(`/v1/customers/${firstCust.id}/aging`);
            const buckets = custAging.data || [];
            ag.innerHTML = `
              <div style="font-size:0.85rem;margin-bottom:8px;color:var(--text-muted);">Customer: <strong style="color:var(--text);">${escapeHTML(firstCust.name)}</strong></div>
              <table class="data-table compact">
                <thead><tr><th>Bucket</th><th class="num">Count</th><th class="num">Outstanding</th></tr></thead>
                <tbody>${buckets.map(b => `
                  <tr>
                    <td><code>${escapeHTML(b.bucket)}</code></td>
                    <td class="num">${b.count}</td>
                    <td class="num">${fmtIDR(b.outstanding_minor)}</td>
                  </tr>`).join('')}</tbody>
              </table>`;
          } else {
            ag.innerHTML = emptyState('📅', 'No aging data', 'No customer accounts found.');
          }
        } catch {
          ag.innerHTML = emptyState('📅', 'Aging data', 'Select Aging tab to view per-customer breakdown.');
        }
      } catch (e) {
        cards.innerHTML = `<p class="error">Failed: ${escapeHTML(e.message)}</p>`;
        oi.innerHTML = '';
        ag.innerHTML = '';
      }
    },

    // ----- Accounts -----
    _allAccounts: [],
    _filteredAccounts: [],
    _accountsPaginator: null,

    initAccountsPaginator() {
      if (!this._accountsPaginator) {
        this._accountsPaginator = new Paginator({
          containerId: 'accounts-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderAccountsTable(),
        });
      }
    },

    filterAccounts() {
      const typeFilter = document.getElementById('filter-account-type')?.value || '';
      const searchFilter = (document.getElementById('filter-account-search')?.value || '').toLowerCase().trim();
      let list = this._allAccounts || [];
      if (typeFilter) list = list.filter(a => a.type === typeFilter);
      if (searchFilter) list = list.filter(a =>
        (a.code || '').toLowerCase().includes(searchFilter) ||
        (a.name || '').toLowerCase().includes(searchFilter)
      );
      this._filteredAccounts = list;
      this.initAccountsPaginator();
      this._accountsPaginator.reset();
      this._accountsPaginator.update(this._filteredAccounts.length);
      this.renderAccountsTable();
    },

    renderAccountsTable() {
      const tbody = document.querySelector('#accounts-table tbody');
      if (!tbody) return;
      if (!this._filteredAccounts || this._filteredAccounts.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('🏦', 'No accounts found', 'Try changing the filters or click "+ New Account".')}</td></tr>`;
        return;
      }
      const slice = this._accountsPaginator.getCurrentSlice(this._filteredAccounts);
      tbody.innerHTML = slice.map(a => `
        <tr>
          <td><code>${escapeHTML(a.code)}</code></td>
          <td>${escapeHTML(a.name)}</td>
          <td>${badge(a.type)}</td>
          <td>${escapeHTML(a.currency)}</td>
          <td class="num">${fmtMinor(a.cached_balance_minor || a.balance_minor, a.currency)}</td>
          <td>${badge(a.status)}</td>
          <td>${fmtAgo(a.created_at)}</td>
        </tr>`).join('');
    },

    async accounts() {
      const tbody = document.querySelector('#accounts-table tbody');
      tbody.innerHTML = '<tr><td colspan="7">' + spinner('Loading accounts…') + '</td></tr>';
      try {
        const res = await api.get('/v1/accounts');
        this._allAccounts = res.data || [];
        this.filterAccounts();
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('❌', 'Error loading accounts', e.message)}</td></tr>`;
      }
    },

    // ----- Transfers -----
    _allTransfers: [],
    _filteredTransfers: [],
    _transfersPaginator: null,

    initTransfersPaginator() {
      if (!this._transfersPaginator) {
        this._transfersPaginator = new Paginator({
          containerId: 'transfers-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderTransfersTable(),
        });
      }
    },

    filterTransfers() {
      const q = (document.getElementById('filter-transfer-search')?.value || '').toLowerCase().trim();
      let list = this._allTransfers || [];
      if (q) {
        list = list.filter(t =>
          (t.id || '').toLowerCase().includes(q) ||
          (t.from?.code || '').toLowerCase().includes(q) ||
          (t.from?.name || '').toLowerCase().includes(q) ||
          (t.to?.code || '').toLowerCase().includes(q) ||
          (t.to?.name || '').toLowerCase().includes(q) ||
          (t.description || '').toLowerCase().includes(q) ||
          (t.currency || '').toLowerCase().includes(q) ||
          (t.status || '').toLowerCase().includes(q)
        );
      }
      this._filteredTransfers = list;
      this.initTransfersPaginator();
      this._transfersPaginator.reset();
      this._transfersPaginator.update(this._filteredTransfers.length);
      this.renderTransfersTable();
    },

    renderTransfersTable() {
      const tbody = document.querySelector('#transfers-table tbody');
      if (!tbody) return;
      if (!this._filteredTransfers || this._filteredTransfers.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('💸', 'Tidak ada transaksi yang cocok', 'Coba ubah kata kunci pencarian atau buat transfer baru.')}</td></tr>`;
        return;
      }
      const slice = this._transfersPaginator.getCurrentSlice(this._filteredTransfers);
      tbody.innerHTML = slice.map(t => `
        <tr>
          <td><code>${escapeHTML(truncate(t.id, 8))}</code></td>
          <td>${escapeHTML(t.from ? `${t.from.code} — ${t.from.name}` : '-')}</td>
          <td>${escapeHTML(t.to ? `${t.to.code} — ${t.to.name}` : '-')}</td>
          <td class="num">${fmtIDR(t.amount_minor)}</td>
          <td>${escapeHTML(t.currency)}</td>
          <td>${badge(t.status)}</td>
          <td>${fmtAgo(t.created_at)}</td>
        </tr>`).join('');
    },

    async transfers() {
      await this.loadAccountOptions();
      document.getElementById('transfer-success').hidden = true;
      document.getElementById('transfer-error').hidden = true;
      await this.loadTransferHistory();
    },

    async loadTransferHistory() {
      const tbody = document.querySelector('#transfers-table tbody');
      tbody.innerHTML = `<tr><td colspan="7">${spinner()}</td></tr>`;
      try {
        const accRes = await api.get('/v1/accounts');
        const accounts = accRes.data || [];
        const accMap = new Map(accounts.map(a => [a.id, a]));
        const cashAccounts = accounts.filter(a => a.type === 'cash');

        const allEntries = [];
        for (const acc of cashAccounts) {
          try {
            const entRes = await api.get(`/v1/accounts/${acc.id}/entries?limit=50`);
            if (entRes.data && Array.isArray(entRes.data)) {
              allEntries.push(...entRes.data);
            }
          } catch (e) { /* continue */ }
        }

        const txMap = new Map();
        for (const e of allEntries) {
          if (!txMap.has(e.transaction_id)) {
            txMap.set(e.transaction_id, {
              id: e.transaction_id,
              created_at: e.created_at,
              amount_minor: e.amount_minor,
              status: 'posted',
              currency: accMap.get(e.account_id)?.currency || 'IDR',
              from: null,
              to: null,
              description: e.description,
            });
          }
          const item = txMap.get(e.transaction_id);
          if (e.type === 'debit') {
            item.from = accMap.get(e.account_id);
          } else if (e.type === 'credit') {
            item.to = accMap.get(e.account_id);
          }
        }

        this._allTransfers = Array.from(txMap.values())
          .sort((a, b) => new Date(b.created_at) - new Date(a.created_at));

        this.filterTransfers();
      } catch (err) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('❌', 'Error loading transfers', err.message)}</td></tr>`;
      }
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
        await this.loadTransferHistory();
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
    _allInvoices: [],
    _filteredInvoices: [],
    _invoicesPaginator: null,
    _invoicesAcctMap: {},

    initInvoicesPaginator() {
      if (!this._invoicesPaginator) {
        this._invoicesPaginator = new Paginator({
          containerId: 'invoices-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderInvoicesTable(),
        });
      }
    },

    filterInvoices() {
      const status = document.getElementById('filter-invoice-status')?.value || '';
      const q = (document.getElementById('filter-invoice-search')?.value || '').toLowerCase().trim();
      let list = this._allInvoices || [];
      if (status) {
        list = list.filter(inv => inv.status === status);
      }
      if (q) {
        list = list.filter(inv => {
          const cust = this._invoicesAcctMap[inv.customer_id];
          return (inv.code || '').toLowerCase().includes(q) ||
                 (inv.id || '').toLowerCase().includes(q) ||
                 (inv.customer_id || '').toLowerCase().includes(q) ||
                 (cust?.name || '').toLowerCase().includes(q) ||
                 (cust?.code || '').toLowerCase().includes(q) ||
                 (inv.status || '').toLowerCase().includes(q) ||
                 (inv.description || '').toLowerCase().includes(q);
        });
      }
      this._filteredInvoices = list;
      this.initInvoicesPaginator();
      this._invoicesPaginator.reset();
      this._invoicesPaginator.update(this._filteredInvoices.length);
      this.renderInvoicesTable();
    },

    renderInvoicesTable() {
      const tbody = document.querySelector('#invoices-table tbody');
      if (!tbody) return;
      if (!this._filteredInvoices || this._filteredInvoices.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('📄', 'No invoices found', 'Try changing the filters or click "+ New Invoice".')}</td></tr>`;
        return;
      }
      const slice = this._invoicesPaginator.getCurrentSlice(this._filteredInvoices);
      tbody.innerHTML = slice.map(inv => {
        const cust = this._invoicesAcctMap[inv.customer_id];
        const paid = Number(inv.paid_minor ?? inv.paid_amount_minor ?? 0);
        const outstanding = Number(inv.outstanding_minor ?? (Number(inv.amount_minor || 0) - paid));
        return `<tr>
          <td><code>${escapeHTML(inv.code)}</code></td>
          <td>${cust ? escapeHTML(cust.name) : '<span class="muted">' + truncate(inv.customer_id, 12) + '</span>'}</td>
          <td class="num">${fmtIDR(inv.amount_minor)}</td>
          <td class="num">${fmtIDR(paid)}</td>
          <td class="num">${fmtIDR(outstanding)}</td>
          <td>${badge(inv.status)}</td>
          <td>${fmtShortDate(inv.due_date)}</td>
        </tr>`;
      }).join('');
    },

    async invoices() {
      const tbody = document.querySelector('#invoices-table tbody');
      tbody.innerHTML = `<tr><td colspan="7">${spinner()}</td></tr>`;
      try {
        const [invRes, acctRes] = await Promise.all([
          api.get('/v1/invoices?limit=200'),
          api.get('/v1/accounts'),
        ]);
        this._allInvoices = invRes.data || [];
        this._invoicesAcctMap = {};
        (acctRes.data || []).forEach(a => { this._invoicesAcctMap[a.id] = a; });
        this.filterInvoices();
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
    _agingInvoices: [],
    _filteredAgingInvoices: [],
    _agingInvoicesPaginator: null,

    initAgingInvoicesPaginator() {
      if (!this._agingInvoicesPaginator) {
        this._agingInvoicesPaginator = new Paginator({
          containerId: 'aging-invoices-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderAgingInvoicesTable(),
        });
      }
    },

    filterAgingInvoices() {
      const status = document.getElementById('filter-aging-status')?.value || '';
      const q = (document.getElementById('filter-aging-search')?.value || '').toLowerCase().trim();
      let list = this._agingInvoices || [];
      if (status) {
        list = list.filter(inv => inv.status === status);
      }
      if (q) {
        list = list.filter(inv =>
          (inv.code || '').toLowerCase().includes(q) ||
          (inv.id || '').toLowerCase().includes(q) ||
          (inv.status || '').toLowerCase().includes(q) ||
          (inv.due_date || '').toLowerCase().includes(q)
        );
      }
      this._filteredAgingInvoices = list;
      this.initAgingInvoicesPaginator();
      this._agingInvoicesPaginator.reset();
      this._agingInvoicesPaginator.update(this._filteredAgingInvoices.length);
      this.renderAgingInvoicesTable();
    },

    renderAgingInvoicesTable() {
      const tbody = document.querySelector('#aging-invoices-table tbody');
      if (!tbody) return;
      if (!this._filteredAgingInvoices || this._filteredAgingInvoices.length === 0) {
        tbody.innerHTML = `<tr><td colspan="7">${emptyState('📄', 'Tidak ada data faktur', 'Tidak ditemukan faktur yang cocok dengan pencarian.')}</td></tr>`;
        return;
      }
      const slice = this._agingInvoicesPaginator.getCurrentSlice(this._filteredAgingInvoices);
      tbody.innerHTML = slice.map(inv => {
        const paid = Number(inv.paid_minor ?? inv.paid_amount_minor ?? 0);
        const amount = Number(inv.amount_minor || 0);
        const outstanding = Number(inv.outstanding_minor ?? (amount - paid));

        const dueDate = new Date(inv.due_date);
        const now = new Date();
        const diffDays = Math.floor((now - dueDate) / (1000 * 60 * 60 * 24));
        let bucketLabel = '<span class="badge badge-active">Lancar (Current)</span>';
        if (diffDays > 90) bucketLabel = '<span class="badge badge-tampered">&gt; 90 Hari</span>';
        else if (diffDays > 60) bucketLabel = '<span class="badge badge-danger">61–90 Hari</span>';
        else if (diffDays > 30) bucketLabel = '<span class="badge badge-warn">31–60 Hari</span>';
        else if (diffDays > 7) bucketLabel = '<span class="badge badge-warn">8–30 Hari</span>';
        else if (diffDays > 0) bucketLabel = '<span class="badge badge-pending">1–7 Hari</span>';

        return `<tr>
          <td><code>${escapeHTML(inv.code)}</code></td>
          <td>${fmtShortDate(inv.due_date)}</td>
          <td class="num">${fmtIDR(amount)}</td>
          <td class="num">${fmtIDR(paid)}</td>
          <td class="num" style="font-weight:600; color:${outstanding > 0 ? 'var(--primary)' : 'inherit'};">${fmtIDR(outstanding)}</td>
          <td>${bucketLabel}</td>
          <td>${badge(inv.status)}</td>
        </tr>`;
      }).join('');
    },

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
          if (this._customers.length > 0 && !this._currentAgingCustomer) {
            select.value = this._customers[0].id;
            this._currentAgingCustomer = this._customers[0].id;
          }
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
        const [res, invRes] = await Promise.all([
          api.get(`/v1/customers/${customerID}/aging`),
          api.get(`/v1/invoices?customer_id=${customerID}&limit=200`).catch(() => ({ data: [] }))
        ]);
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

        // Also update aging invoices list with search & pagination
        this._agingInvoices = (invRes.data || []).sort((a, b) => new Date(a.due_date) - new Date(b.due_date));
        this.filterAgingInvoices();
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="4">${emptyState('❌', 'Error', e.message)}</td></tr>`;
        cards.innerHTML = '';
      }
    },

    // ----- Periods -----
    _allPeriods: [],
    _filteredPeriods: [],
    _periodsPaginator: null,

    initPeriodsPaginator() {
      if (!this._periodsPaginator) {
        this._periodsPaginator = new Paginator({
          containerId: 'periods-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderPeriodsTable(),
        });
      }
    },

    filterPeriods() {
      const status = document.getElementById('filter-period-status')?.value || '';
      const q = (document.getElementById('filter-period-search')?.value || '').toLowerCase().trim();
      let list = this._allPeriods || [];
      if (status) {
        list = list.filter(p => p.status === status);
      }
      if (q) {
        list = list.filter(p =>
          (p.id || '').toLowerCase().includes(q) ||
          (p.period_start || p.start_date || '').toLowerCase().includes(q) ||
          (p.period_end || p.end_date || '').toLowerCase().includes(q) ||
          (p.status || '').toLowerCase().includes(q)
        );
      }
      this._filteredPeriods = list;
      this.initPeriodsPaginator();
      this._periodsPaginator.reset();
      this._periodsPaginator.update(this._filteredPeriods.length);
      this.renderPeriodsTable();
    },

    renderPeriodsTable() {
      const tbody = document.querySelector('#periods-table tbody');
      if (!tbody) return;
      if (!this._filteredPeriods || this._filteredPeriods.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6">${emptyState('📅', 'Tidak ada periode yang cocok', 'Coba ubah kata kunci pencarian atau filter status.')}</td></tr>`;
        return;
      }
      const slice = this._periodsPaginator.getCurrentSlice(this._filteredPeriods);
      tbody.innerHTML = slice.map(p => {
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
    },

    async periods() {
      const tbody = document.querySelector('#periods-table tbody');
      tbody.innerHTML = `<tr><td colspan="6">${spinner()}</td></tr>`;
      try {
        const res = await api.get('/v1/periods');
        this._allPeriods = res.data || [];
        this.filterPeriods();
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
    _allRuns: [],
    _filteredRuns: [],
    _reconcilerPeriodMap: new Map(),
    _reconcilerPaginator: null,

    initReconcilerPaginator() {
      if (!this._reconcilerPaginator) {
        this._reconcilerPaginator = new Paginator({
          containerId: 'reconciler-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderReconcilerTable(),
        });
      }
    },

    filterReconciler() {
      const status = document.getElementById('filter-reconciler-status')?.value || '';
      const q = (document.getElementById('filter-reconciler-search')?.value || '').toLowerCase().trim();
      let list = this._allRuns || [];
      if (status) {
        list = list.filter(r => r.status === status);
      }
      if (q) {
        list = list.filter(r => {
          const matchedPeriod = this._reconcilerPeriodMap.get(r.period_id);
          const periodLabel = matchedPeriod
            ? `${matchedPeriod.period_start} ${matchedPeriod.period_end}`
            : '';
          return (r.id || '').toLowerCase().includes(q) ||
                 (r.period_id || '').toLowerCase().includes(q) ||
                 periodLabel.toLowerCase().includes(q) ||
                 (r.status || '').toLowerCase().includes(q);
        });
      }
      this._filteredRuns = list;
      this.initReconcilerPaginator();
      this._reconcilerPaginator.reset();
      this._reconcilerPaginator.update(this._filteredRuns.length);
      this.renderReconcilerTable();
    },

    renderReconcilerTable() {
      const tbody = document.querySelector('#runs-table tbody');
      if (!tbody) return;
      if (!this._filteredRuns || this._filteredRuns.length === 0) {
        tbody.innerHTML = `<tr><td colspan="9">${emptyState('🔍', 'Tidak ada riwayat rekonsiliasi yang cocok', 'Coba ubah kata kunci pencarian atau filter status.')}</td></tr>`;
        return;
      }
      const slice = this._reconcilerPaginator.getCurrentSlice(this._filteredRuns);
      tbody.innerHTML = slice.map(r => {
        let dur = '—';
        if (r.duration_ms) {
          dur = `${(r.duration_ms / 1000).toFixed(2)}s`;
        } else if (r.started_at && r.finished_at) {
          const ms = Math.max(0, new Date(r.finished_at).getTime() - new Date(r.started_at).getTime());
          dur = ms < 1000 ? `${ms}ms` : `${(ms / 1000).toFixed(2)}s`;
        }

        const imbalance = Number(r.imbalance_minor || 0);
        const isImbalanced = imbalance !== 0;
        const isTampered = r.status === 'tampered';
        const rowClass = (isImbalanced || isTampered) ? 'class="row-danger"' : '';

        const matchedPeriod = this._reconcilerPeriodMap.get(r.period_id);
        const periodLabel = matchedPeriod
          ? `${fmtShortDate(matchedPeriod.period_start)} – ${fmtShortDate(matchedPeriod.period_end)}`
          : truncate(r.period_id, 8);

        let hashBadge = '<span class="text-muted" style="font-size:12px;">— Dilewati</span>';
        if (r.hash_chain_ok === true) {
          hashBadge = '<span class="badge badge-balanced" title="Seluruh rantai hash kriptografi SHA-256 valid">✅ Valid</span>';
        } else if (r.hash_chain_ok === false) {
          hashBadge = `<span class="badge badge-tampered" title="Terdeteksi pelanggaran integritas data atau rantai hash terputus">🚨 ${r.hash_chain_errors || 0} Error</span>`;
        }

        return `<tr ${rowClass}>
          <td><code>${truncate(r.id, 8)}</code></td>
          <td><strong>${periodLabel}</strong></td>
          <td>${badge(r.status)}</td>
          <td class="num">${fmtIDR(r.total_debit_minor || 0)}</td>
          <td class="num">${fmtIDR(r.total_credit_minor || 0)}</td>
          <td class="num" style="${isImbalanced ? 'color: var(--error); font-weight: 700;' : ''}">${fmtIDR(imbalance)}</td>
          <td class="text-center">${hashBadge}</td>
          <td>${fmtAgo(r.started_at)}</td>
          <td>${dur}</td>
        </tr>`;
      }).join('');
    },

    async reconciler() {
      const tbody = document.querySelector('#runs-table tbody');
      const periodSelect = document.getElementById('reconciler-period-select');
      tbody.innerHTML = `<tr><td colspan="9">${spinner()}</td></tr>`;
      try {
        const [runsRes, periodsRes] = await Promise.all([
          api.get(`/v1/reconciler/runs?tenant_id=${encodeURIComponent(api.getUser()?.tenant_id || '')}&limit=200`),
          api.get('/v1/periods'),
        ]);
        const periods = periodsRes.data || [];
        this._reconcilerPeriodMap = new Map(periods.map(p => [p.id, p]));

        periodSelect.innerHTML = periods.length
          ? periods.map(p => {
              const startStr = fmtShortDate(p.period_start || p.start_date);
              const endStr = fmtShortDate(p.period_end || p.end_date);
              const status = (p.status || 'open').toUpperCase();
              const statusEmoji = p.status === 'open' ? '🟢' : (p.status === 'closing' ? '🟡' : '🔒');
              return `<option value="${p.id}">${statusEmoji} Periode: ${startStr} s/d ${endStr} (${status})</option>`;
            }).join('')
          : '<option value="">(Belum ada periode akuntansi)</option>';

        this._allRuns = runsRes.data || [];
        this.filterReconciler();
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="9">${emptyState('❌', 'Error', e.message)}</td></tr>`;
      }
    },

    async submitReconciler(form) {
      const fd = new FormData(form);
      const btn = form.querySelector('button[type="submit"]');
      const errEl = document.getElementById('reconciler-error');
      if (errEl) { errEl.hidden = true; errEl.textContent = ''; }

      const origBtnHtml = btn ? btn.innerHTML : '';
      if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<span>⏳ Memproses Rekonsiliasi...</span>';
      }

      const body = {
        tenant_id: api.getUser()?.tenant_id,
        period_id: fd.get('period_id'),
        run_hash_check: fd.get('run_hash_check') === 'on',
      };
      try {
        const res = await api.post('/v1/reconciler/run', body);
        const run = res.data;
        if (run?.status === 'tampered') {
          toast(`🚨 Rekonsiliasi selesai: Terdeteksi manipulasi data! (${run.hash_chain_errors || 0} hash rusak)`, 'error', 6000);
        } else if (run?.status === 'imbalanced') {
          toast(`⚠️ Rekonsiliasi selesai: Terdapat selisih Debit ≠ Kredit!`, 'warning', 5000);
        } else {
          toast(`✅ Rekonsiliasi selesai: Pembukuan seimbang (BALANCED)!`, 'success', 4000);
        }
        await this.reconciler();
      } catch (e) {
        if (errEl) {
          errEl.textContent = `${e.code || 'Error'}: ${e.message}`;
          errEl.hidden = false;
        }
        toast(`${e.code || 'Error'}: ${e.message}`, 'error', 6000);
      } finally {
        if (btn) {
          btn.disabled = false;
          btn.innerHTML = origBtnHtml;
        }
      }
    },

    // ----- Currencies -----
    _currencies: [],
    _filteredCurrencies: [],
    _currenciesPaginator: null,

    initCurrenciesPaginator() {
      if (!this._currenciesPaginator) {
        this._currenciesPaginator = new Paginator({
          containerId: 'currencies-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderCurrenciesTable(),
        });
      }
    },

    filterCurrencies() {
      const q = (document.getElementById('filter-currency-search')?.value || '').toLowerCase().trim();
      let list = this._currencies || [];
      if (q) {
        list = list.filter(c =>
          (c.code || '').toLowerCase().includes(q) ||
          (c.name || '').toLowerCase().includes(q)
        );
      }
      this._filteredCurrencies = list;
      this.initCurrenciesPaginator();
      this._currenciesPaginator.reset();
      this._currenciesPaginator.update(this._filteredCurrencies.length);
      this.renderCurrenciesTable();
    },

    renderCurrenciesTable() {
      const tbody = document.querySelector('#currencies-table tbody');
      if (!tbody) return;
      if (!this._filteredCurrencies || this._filteredCurrencies.length === 0) {
        tbody.innerHTML = `<tr><td colspan="4">${emptyState('💱', 'Mata uang tidak ditemukan', 'Coba cari dengan kode lain.')}</td></tr>`;
        return;
      }
      const slice = this._currenciesPaginator.getCurrentSlice(this._filteredCurrencies);
      tbody.innerHTML = slice.map(c => `
        <tr>
          <td><span class="badge" style="background:#e0f2fe; color:#0369a1; font-weight:700;">${escapeHTML(c.code)}</span></td>
          <td><strong>${escapeHTML(c.name)}</strong></td>
          <td class="num">${c.decimal_places != null ? `${c.decimal_places} desimal` : '—'}</td>
          <td class="text-center"><span class="badge ${c.is_active ? 'active' : 'closed'}">${c.is_active ? 'Aktif' : 'Nonaktif'}</span></td>
        </tr>`).join('');
    },

    async currencies() {
      const tbody = document.querySelector('#currencies-table tbody');
      const fromSel = document.getElementById('convert-from');
      const toSel = document.getElementById('convert-to');
      tbody.innerHTML = `<tr><td colspan="4">${spinner()}</td></tr>`;
      try {
        const res = await api.get('/v1/currencies');
        this._currencies = res.data || [];
        this.filterCurrencies();

        const opts = this._currencies.map(c => `<option value="${escapeHTML(c.code)}">${escapeHTML(c.code)} — ${escapeHTML(c.name)}</option>`).join('');
        fromSel.innerHTML = opts;
        toSel.innerHTML = opts;
        if (!fromSel.value || fromSel.value === 'IDR') fromSel.value = 'USD';
        if (!toSel.value) toSel.value = 'IDR';
        this.updateConvertAddon();
      } catch (e) {
        tbody.innerHTML = `<tr><td colspan="4">${emptyState('❌', 'Error', e.message)}</td></tr>`;
      }
    },

    updateConvertAddon() {
      const fromSel = document.getElementById('convert-from');
      const addon = document.getElementById('convert-currency-addon');
      const hint = document.getElementById('convert-amount-hint');
      if (!fromSel || !addon) return;
      const code = fromSel.value || 'USD';
      addon.textContent = code;
      if (hint) {
        hint.textContent = `💡 Masukkan nominal dalam satuan mata uang ${code} (contoh: ${code === 'IDR' ? '100000 untuk Rp 100.000' : '100 untuk ' + code + ' 100.00'}).`;
      }
    },

    async submitConvert(form) {
      const fd = new FormData(form);
      const fromCurr = fd.get('from_currency');
      const toCurr = fd.get('to_currency');
      const rawAmount = parseFloat(fd.get('amount') || fd.get('amount_minor') || '0');
      if (isNaN(rawAmount) || rawAmount <= 0) {
        toast('Masukkan jumlah nominal yang valid lebih dari 0', 'warning');
        return;
      }

      // Convert standard amount to minor units based on from_currency decimals
      const currObj = this._currencies.find(c => c.code === fromCurr);
      const decimals = currObj?.decimal_places ?? 2;
      const amountMinor = Math.round(rawAmount * Math.pow(10, decimals));

      const body = {
        tenant_id: api.getUser()?.tenant_id,
        from_currency: fromCurr,
        to_currency: toCurr,
        amount_minor: amountMinor,
      };

      const out = document.getElementById('convert-result');
      out.hidden = false;
      out.innerHTML = spinner('Menghitung kurs konversi...');

      const btn = form.querySelector('button[type="submit"]');
      const origBtn = btn ? btn.innerHTML : '';
      if (btn) {
        btn.disabled = true;
        btn.innerHTML = '<span>⏳ Mengonversi...</span>';
      }

      try {
        const res = await api.post('/v1/currencies/convert', body);
        const r = res.data || res;
        const fromMinor = r.from_minor ?? amountMinor;
        const toMinor = r.to_minor ?? r.converted_amount_minor ?? 0;
        const rateStr = r.rate ? Number(r.rate).toLocaleString('id-ID', { maximumFractionDigits: 6 }) : '—';
        const rateId = r.rate_id || r.fx_rate_id;
        const timeStr = r.at ? fmtDateTime(r.at) : 'Baru saja';

        out.innerHTML = `
          <div class="fx-result-card">
            <div class="fx-result-header">
              <span class="fx-result-badge">⚡ Hasil Konversi Kurs Real-time</span>
              <span class="badge badge-active">Live FX Rate</span>
            </div>
            <div class="fx-result-main">
              <div class="fx-result-col">
                <span class="fx-result-sublabel">Nominal Awal</span>
                <span class="fx-result-from">${fmtMinor(fromMinor, r.from_currency || fromCurr)}</span>
              </div>
              <div class="fx-result-equals">➔</div>
              <div class="fx-result-col">
                <span class="fx-result-sublabel">Hasil Konversi</span>
                <span class="fx-result-to">${fmtMinor(toMinor, r.to_currency || toCurr)}</span>
              </div>
            </div>
            <div class="fx-result-details">
              <div class="fx-detail-item">
                <span class="fx-detail-label">Nilai Kurs Acuan:</span>
                <span class="fx-detail-value"><strong>1 ${escapeHTML(fromCurr)} = ${rateStr} ${escapeHTML(toCurr)}</strong></span>
              </div>
              <div class="fx-detail-item">
                <span class="fx-detail-label">Waktu Penetapan:</span>
                <span class="fx-detail-value">${timeStr}</span>
              </div>
              ${rateId ? `
              <div class="fx-detail-item">
                <span class="fx-detail-label">ID Kurs (FX Rate ID):</span>
                <span class="fx-detail-value"><code>${truncate(rateId, 16)}</code></span>
              </div>` : ''}
            </div>
          </div>`;
      } catch (e) {
        out.innerHTML = `
          <div class="result-error-card">
            <p class="error">❌ ${escapeHTML(e.message || 'Gagal menghitung konversi kurs')}</p>
          </div>`;
        toast(`${e.code || 'Error'}: ${e.message}`, 'error');
      } finally {
        if (btn) {
          btn.disabled = false;
          btn.innerHTML = origBtn;
        }
      }
    },

    // ----- Audit -----
    _allAudit: [],
    _filteredAudit: [],
    _auditPaginator: null,

    initAuditPaginator() {
      if (!this._auditPaginator) {
        this._auditPaginator = new Paginator({
          containerId: 'audit-pagination',
          defaultPageSize: 10,
          pageSizes: [10, 25, 50, 100],
          onChange: () => this.renderAuditTable(),
        });
      }
    },

    filterAudit() {
      const searchFilter = (document.getElementById('filter-audit-search')?.value || '').toLowerCase().trim();
      let list = this._allAudit || [];
      if (searchFilter) {
        list = list.filter(a =>
          (a.action || '').toLowerCase().includes(searchFilter) ||
          (a.actor_id || '').toLowerCase().includes(searchFilter) ||
          (a.resource_type || '').toLowerCase().includes(searchFilter) ||
          (a.resource_id || '').toLowerCase().includes(searchFilter) ||
          (a.outcome || a.status || '').toLowerCase().includes(searchFilter) ||
          (a.ip_address || '').toLowerCase().includes(searchFilter)
        );
      }
      this._filteredAudit = list;
      this.initAuditPaginator();
      this._auditPaginator.reset();
      this._auditPaginator.update(this._filteredAudit.length);
      this.renderAuditTable();
    },

    renderAuditTable() {
      const tbody = document.querySelector('#audit-table tbody');
      if (!tbody) return;
      if (!this._filteredAudit || this._filteredAudit.length === 0) {
        tbody.innerHTML = `<tr><td colspan="6">${emptyState('📋', 'No audit entries', 'Audit log fills up as you use the app.')}</td></tr>`;
        return;
      }
      const slice = this._auditPaginator.getCurrentSlice(this._filteredAudit);
      tbody.innerHTML = slice.map(a => `
        <tr>
          <td title="${escapeHTML(a.occurred_at || a.created_at)}">${fmtAgo(a.occurred_at || a.created_at)}</td>
          <td><code>${truncate(a.actor_id, 8)}</code></td>
          <td>${escapeHTML(a.action || '—')}</td>
          <td>${escapeHTML(a.resource_type || '')} <code>${truncate(a.resource_id, 8)}</code></td>
          <td>${badge(a.outcome || a.status || (a.status_code ? String(a.status_code) : 'success'))}</td>
          <td>${escapeHTML(a.ip_address || '—')}</td>
        </tr>`).join('');
    },

    async audit() {
      const tbody = document.querySelector('#audit-table tbody');
      const resourceFilter = document.getElementById('filter-audit-resource').value;
      tbody.innerHTML = `<tr><td colspan="6">${spinner()}</td></tr>`;
      try {
        const qs = resourceFilter ? `?resource_type=${resourceFilter}&limit=500` : '?limit=500';
        const res = await api.get(`/v1/audit${qs}`);
        this._allAudit = res.data || [];
        this.filterAudit();
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
      // Production mode / Option A: remove dev ecosystem toolbar if not on localhost
      const isLocalHost = ['localhost', '127.0.0.1', '::1'].includes(window.location.hostname) || window.location.hostname.endsWith('.local');
      const forceDev = window.location.search.indexOf('dev=true') !== -1 || localStorage.getItem('fledger.show_dev_toolbar') === 'true';
      if (!isLocalHost && !forceDev) {
        document.documentElement.classList.add('prod-mode');
        document.querySelectorAll('.dev-toolbar, .ecosystem-nav, .ecosystem-nav-login').forEach(el => el.remove());
      } else {
        const isDirect = /:808[1-6]/.test(window.location.host);
        if (!isDirect) {
          document.querySelectorAll('.ecosystem-nav a, a.ecosystem-link').forEach(a => {
            const h = a.getAttribute('href');
            if (!h) return;
            if (h.includes(':8000')) a.setAttribute('href', '/hub');
            else if (h.includes(':8081')) a.setAttribute('href', '/core/web/');
            else if (h.includes(':8082')) a.setAttribute('href', '/fleet/');
            else if (h.includes(':8083')) a.setAttribute('href', '/pay/');
            else if (h.includes(':8084')) a.setAttribute('href', '/force/');
            else if (h.includes(':8085')) a.setAttribute('href', '/order/');
            else if (h.includes(':8086')) a.setAttribute('href', '/dunning/');
          });
        }
      }

      // Password visibility toggle
      document.getElementById('toggle-pw')?.addEventListener('click', () => {
        const pw = document.getElementById('password');
        if (pw) pw.type = pw.type === 'password' ? 'text' : 'password';
      });

      // API status indicator
      const updateStatus = async () => {
        const el = document.getElementById('api-status');
        if (el) el.textContent = await api.ping();
      };
      await updateStatus();
      setInterval(updateStatus, 30_000);

      // Quick fill credentials buttons
      const fillAdmin = document.getElementById('fill-admin-btn');
      const fillSales = document.getElementById('fill-sales-btn');
      if (fillAdmin) {
        fillAdmin.addEventListener('click', () => {
          document.getElementById('login-tenant').value = '00000000-0000-0000-0000-000000000001';
          document.getElementById('login-username').value = '33333333-3333-3333-3333-333333333333';
          document.getElementById('password').value = 'DemoTest1234!';
          toast('Kredensial Admin HQ diisikan. Klik "Sign in".', 'info');
        });
      }
      if (fillSales) {
        fillSales.addEventListener('click', () => {
          document.getElementById('login-tenant').value = '00000000-0000-0000-0000-000000000001';
          document.getElementById('login-username').value = '44444444-4444-4444-4444-444444444444';
          document.getElementById('password').value = 'DemoTest1234!';
          toast('Kredensial Sales Joni diisikan. Klik "Sign in".', 'info');
        });
      }

      // Check existing session
      const token = api.getToken();
      const user = api.getUser();
      if (token && user && user.tenant_id) {
        let isExpired = false;
        try {
          const payload = JSON.parse(atob(token.split('.')[1].replace(/-/g, '+').replace(/_/g, '/')));
          if (payload.exp && (Date.now() / 1000) >= payload.exp) {
            isExpired = true;
          }
        } catch { isExpired = true; }

        if (isExpired) {
          const refresh = api.getRefresh();
          let refreshed = false;
          if (refresh) {
            try {
              const res = await api.post('/v1/auth/refresh', { refresh_token: refresh }, { skipAuth: true, skipTenant: true });
              if (res.data && res.data.access_token) {
                api.setToken(res.data.access_token);
                api.setRefresh(res.data.refresh_token);
                refreshed = true;
              }
            } catch (e) {
              console.warn('Auto refresh on boot failed:', e);
            }
          }
          if (!refreshed) {
            await api.logout();
            auth.showLogin();
            toast('Sesi Anda sebelumnya telah kedaluwarsa. Silakan sign in kembali.', 'warning');
            return;
          }
        }

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

      // Profile dropdown toggle & close on outside click
      const profileBtn = document.getElementById('user-profile-btn');
      const dropdownCard = document.getElementById('user-dropdown-card');

      profileBtn?.addEventListener('click', (e) => {
        e.stopPropagation();
        const isHidden = dropdownCard?.hasAttribute('hidden');
        if (isHidden) {
          dropdownCard?.removeAttribute('hidden');
          profileBtn?.classList.add('active');
          profileBtn?.setAttribute('aria-expanded', 'true');
        } else {
          dropdownCard?.setAttribute('hidden', '');
          profileBtn?.classList.remove('active');
          profileBtn?.setAttribute('aria-expanded', 'false');
        }
      });

      document.addEventListener('click', (e) => {
        if (!dropdownCard?.hasAttribute('hidden') && !dropdownCard?.contains(e.target) && !profileBtn?.contains(e.target)) {
          dropdownCard?.setAttribute('hidden', '');
          profileBtn?.classList.remove('active');
          profileBtn?.setAttribute('aria-expanded', 'false');
        }
      });

      // Copy User ID and Tenant ID
      document.getElementById('btn-copy-user-id')?.addEventListener('click', () => {
        const txt = document.getElementById('dropdown-user-id')?.textContent;
        if (txt) {
          navigator.clipboard.writeText(txt);
          toast('User ID disalin ke clipboard!', 'success');
        }
      });
      document.getElementById('btn-copy-tenant-id')?.addEventListener('click', () => {
        const txt = document.getElementById('dropdown-tenant-id')?.textContent;
        if (txt) {
          navigator.clipboard.writeText(txt);
          toast('Tenant ID disalin ke clipboard!', 'success');
        }
      });

      // Dropdown Logout handler
      const handleLogout = async () => {
        dropdownCard?.setAttribute('hidden', '');
        profileBtn?.classList.remove('active');
        await api.logout();
        auth.showLogin();
        toast('Berhasil keluar (Logged out)', 'info');
      };
      document.getElementById('dropdown-logout-btn')?.addEventListener('click', handleLogout);

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

      // Currency dropdown change
      document.getElementById('convert-from')?.addEventListener('change', () => views.updateConvertAddon());

      // Currency swap button
      document.getElementById('btn-swap-currency')?.addEventListener('click', () => {
        const fromSel = document.getElementById('convert-from');
        const toSel = document.getElementById('convert-to');
        if (!fromSel || !toSel) return;
        const temp = fromSel.value;
        fromSel.value = toSel.value;
        toSel.value = temp;
        views.updateConvertAddon();
      });

      // Preset chips
      document.querySelectorAll('.preset-chips .chip-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          const input = document.getElementById('convert-amount');
          if (input) {
            input.value = btn.dataset.preset;
            input.focus();
          }
        });
      });

      // New Account button
      document.getElementById('new-account-btn').addEventListener('click', () => showNewAccountModal());

      // New Invoice button
      document.getElementById('new-invoice-btn')?.addEventListener('click', () => views.showNewInvoiceModal());

      // Account filters
      document.getElementById('filter-account-type')?.addEventListener('change', () => views.filterAccounts());
      document.getElementById('filter-account-search')?.addEventListener('input', debounce(() => views.filterAccounts(), 300));

      // Transfer search
      document.getElementById('filter-transfer-search')?.addEventListener('input', debounce(() => views.filterTransfers(), 300));

      // Invoice filters (status & search)
      document.getElementById('filter-invoice-status')?.addEventListener('change', () => views.filterInvoices());
      document.getElementById('filter-invoice-search')?.addEventListener('input', debounce(() => views.filterInvoices(), 300));

      // Aging customer invoice filters (status & search) & refresh
      document.getElementById('filter-aging-status')?.addEventListener('change', () => views.filterAgingInvoices());
      document.getElementById('filter-aging-search')?.addEventListener('input', debounce(() => views.filterAgingInvoices(), 300));
      document.getElementById('aging-refresh-btn')?.addEventListener('click', () => {
        const sel = document.getElementById('aging-customer-select');
        if (sel?.value) views.loadAgingFor(sel.value);
      });

      // Period filters (status & search)
      document.getElementById('filter-period-status')?.addEventListener('change', () => views.filterPeriods());
      document.getElementById('filter-period-search')?.addEventListener('input', debounce(() => views.filterPeriods(), 300));

      // Reconciler filters (status & search)
      document.getElementById('filter-reconciler-status')?.addEventListener('change', () => views.filterReconciler());
      document.getElementById('filter-reconciler-search')?.addEventListener('input', debounce(() => views.filterReconciler(), 300));

      // Currency search
      document.getElementById('filter-currency-search')?.addEventListener('input', debounce(() => views.filterCurrencies(), 300));

      // Audit filters
      document.getElementById('filter-audit-resource')?.addEventListener('change', () => views.audit());
      document.getElementById('filter-audit-search')?.addEventListener('input', debounce(() => views.filterAudit(), 300));

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
