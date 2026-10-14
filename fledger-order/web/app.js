// Fledger Order PWA — UI controller (Enhanced Launch UI Style).
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
    if (!el) return;
    el.className = 'status ' + (kind || '');
    el.textContent = msg;
  }

  const categoryLabels = {
    'SEMBAKO': 'Sembako',
    'MIE_INSTAN': 'Mie Instan',
    'MINUMAN': 'Minuman',
    'TOILETRIES': 'Toiletries',
    'ROKOK': 'Rokok',
  };

  const cart = new Map(); // productId -> { product, qty }

  document.addEventListener('DOMContentLoaded', async () => {
    $('customer_id').value = uuid();
    $('load-catalog').addEventListener('click', loadCatalog);
    $('submit-order').addEventListener('click', submitOrder);
    $('evaluate-credit').addEventListener('click', evaluateCredit);
    $('override-form').addEventListener('submit', (e) => { e.preventDefault(); doOverride(); });

    // Quick Store Buttons
    document.querySelectorAll('.quick-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        $('customer_name').value = btn.dataset.name;
        $('customer_tier').value = btn.dataset.tier;
        $('destination_address').value = btn.dataset.addr;
        $('customer_id').value = uuid();
      });
    });

    try {
      const r = await Order.login();
      $('who').textContent = 'Token aktif (berakhir ' + new Date(r.expires_at).toLocaleTimeString('id-ID') + ')';
    } catch (e) {
      $('who').textContent = 'Dev Mode (Local)';
    }
    loadCatalog();
    refreshOutbox();
    setInterval(refreshOutbox, 5000);
  });

  async function loadCatalog() {
    $('catalog').innerHTML = '<div style="padding:20px; color:#94a3b8;">Memuat katalog produk FMCG…</div>';
    const cat = $('catalog-category').value;
    try {
      const items = await Order.listProducts(cat);
      if (!items.length) {
        $('catalog').innerHTML = '<div style="padding:20px; color:#94a3b8;">Katalog kosong untuk filter kategori ini.</div>';
        return;
      }
      $('catalog').innerHTML = '';
      for (const p of items) {
        const catLabel = categoryLabels[p.category] || p.category;
        const el = document.createElement('div');
        el.className = 'product-card';
        el.innerHTML = `
          <div>
            <div class="product-badge-row">
              <span class="sku-pill">${p.sku}</span>
              <span class="cat-pill">${catLabel}</span>
            </div>
            <div class="product-title">${p.name}</div>
            <div class="product-meta">Satuan: <strong>${p.unit}</strong> &bull; Bobot: <strong>${p.weight_grams} g</strong></div>
            <div class="product-stock-row">
              <span style="color:#94a3b8;">Alokasi Stok:</span>
              <span class="stock-tag">Ready &bull; 1.000 Unit</span>
            </div>
          </div>
          <div class="product-action-row">
            <div class="qty-input-group">
              <input type="number" min="1" value="10" id="qty-${p.id}" class="tabular-nums" />
            </div>
            <button class="primary" style="flex:1;" data-add="${p.id}">
              + Keranjang
            </button>
          </div>
        `;
        el.querySelector('[data-add]').addEventListener('click', () => addToCart(p));
        $('catalog').appendChild(el);
      }
    } catch (e) {
      $('catalog').innerHTML = `<div style="padding:20px; color:#ef4444;">Gagal memuat katalog: ${e.message}</div>`;
    }
  }

  function addToCart(p) {
    const qtyEl = $('qty-' + p.id);
    const qty = parseInt(qtyEl.value, 10) || 1;
    const current = cart.get(p.id);
    const totalQty = current ? current.qty + qty : qty;
    cart.set(p.id, { product: p, qty: totalQty });
    renderCart();
  }

  function renderCart() {
    if (cart.size === 0) {
      $('cart-body').innerHTML = '<tr><td colspan="6" class="center muted" style="padding:24px;">Belum ada item di keranjang. Klik "+ Keranjang" pada katalog di atas.</td></tr>';
      $('cart-total').textContent = 'Rp 0';
      return;
    }
    let total = 0;
    let totalWeightGrams = 0;
    let rows = '';
    for (const { product, qty } of cart.values()) {
      const est = (product.weight_grams || 0) * 50;
      const sub = est * qty;
      total += sub;
      totalWeightGrams += (product.weight_grams || 0) * qty;
      rows += `<tr>
        <td class="sku"><span class="sku-pill">${product.sku}</span></td>
        <td><strong>${product.name}</strong></td>
        <td class="center tabular-nums"><strong>${qty}</strong></td>
        <td class="right tabular-nums">~${fmtIDR(est)}</td>
        <td class="right tabular-nums" style="color:#38bdf8; font-weight:600;">${fmtIDR(sub)}</td>
        <td class="center"><button class="remove-btn" title="Hapus item" data-rm="${product.id}">✕</button></td>
      </tr>`;
    }
    $('cart-body').innerHTML = rows;
    $('cart-total').innerHTML = `${fmtIDR(total)} <small style="color:#94a3b8; font-size:0.75rem; font-weight:normal;">(${(totalWeightGrams / 1000).toFixed(2)} kg)</small>`;
    $('cart-body').querySelectorAll('[data-rm]').forEach((b) => {
      b.addEventListener('click', () => { cart.delete(b.dataset.rm); renderCart(); });
    });
  }

  async function submitOrder() {
    if (cart.size === 0) { setStatus($('order-status'), 'Keranjang belanja masih kosong! Tambahkan produk terlebih dahulu.', 'err'); return; }
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
    setStatus($('order-status'), 'Membuat pesanan B2B dan memesan alokasi stok…', '');
    try {
      const out = await Order.createOrder(body);
      const o = out.order;
      setStatus($('order-status'),
        `✓ Order Berhasil Terbit!\n• No Pesanan: ${o.order_number}\n• Subtotal: ${fmtIDR(o.subtotal)} | Berat Muatan: ${o.total_weight_kg} kg\n• Status Order: ${o.status}\nSilakan lanjutkan evaluasi Credit Gate pada langkah 3 di bawah.`,
        'ok');
      $('credit-card').hidden = false;
      $('credit-card').dataset.orderId = o.id;
      $('override-block').hidden = true;
      $('credit-metrics-container').innerHTML = '';
      $('credit-card').scrollIntoView({ behavior: 'smooth' });
    } catch (err) {
      setStatus($('order-status'), 'Gagal membuat pesanan: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
    }
  }

  async function evaluateCredit() {
    const orderId = $('credit-card').dataset.orderId;
    if (!orderId) { setStatus($('credit-status'), 'Belum ada pesanan aktif untuk dievaluasi.', 'err'); return; }
    setStatus($('credit-status'), 'Mengevaluasi Hard Credit Gate secara real-time ke Fledger Core…', '');
    try {
      const res = await Order.evaluateCredit(orderId);
      const pass = res.credit_gate_status === 'PASSED';
      const ev = res.evaluation;

      const limit = ev.credit_limit_minor || 1;
      const projected = ev.projected_ar_minor || 0;
      const pct = Math.min(100, Math.round((projected / limit) * 100));
      const barClass = pass ? (pct > 75 ? 'warning' : 'normal') : 'danger';

      $('credit-metrics-container').innerHTML = `
        <div class="credit-gate-box">
          <div class="credit-meter-wrap">
            <div class="credit-meter-labels">
              <span>Proyeksi Pemakaian Plafon Piutang: <strong>${pct}%</strong></span>
              <span>Batas Limit: <strong>${fmtIDR(limit)}</strong></span>
            </div>
            <div class="credit-bar-bg">
              <div class="credit-bar-fill ${barClass}" style="width: ${pct}%"></div>
            </div>
          </div>
          <div class="credit-metrics-grid">
            <div class="metric-card">
              <div class="metric-card-label">Outstanding AR Saat Ini</div>
              <div class="metric-card-value">${fmtIDR(ev.outstanding_ar_minor)}</div>
            </div>
            <div class="metric-card">
              <div class="metric-card-label">Nilai Order Baru</div>
              <div class="metric-card-value" style="color:#38bdf8;">${fmtIDR(ev.new_order_minor)}</div>
            </div>
            <div class="metric-card">
              <div class="metric-card-label">Total Proyeksi AR</div>
              <div class="metric-card-value">${fmtIDR(ev.projected_ar_minor)}</div>
            </div>
            <div class="metric-card">
              <div class="metric-card-label">Tunggakan >30 Hari</div>
              <div class="metric-card-value" style="color:${ev.has_overdue_30d ? '#ef4444' : '#10b981'};">
                ${ev.has_overdue_30d ? '⚠️ YA (' + ev.oldest_overdue_days + ' hari)' : '✓ Tidak Ada'}
              </div>
            </div>
            <div class="metric-card">
              <div class="metric-card-label">Hasil Evaluasi Gate</div>
              <div class="metric-card-value" style="color:${pass ? '#10b981' : '#ef4444'};">
                ${pass ? '✓ PASSED (Disetujui)' : '⛔ ' + res.credit_gate_status}
              </div>
            </div>
          </div>
        </div>
      `;

      setStatus($('credit-status'),
        pass ? '✓ Hard Credit Gate Lolos: Pesanan diteruskan ke Dispatch Fleet Logistik.'
             : '⛔ Pesanan Tertahan: Melanggar batas kredit atau memiliki tunggakan >30 hari.',
        pass ? 'ok' : 'err');

      $('override-block').hidden = pass;
      if (pass) refreshOutbox();
    } catch (err) {
      setStatus($('credit-status'), 'Kesalahan evaluasi credit gate: ' + err.message, 'err');
    }
  }

  async function doOverride() {
    const orderId = $('credit-card').dataset.orderId;
    if (!orderId) { setStatus($('override-status'), 'Belum ada pesanan aktif.', 'err'); return; }
    setStatus($('override-status'), 'Mengirimkan otorisasi override PIN ke sistem…', '');
    try {
      const o = await Order.overrideCredit(orderId, {
        override_pin: $('override_pin').value,
        reason: $('override_reason').value,
      });
      setStatus($('override-status'),
        `✓ Override Berhasil Disetujui!\nStatus Order: ${o.status} | Credit Gate: ${o.credit_gate_status}\nOtorisasi oleh: ${o.override_by}`,
        'ok');
      $('override-block').hidden = true;
      refreshOutbox();
    } catch (err) {
      setStatus($('override-status'), 'Override Gagal: ' + err.message + (err.body ? '\n' + JSON.stringify(err.body) : ''), 'err');
    }
  }

  function refreshOutbox() {
    Order.outboxCounts()
      .then((c) => {
        $('outbox-counts').innerHTML = `
          <div class="kpi-card pending">
            <div class="kpi-title">Pending</div>
            <div class="kpi-value">${c.pending ?? 0}</div>
          </div>
          <div class="kpi-card processing">
            <div class="kpi-title">Processing</div>
            <div class="kpi-value">${c.processing ?? 0}</div>
          </div>
          <div class="kpi-card sent">
            <div class="kpi-title">Sent / Dispatched</div>
            <div class="kpi-value">${c.sent ?? 0}</div>
          </div>
          <div class="kpi-card failed">
            <div class="kpi-title">Failed</div>
            <div class="kpi-value">${c.failed ?? 0}</div>
          </div>
        `;
      })
      .catch((err) => {
        $('outbox-counts').innerHTML = `<div style="color:#ef4444; padding:12px;">Gagal memuat status outbox: ${err.message}</div>`;
      });
  }
})();