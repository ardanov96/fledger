// Fledger Dunning PWA client SDK.
(function (global) {
  'use strict';
  const baseURL = (location.origin && location.origin !== 'null') ? location.origin : 'http://localhost:8086';
  const TENANT = 'a0000000-0000-0000-0000-000000000001';

  function headers(extra) {
    const h = { 'Content-Type': 'application/json', 'X-Tenant-ID': TENANT, 'X-Actor-Id': 'pwa-user' };
    try {
      const t = localStorage.getItem('fdunning_token');
      if (t) h['Authorization'] = 'Bearer ' + t;
    } catch (e) {}
    return Object.assign(h, extra || {});
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

  const FD = {
    baseURL, tenant: TENANT,
    login: () => call('POST', '/v1/dev/login?tenant_id=' + TENANT),
    listQueues: (status) => call('GET', '/v1/dunning/queues' + (status ? '?status=' + status : '')),
    ingestInvoice: (body) => call('POST', '/v1/dunning/queues/ingest-invoice', body),
    cancelInvoice: (invoiceId) => call('POST', '/v1/dunning/queues/cancel-invoice', { invoice_id: invoiceId }),
    dispatchQueue: (id) => call('POST', '/v1/dunning/queues/' + id + '/dispatch'),
    cronRun: () => call('POST', '/v1/dunning/queues/cron-run'),
    queueCounts: () => call('GET', '/v1/dunning/outbox/counts'),
    whatsappStatus: () => call('GET', '/v1/dunning/whatsapp/status'),
    whatsappQR: (sessionName) => call('POST', '/v1/dunning/whatsapp/qr/generate', { session_name: sessionName }),
    whatsappDisconnect: () => call('POST', '/v1/dunning/whatsapp/disconnect', {}),
    whatsappTestSend: (body) => call('POST', '/v1/dunning/whatsapp/test-send', body),
    listStatements: () => call('GET', '/v1/dunning/statements'),
    generateStatement: (body) => call('POST', '/v1/dunning/statements/generate', body),
    sendStatement: (id) => call('POST', '/v1/dunning/statements/' + id + '/send'),
    listContacts: () => call('GET', '/v1/dunning/contacts'),
    upsertContact: (body) => call('POST', '/v1/dunning/contacts', body),
  };
  global.FD = FD;
})(window);