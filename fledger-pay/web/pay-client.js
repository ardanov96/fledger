// Fledger Pay Client SDK — minimal fetch wrapper used by the simulator UI.
(function (global) {
  'use strict';

  const baseURL = (location.origin && location.origin !== 'null') ? location.origin : 'http://localhost:8083';

  function headers(extra) {
    const h = Object.assign({
      'Content-Type': 'application/json',
      'X-Tenant-ID': '00000000-0000-0000-0000-000000000001',
      'X-Actor-Id': 'simulator-ui',
    }, extra || {});
    return h;
  }

  async function call(method, path, body, extraHeaders) {
    const init = {
      method,
      headers: headers(extraHeaders),
    };
    if (body !== undefined && body !== null) {
      init.body = JSON.stringify(body);
    }
    const res = await fetch(baseURL + path, init);
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

    /** Mint a dev JWT (APP_ENV=development only). */
    login: function (tenantId) {
      const q = tenantId ? '?tenant_id=' + encodeURIComponent(tenantId) : '';
      return call('POST', '/v1/dev/login' + q);
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