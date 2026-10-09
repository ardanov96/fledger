// Fledger Force PWA — UI controller.
(function () {
  'use strict';
  const $ = (id) => document.getElementById(id);
  const fmtIDR = (n) => 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(n) || 0);

  function setStatus(el, msg, kind) {
    el.className = 'status ' + (kind || '');
    el.textContent = msg;
  }

  function setKv(target, pairs) {
    let html = '<div class="kv">';
    for (const [k, v] of pairs) {
      html += `<div><strong>${k}:</strong> <span>${v}</span></div>`;
    }
    html += '</div>';
    target.innerHTML = html;
  }

  function uuid() {
    if (window.crypto && window.crypto.randomUUID) return window.crypto.randomUUID();
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
      return v.toString(16);
    });
  }

  document.addEventListener('DOMContentLoaded', async () => {
    // Mode switcher
    document.querySelectorAll('.mode-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('.mode-btn').forEach((b) => b.classList.remove('active'));
        document.querySelectorAll('.mode-pane').forEach((p) => p.classList.remove('active'));
        btn.classList.add('active');
        document.getElementById('mode-' + btn.dataset.mode).classList.add('active');
      });
    });

    // Login (sets cookie-free JWT in subsequent calls; we just demonstrate the flow)
    try {
      const r = await Force.login();
      $('who').textContent = 'Token expires ' + new Date(r.expires_at).toLocaleString('id-ID');
    } catch (e) {
      $('who').textContent = 'PWA (token required via dev login)';
    }

    // Initial placeholders
    $('beat_plan_id').value = uuid();
    $('store_id').value = uuid();
    $('collect_store_id').value = uuid();
    $('collect_invoice_id').value = uuid();
    $('inquiry_rep_id').value = uuid();
    $('settle_rep_id').value = uuid();

    // Default: -6.175392, 106.827153 (Monas, Jakarta Pusat)
    $('latitude').value = -6.175392;
    $('longitude').value = 106.827153;

    bindEvents();
    refreshOutbox();
    setInterval(refreshOutbox, 5000);
  });

  function bindEvents() {
    $('load-today').addEventListener('click', loadToday);
    $('gps-btn').addEventListener('click', acquireGPS);
    $('checkin-form').addEventListener('submit', onCheckIn);
    $('collect-form').addEventListener('submit', onCollect);
    $('inquiry-form').addEventListener('submit', onInquiry);
    $('settle-form').addEventListener('submit', onSettle);
  }

  async function loadToday() {
    setStatus($('checkin-status'), 'Memuat rute…', '');
    const repID = $('collect_store_id').value || uuid(); // no rep yet, fetch all
    try {
      const reps = await Force.listReps();
      if (reps.length === 0) {
        setStatus($('checkin-status'), 'Belum ada salesman. Buat lewat POST /v1/force/sales-reps.', 'err');
        return;
      }
      const repId = reps[0].id;
      const plans = await Force.todayBeatPlans(repId);
      $('beat_plan_id').value = plans[0] ? plans[0].id : uuid();
      $('inquiry_rep_id').value = repId;
      $('settle_rep_id').value = repId;
      const list = plans.map((p) => `<div class="list-item"><strong>${p.plan_number}</strong> — ${p.territory} (${p.plan_date}) — ${p.status}<br><small>rep: ${p.sales_rep_id}</small></div>`).join('');
      $('beat-list').innerHTML = list || '<em>Tidak ada beat plan hari ini untuk salesman pertama.</em>';
      setStatus($('checkin-status'), 'OK — Rep dipilih: ' + reps[0].name, 'ok');
    } catch (err) {
      setStatus($('checkin-status'), 'ERR: ' + err.message, 'err');
    }
  }

  function acquireGPS() {
    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition((pos) => {
        $('latitude').value = pos.coords.latitude.toFixed(6);
        $('longitude').value = pos.coords.longitude.toFixed(6);
      }, () => {
        // Mock fallback: Monas, Jakarta
        $('latitude').value = -6.175392;
        $('longitude').value = 106.827153;
      });
    } else {
      $('latitude').value = -6.175392;
      $('longitude').value = 106.827153;
    }
  }

  async function onCheckIn(ev) {
    ev.preventDefault();
    const body = {
      beat_plan_id: $('beat_plan_id').value,
      store_id: $('store_id').value,
      latitude: parseFloat($('latitude').value),
      longitude: parseFloat($('longitude').value),
    };
    // For demo, attribute the check-in to the first rep in the system.
    let repId;
    try {
      const reps = await Force.listReps();
      if (reps.length === 0) { setStatus($('checkin-status'), 'Tidak ada salesman', 'err'); return; }
      repId = reps[0].id;
    } catch (err) { setStatus($('checkin-status'), 'ERR: ' + err.message, 'err'); return; }

    setStatus($('checkin-status'), 'Mengirim check-in…', '');
    try {
      const out = await Force.checkIn(repId, body);
      const tag = out.geofence_verified ? 'ok' : 'warn';
      setStatus($('checkin-status'),
        `Check-in ${out.status}\nStore: ${out.store_name}\nJarak: ${out.distance_meters} m\nGeofence: ${out.geofence_verified ? 'PASS' : 'OUT OF RADIUS'}`,
        tag);
    } catch (err) {
      setStatus($('checkin-status'), 'ERR: ' + err.message, 'err');
    }
  }

  async function onCollect(ev) {
    ev.preventDefault();
    let repId;
    try {
      const reps = await Force.listReps();
      if (reps.length === 0) { setStatus($('collect-status'), 'Tidak ada salesman', 'err'); return; }
      repId = reps[0].id;
    } catch (err) { setStatus($('collect-status'), 'ERR: ' + err.message, 'err'); return; }

    const body = {
      visit_id: $('collect_visit_id').value || null,
      store_id: $('collect_store_id').value,
      fledger_invoice_id: $('collect_invoice_id').value,
      amount: parseInt($('collect_amount').value, 10),
      payer_name: $('collect_payer').value,
      payer_phone: $('collect_payer_phone').value,
    };
    setStatus($('collect-status'), 'Mencatat kas…', '');
    try {
      const out = await Force.collect(repId, body);
      const cash = out.sales_rep_current_cash_held;
      setStatus($('collect-status'),
        `Kwitansi: ${out.collection.receipt_number}\nNominal: ${fmtIDR(out.collection.amount)}\nSaldo kas salesman: ${fmtIDR(cash)}\nWA: ${out.wa_receipt_payload.message}`,
        'ok');
      refreshRepCash(repId);
    } catch (err) {
      setStatus($('collect-status'), 'ERR: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
    }
  }

  async function refreshRepCash(repId) {
    if (!repId) return;
    try {
      const rep = await Force.getRep(repId);
      $('rep-cash').innerHTML = `<div class="kv">
        <div><strong>Nama:</strong> <span>${rep.name}</span></div>
        <div><strong>Employee Code:</strong> <span>${rep.employee_code}</span></div>
        <div><strong>Saldo Kas Saat Ini:</strong> <span>${fmtIDR(rep.current_cash_held)}</span></div>
        <div><strong>Plafon Maks:</strong> <span>${fmtIDR(rep.max_cash_limit)}</span></div>
        <div><strong>Status:</strong> <span>${rep.status}</span></div>
        <div><strong>Wallet (Core):</strong> <span>${rep.fledger_wallet_account_id}</span></div>
      </div>`;
    } catch (err) {
      $('rep-cash').textContent = 'ERR: ' + err.message;
    }
  }

  async function onInquiry(ev) {
    ev.preventDefault();
    const repId = $('inquiry_rep_id').value;
    setStatus($('inquiry-result'), 'Mencari saldo salesman…', '');
    try {
      const out = await Force.inquiry(repId);
      setStatus($('inquiry-result'),
        `Salesman: ${out.sales_rep_name} (${out.employee_code})\nWajib setor: ${fmtIDR(out.total_cash_held)}\nKwitansi tertunda: ${out.collections_count}`,
        'ok');
      refreshRepCash(repId);
    } catch (err) {
      setStatus($('inquiry-result'), 'ERR: ' + err.message, 'err');
    }
  }

  async function onSettle(ev) {
    ev.preventDefault();
    const body = {
      sales_rep_id: $('settle_rep_id').value,
      physical_cash_received: parseInt($('settle_physical').value, 10),
      cashier_notes: $('settle_notes').value,
    };
    setStatus($('settle-status'), 'Memvalidasi setoran…', '');
    try {
      const out = await Force.settle(body);
      setStatus($('settle-status'),
        `Settlement: ${out.settlement.settlement_number}\nDiscrepancy: ${fmtIDR(out.discrepancy_amount)}\nSalesman baru: kas=${fmtIDR(out.sales_rep_new_cash_held)}, status=${out.sales_rep_status}\nOutbox: ${out.outbox_status}`,
        out.discrepancy_amount === 0 ? 'ok' : 'warn');
      refreshRepCash(body.sales_rep_id);
      refreshOutbox();
    } catch (err) {
      setStatus($('settle-status'), 'ERR: ' + err.message, 'err');
    }
  }

  function refreshOutbox() {
    Force.outboxCounts()
      .then((c) => {
        $('outbox-counts').innerHTML = `<div class="kv">
          <div><strong>Pending:</strong> <span>${c.pending}</span></div>
          <div><strong>Processing:</strong> <span>${c.processing}</span></div>
          <div><strong>Sent:</strong> <span>${c.sent}</span></div>
          <div><strong>Failed:</strong> <span>${c.failed}</span></div>
        </div>`;
      })
      .catch((err) => { $('outbox-counts').textContent = 'ERR: ' + err.message; });
  }
})();