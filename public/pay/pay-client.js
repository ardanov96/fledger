// Fledger Pay Client SDK ??? minimal fetch wrapper used by the simulator UI.
(function (global) {
  'use strict';

  const subpath = location.pathname.startsWith('/pay') ? '/pay' : '';
  const baseURL = (location.origin && location.origin !== 'null') ? (location.origin + subpath) : 'http://localhost:8083';

  // Persistent token storage
  let savedToken = '';
  try {
    savedToken = localStorage.getItem('fledger_pay_token') || '';
  } catch (e) {}

  function headers(extra, skipAuth) {
    const h = Object.assign({
      'Content-Type': 'application/json',
      'X-Tenant-ID': 'a0000000-0000-0000-0000-000000000001',
      'X-Actor-Id': 'simulator-ui',
    }, extra || {});

    if (!skipAuth) {
      const tok = PayClient.token || savedToken;
      if (tok) {
        h['Authorization'] = 'Bearer ' + tok;
      }
    }
    return h;
  }

  async function call(method, path, body, extraHeaders, isRetry) {
    const isLogin = path.includes('/dev/login');
    const init = {
      method,
      headers: headers(extraHeaders, isLogin),
    };
    if (body !== undefined && body !== null) {
      init.body = JSON.stringify(body);
    }
    const res = await fetch(baseURL + path, init);

    // Auto-refresh token on 401 Unauthorized
    if (res.status === 401 && !isRetry && !isLogin) {
      console.warn('PayClient: 401 Unauthorized received, refreshing dev token...');
      PayClient.token = '';
      savedToken = '';
      try { localStorage.removeItem('fledger_pay_token'); } catch (e) {}
      try {
        await PayClient.login();
        return await call(method, path, body, extraHeaders, true);
      } catch (loginErr) {
        console.error('PayClient: auto-login recovery failed:', loginErr);
      }
    }

    const text = await res.text();
    let parsed;
    try { parsed = text ? JSON.parse(text) : {}; } catch (e) { parsed = { raw: text }; }
    if (!res.ok) {
      const err = new Error(parsed.message || ('HTTP ' + res.status));
      err.status = res.status;
      err.body = parsed;
      throw err;
    }
    return parsed;
  }

  // Public API
  const PayClient = {
    baseURL: baseURL,
    token: savedToken,

    /** Mint a dev JWT (APP_ENV=development only). */
    login: async function (tenantId) {
      const q = tenantId ? '?tenant_id=' + encodeURIComponent(tenantId) : '';
      const res = await call('POST', '/v1/dev/login' + q);
      if (res && res.access_token) {
        PayClient.token = res.access_token;
        savedToken = res.access_token;
        try { localStorage.setItem('fledger_pay_token', res.access_token); } catch(e) {}
      }
      return res;
    },

    /** Create payment request. */
    createRequest: function (body) {
      return call('POST', '/v1/pay/requests', body);
    },

    /** Get one payment request. */
    getRequest: function (id) {
      return call('GET', '/v1/pay/requests/' + id);
    },

    /** Simulate a bank payment (sandbox). */
    simulateSettle: function (body) {
      return call('POST', '/v1/pay/simulator/settle', body);
    },

    /** Outbox dashboard counts. */
    outboxCounts: function () {
      return call('GET', '/v1/pay/outbox/counts');
    },
  };

  global.PayClient = PayClient;
})(window);