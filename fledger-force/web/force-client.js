// Fledger Force PWA client SDK.
(function (global) {
  'use strict';
  const baseURL = (location.origin && location.origin !== 'null') ? location.origin : 'http://localhost:8084';

  let savedToken = '';
  try {
    savedToken = localStorage.getItem('fledger_force_token') || '';
  } catch (e) {}

  function headers(extra) {
    const h = Object.assign({
      'Content-Type': 'application/json',
      'X-Tenant-ID': '00000000-0000-0000-0000-000000000001',
      'X-Actor-Id': 'salesman-pwa',
    }, extra || {});

    const tok = (global.Force && global.Force.token) || savedToken;
    if (tok) {
      h['Authorization'] = 'Bearer ' + tok;
    }
    return h;
  }

  async function call(method, path, body, extra) {
    const init = { method, headers: headers(extra) };
    if (body !== undefined) init.body = JSON.stringify(body);
    const res = await fetch(baseURL + path, init);
    const text = await res.text();
    let parsed;
    try { parsed = text ? JSON.parse(text) : {}; } catch (e) { parsed = { raw: text }; }
    if (!res.ok) throw Object.assign(new Error(parsed.message || ('HTTP ' + res.status)), { status: res.status, body: parsed });
    return parsed;
  }

  const Force = {
    baseURL,
    token: savedToken,
    login: async (tid) => {
      const res = await call('POST', '/v1/dev/login' + (tid ? '?tenant_id=' + tid : ''));
      if (res && res.access_token) {
        Force.token = res.access_token;
        savedToken = res.access_token;
        try { localStorage.setItem('fledger_force_token', res.access_token); } catch (e) {}
      }
      return res;
    },
    listReps: () => call('GET', '/v1/force/sales-reps'),
    getRep: (id) => call('GET', '/v1/force/sales-reps/' + id),
    listStores: () => call('GET', '/v1/force/stores'),
    createRep: (body) => call('POST', '/v1/force/sales-reps', body),
    createStore: (body) => call('POST', '/v1/force/stores', body),
    todayBeatPlans: (repId) => call('GET', '/v1/force/beat-plans/today?sales_rep_id=' + repId),
    createBeatPlan: (body) => call('POST', '/v1/force/beat-plans', body),
    checkIn: (salesRepId, body) => {
      const b = Object.assign({}, body);
      if (salesRepId && !b.sales_rep_id) b.sales_rep_id = salesRepId;
      return call('POST', '/v1/force/visits/check-in' + (salesRepId ? '?sales_rep_id=' + salesRepId : ''), b);
    },
    completeVisit: (id) => call('POST', '/v1/force/visits/' + id + '/complete'),
    collect: (salesRepId, body) => {
      const b = Object.assign({}, body);
      if (salesRepId && !b.sales_rep_id) b.sales_rep_id = salesRepId;
      return call('POST', '/v1/force/collections' + (salesRepId ? '?sales_rep_id=' + salesRepId : ''), b);
    },
    listCollectionsToday: (repId) => call('GET', '/v1/force/collections/today?sales_rep_id=' + repId),
    inquiry: (repId) => call('GET', '/v1/force/settlements/reps/' + repId + '/inquiry'),
    settle: (body) => call('POST', '/v1/force/settlements', body),
    outboxCounts: () => call('GET', '/v1/force/outbox/counts'),
  };
  global.Force = Force;
})(window);