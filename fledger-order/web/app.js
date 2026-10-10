// Fledger Order PWA — UI controller.
(function () {
  'use strict';
  const $ = (id) => document.getElementById(id);
  const fmtIDR = (n) => 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(n) || 0);
  const uuid = () => (window.crypto && window.crypto.randomUUID)
    ? window.crypto.randomUUID()
    : 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
      });

  function setStatus(el, msg, kind) {
    el.className = 'status ' + (kind || '');
    el.textContent = msg;
  }
  function setKv(target, pairs) {
    let html = '<div class="kv">';
    for (const [k, v] of pairs) html += `<div><strong>${k}:</strong> <span>${v}</span></div>`;
    html += '</div>';
    target.innerHTML = html;
  }

  const cart = new Map(); // productId -> { product, qty }

  document.addEventListener('DOMContentLoaded', async () => {
    $('customer_id').value = uuid();
    $('load-catalog').addEventListener('click', loadCatalog);
    $('submit-order').addEventListener('click', submitOrder);
    $('evaluate-credit').addEventListener('click', evaluateCredit);
    $('override-form').addEventListener('submit', (e) => { e.preventDefault(); doOverride(); });

    try {
      const r = await Order.login();
      $('who').textContent = 'Token aktif (berakhir ' + new Date(r.expires_at).toLocaleString('id-ID') + ')';
    } catch (e) {
      $('who').textContent = 'PWA offline/dev mode: ' + (e.message || e);
    }
    loadCatalog();
    refreshOutbox();
    setInterval(refreshOutbox, 5000);
  });

  async function loadCatalog() {
    setStatus(elm('catalog'), 'Memuat katalog…', '');
    const cat = $('catalog-category').value;
    const items = await Order.listProducts(cat);
    if (!items.length) {
      $('catalog').innerHTML = '<em class="muted">Katalog kosong untuk filter ini.</em>';
      return;
    }
    $('catalog').innerHTML = '';
    for (const p of items) {
      const el = document.createElement('div');
      el.className = 'catalog-item';
      el.innerHTML = `
        <div><strong>${p.sku}</strong> — ${p.name}</div>
        <div class="muted">${p.category} • ${p.unit} • ${p.weight_grams} g</div>
        <div style="margin-top:6px;display:flex;gap:8px;align-items:center;">
          <input type="number" min="1" value="10" id="qty-${p.id}" style="width:80px;" />
          <button class="primary" data-add="${p.id}">+ Keranjang</button>
        </div>
      `;
      el.querySelector('[data-add]').addEventListener('click', () => addToCart(p));
      $('catalog').appendChild(el);
    }
  }

  function addToCart(p) {
    const qtyEl = $('qty-' + p.id);
    const qty = parseInt(qtyEl.value, 10) || 1;
    cart.set(p.id, { product: p, qty: qty });
    renderCart();
  }

  function renderCart() {
    if (cart.size === 0) {
      $('cart-body').innerHTML = '<tr><td colspan="6" class="muted">Belum ada item. Klik "Tambah" di katalog.</td></tr>';
      $('cart-total').textContent = 'Rp 0';
      return;
    }
    let total = 0;
    let rows = '';
    for (const { product, qty } of cart.values()) {
      // Use a heuristic price (we don't know the real per-tier price from the
      // catalog endpoint yet). The order endpoint looks up the real price.
      const est = (product.weight_grams || 0) * 50;
      const sub = est * qty;
      total += sub;
      rows += `<tr>
        <td>${product.sku}</td>
        <td>${product.name}</td>
        <td>${qty}</td>
        <td>~${fmtIDR(est)}</td>
        <td>${fmtIDR(sub)}</td>
        <td><button class="secondary" data-rm="${product.id}">×</button></td>
      </tr>`;
    }
    $('cart-body').innerHTML = rows;
    $('cart-total').textContent = fmtIDR(total);
    $('cart-body').querySelectorAll('[data-rm]').forEach((b) => {
      b.addEventListener('click', () => { cart.delete(b.dataset.rm); renderCart(); });
    });
  }

  function elm(id) { return document.getElementById(id); }

  async function submitOrder() {
    if (cart.size === 0) { setStatus($('order-status'), 'Keranjang kosong', 'err'); return; }
    const items = [];
    for (const { product, qty } of cart.values()) items.push({ product_id: product.id, quantity: qty });
    const body = {
      customer_id: $('customer_id').value,
      customer_name: $('customer_name').value,
      customer_tier: $('customer_tier').value,
      customer_phone: $('customer_phone').value,
      destination_address: $('destination_address').value,
      items,
    };
    setStatus($('order-status'), 'Membuat pesanan…', '');
    try {
      const out = await Order.createOrder(body);
      const o = out.order;
      setStatus($('order-status'),
        `Order: ${o.order_number}\nSubtotal: ${fmtIDR(o.subtotal)} • Berat: ${o.total_weight_kg} kg\nStatus: ${o.status}`,
        'ok');
      $('credit-card').hidden = false;
      $('credit-card').dataset.orderId = o.id;
      $('override-block').hidden = true;
    } catch (err) {
      setStatus($('order-status'), 'ERR: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
    }
  }

  async function evaluateCredit() {
    const orderId = $('credit-card').dataset.orderId;
    if (!orderId) { setStatus($('credit-status'), 'Belum ada order', 'err'); return; }
    setStatus($('credit-status'), 'Mengevaluasi credit gate ke Fledger Core…', '');
    try {
      const res = await Order.evaluateCredit(orderId);
      let pass = res.credit_gate_status === 'PASSED';
      let kv = [
        ['Credit Limit', fmtIDR(res.evaluation.credit_limit_minor)],
        ['Outstanding AR', fmtIDR(res.evaluation.outstanding_ar_minor)],
        ['New Order', fmtIDR(res.evaluation.new_order_minor)],
        ['Projected AR', fmtIDR(res.evaluation.projected_ar_minor)],
        ['Overdue > 30d', res.evaluation.has_overdue_30d ? 'YES' : 'no'],
        ['Oldest Overdue (days)', res.evaluation.oldest_overdue_days],
        ['Status', res.credit_gate_status],
      ];
      $('credit-status').innerHTML = '<div class="kv">' + kv.map(([k, v]) => `<div><strong>${k}:</strong> <span>${v}</span></div>`).join('') + '</div>';
      setStatus($('credit-status'),
        'Gate: ' + res.credit_gate_status + (res.evaluation.reason ? ' — ' + res.evaluation.reason : ''),
        pass ? 'ok' : 'err');
      $('override-block').hidden = pass;
      if (pass) refreshOutbox();
    } catch (err) {
      setStatus($('credit-status'), 'ERR: ' + err.message, 'err');
    }
  }

  async function doOverride() {
    const orderId = $('credit-card').dataset.orderId;
    if (!orderId) { setStatus($('override-status'), 'Belum ada order', 'err'); return; }
    setStatus($('override-status'), 'Mencoba override…', '');
    try {
      const o = await Order.overrideCredit(orderId, {
        override_pin: $('override_pin').value,
        reason: $('override_reason').value,
      });
      setStatus($('override-status'),
        `Override OK. status=${o.status} credit_gate=${o.credit_gate_status} oleh ${o.override_by}`,
        'ok');
      $('override-block').hidden = true;
      refreshOutbox();
    } catch (err) {
      setStatus($('override-status'), 'ERR: ' + err.message + (err.body ? '\n' + JSON.stringify(err.body) : ''), 'err');
    }
  }

  function refreshOutbox() {
    Order.outboxCounts()
      .then((c) => {
        setKv($('outbox-counts'), [
          ['Pending', c.pending], ['Processing', c.processing],
          ['Sent', c.sent], ['Failed', c.failed],
        ]);
      })
      .catch((err) => { $('outbox-counts').textContent = 'ERR: ' + err.message; });
  }
})();