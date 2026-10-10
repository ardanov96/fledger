// Fledger Order PWA client SDK.
(function (global) {
  'use strict';
  const baseURL = (location.origin && location.origin !== 'null') ? location.origin : 'http://localhost:8085';

  function headers(extra) {
    return Object.assign({
      'Content-Type': 'application/json',
      'X-Tenant-ID': '00000000-0000-0000-0000-000000000001',
      'X-Actor-Id': 'pwa-user',
    }, extra || {});
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

  const Order = {
    baseURL,
    login: (tid) => call('POST', '/v1/dev/login' + (tid ? '?tenant_id=' + tid : '')),
    listProducts: (cat) => call('GET', '/v1/order/products' + (cat ? '?category=' + cat : '')),
    createProduct: (body) => call('POST', '/v1/order/products', body),
    createPricing: (productId, body) => call('POST', '/v1/order/products/' + productId + '/pricing', body),
    adjustStock: (body) => call('POST', '/v1/order/inventory/adjust', body),
    listInventory: (productId) => call('GET', '/v1/order/inventory?product_id=' + productId),
    createOrder: (body) => call('POST', '/v1/order/orders', body),
    getOrder: (id) => call('GET', '/v1/order/orders/' + id),
    evaluateCredit: (id) => call('POST', '/v1/order/orders/' + id + '/evaluate-credit'),
    overrideCredit: (id, body) => call('POST', '/v1/order/orders/' + id + '/override-credit', body),
    cancelOrder: (id) => call('POST', '/v1/order/orders/' + id + '/cancel'),
    outboxCounts: () => call('GET', '/v1/order/outbox/counts'),
  };
  global.Order = Order;
})(window);