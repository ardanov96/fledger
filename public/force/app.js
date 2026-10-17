// Fledger Force PWA — UI controller (Enhanced Launch UI Style).
(function () {
  'use strict';
  const $ = (id) => document.getElementById(id);
  const fmtIDR = (n) => 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(n) || 0);

  function setStatus(el, msg, kind) {
    if (!el) return;
    el.className = 'status ' + (kind || '');
    el.textContent = msg;
  }

  function uuid() {
    if (window.crypto && window.crypto.randomUUID) return window.crypto.randomUUID();
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
      return v.toString(16);
    });
  }

  let cachedStores = [];
  let cachedReps = [];
  let currentRep = null;
  let cachedPlans = [];

  document.addEventListener('DOMContentLoaded', async () => {
    // Dynamic Ecosystem Nav link resolver for Gateway and Cloudflare Tunnel
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

    // Mode switcher
    document.querySelectorAll('.mode-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        document.querySelectorAll('.mode-btn').forEach((b) => b.classList.remove('active'));
        document.querySelectorAll('.mode-pane').forEach((p) => p.classList.remove('active'));
        btn.classList.add('active');
        document.getElementById('mode-' + btn.dataset.mode).classList.add('active');
      });
    });

    try {
      const r = await Force.login('a0000000-0000-0000-0000-000000000001');
      $('who').textContent = 'Token aktif (berakhir ' + new Date(r.expires_at).toLocaleTimeString('id-ID') + ')';
    } catch (e) {
      $('who').textContent = 'Dev Mode (Local)';
    }

    // Default invoice ID
    if ($('collect_invoice_id')) $('collect_invoice_id').value = uuid();

    bindEvents();
    initPWA();
    await initRepAndStores();
    refreshOutbox();
    setInterval(refreshOutbox, 5000);
  });

  async function initRepAndStores() {
    try {
      // 1. Fetch or create sales reps
      let reps = await Force.listReps();
      if (!reps || reps.length === 0) {
        try {
          const newRep = await Force.createRep({
            employee_code: 'SLS-DEMO-001',
            name: 'Budi Santoso',
            phone: '081298765432',
            role: 'CANVASSER',
            max_cash_limit: 50000000,
          });
          reps = [newRep];
        } catch (e) {
          console.warn('createRep fallback error:', e);
        }
      }
      cachedReps = reps || [];
      if (cachedReps.length > 0) {
        currentRep = cachedReps[0];
        if ($('inquiry_rep_id')) $('inquiry_rep_id').value = currentRep.id;
        if ($('settle_rep_id')) $('settle_rep_id').value = currentRep.id;
        refreshRepCash(currentRep.id);
      }

      // 2. Fetch stores & populate dropdowns
      const stores = await Force.listStores();
      cachedStores = stores || [];
      populateStoreSelects(cachedStores);

      // 3. Load or auto-create beat plan for today
      if (currentRep) {
        await loadToday();
      }
    } catch (e) {
      console.warn('initRepAndStores error:', e);
    }
  }

  function populateStoreSelects(stores) {
    const storeSel = $('store_select');
    const collectSel = $('collect_store_select');
    if (!stores || stores.length === 0) {
      if (storeSel) storeSel.innerHTML = '<option value="">(Belum ada toko terdaftar)</option>';
      if (collectSel) collectSel.innerHTML = '<option value="">(Belum ada toko terdaftar)</option>';
      return;
    }

    const opts = stores.map((s, idx) => `
      <option value="${s.id}" data-idx="${idx}" data-lat="${s.latitude}" data-lng="${s.longitude}">
        ${s.store_code ? s.store_code + ' - ' : ''}${s.name} (${s.owner_name || 'Toko'})
      </option>
    `).join('');

    if (storeSel) {
      storeSel.innerHTML = opts;
      onStoreSelectChanged();
    }
    if (collectSel) {
      collectSel.innerHTML = opts;
      onCollectStoreSelectChanged();
    }
  }

  function onStoreSelectChanged() {
    const storeSel = $('store_select');
    if (!storeSel || !cachedStores.length) return;
    const store = cachedStores[storeSel.selectedIndex] || cachedStores[0];
    if (store) {
      if ($('store_id')) $('store_id').value = store.id;
      if ($('latitude')) $('latitude').value = Number(store.latitude).toFixed(6);
      if ($('longitude')) $('longitude').value = Number(store.longitude).toFixed(6);
    }
  }

  function onCollectStoreSelectChanged() {
    const collectSel = $('collect_store_select');
    if (!collectSel || !cachedStores.length) return;
    const store = cachedStores[collectSel.selectedIndex] || cachedStores[0];
    if (store) {
      if ($('collect_store_id')) $('collect_store_id').value = store.id;
      if ($('collect_payer')) $('collect_payer').value = store.owner_name || 'Pemilik Toko';
      if ($('collect_payer_phone')) $('collect_payer_phone').value = store.phone || '081298765432';
    }
  }

  function bindEvents() {
    if ($('load-today')) $('load-today').addEventListener('click', loadToday);
    if ($('gps-btn')) $('gps-btn').addEventListener('click', acquireGPS);
    if ($('btn-exact-store-gps')) $('btn-exact-store-gps').addEventListener('click', setExactStoreGPS);
    if ($('btn-far-gps')) $('btn-far-gps').addEventListener('click', setFarGPS);
    if ($('store_select')) $('store_select').addEventListener('change', onStoreSelectChanged);
    if ($('collect_store_select')) $('collect_store_select').addEventListener('change', onCollectStoreSelectChanged);
    if ($('beat_plan_select')) $('beat_plan_select').addEventListener('change', onBeatPlanSelectChanged);
    if ($('btn-new-inv')) $('btn-new-inv').addEventListener('click', () => {
      $('collect_invoice_id').value = uuid();
    });

    if ($('checkin-form')) $('checkin-form').addEventListener('submit', onCheckIn);
    if ($('collect-form')) $('collect-form').addEventListener('submit', onCollect);
    if ($('inquiry-form')) $('inquiry-form').addEventListener('submit', onInquiry);
    if ($('settle-form')) $('settle-form').addEventListener('submit', onSettle);

    // Quick cash chips
    document.querySelectorAll('.chip-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        const inp = $('collect_amount');
        if (inp && btn.dataset.amt) {
          inp.value = btn.dataset.amt;
        }
      });
    });
  }

  function setExactStoreGPS() {
    const storeSel = $('store_select');
    if (!storeSel || !cachedStores.length) return;
    const store = cachedStores[storeSel.selectedIndex] || cachedStores[0];
    if (store) {
      $('latitude').value = Number(store.latitude).toFixed(6);
      $('longitude').value = Number(store.longitude).toFixed(6);
      setStatus($('checkin-status'), `📍 Koordinat diset tepat di titik toko: ${store.name} (${store.latitude}, ${store.longitude}). Geofence 100% lolos!`, 'ok');
    }
  }

  function setFarGPS() {
    const storeSel = $('store_select');
    if (!storeSel || !cachedStores.length) return;
    const store = cachedStores[storeSel.selectedIndex] || cachedStores[0];
    if (store) {
      // Offset by approx ~1.5 km to test out-of-geofence guard
      $('latitude').value = (Number(store.latitude) + 0.015).toFixed(6);
      $('longitude').value = (Number(store.longitude) + 0.015).toFixed(6);
      setStatus($('checkin-status'), `⚠️ Posisi GPS diatur >1.5 km di luar radius toko untuk menguji deteksi audit geofence.`, 'err');
    }
  }

  function onBeatPlanSelectChanged() {
    const sel = $('beat_plan_select');
    if (sel && sel.value) {
      $('beat_plan_id').value = sel.value;
    }
  }

  function initPWA() {
    let deferredPrompt = null;
    window.addEventListener('beforeinstallprompt', (e) => {
      e.preventDefault();
      deferredPrompt = e;
      const banner = $('pwa-install-banner');
      if (banner) banner.style.display = 'flex';
    });

    const installBtn = $('pwa-install-btn');
    if (installBtn) {
      installBtn.addEventListener('click', async () => {
        if (deferredPrompt) {
          deferredPrompt.prompt();
          const choice = await deferredPrompt.userChoice;
          if (choice.outcome === 'accepted') {
            const banner = $('pwa-install-banner');
            if (banner) banner.style.display = 'none';
          }
          deferredPrompt = null;
        }
      });
    }

    function updateOnlineStatus() {
      const badge = $('offline-badge');
      if (badge) {
        if (navigator.onLine) {
          badge.classList.remove('is-offline');
        } else {
          badge.classList.add('is-offline');
        }
      }
    }
    window.addEventListener('online', updateOnlineStatus);
    window.addEventListener('offline', updateOnlineStatus);
    updateOnlineStatus();
  }

  async function loadToday() {
    setStatus($('checkin-status'), 'Memuat rute kunjungan salesman hari ini…', '');
    try {
      if (!currentRep) {
        const reps = await Force.listReps();
        if (reps && reps.length > 0) currentRep = reps[0];
      }
      if (!currentRep) {
        setStatus($('checkin-status'), 'Belum ada salesman terdaftar dalam sistem.', 'err');
        return;
      }
      const repId = currentRep.id;
      let plans = await Force.todayBeatPlans(repId);

      // Auto-create today beat plan if none exists
      if (!plans || plans.length === 0) {
        const todayStr = new Date().toISOString().slice(0, 10);
        const planCode = 'BP-' + todayStr.replace(/-/g, '') + '-01';
        try {
          const newPlan = await Force.createBeatPlan({
            plan_number: planCode,
            sales_rep_id: repId,
            plan_date: todayStr,
            territory: 'DKI Jakarta FMCG Route',
            target_stores_count: cachedStores.length || 5,
            notes: 'Rute Harian Kunjungan Toko FMCG',
          });
          plans = [newPlan];
        } catch (e) {
          console.warn('Auto create beat plan fallback:', e);
        }
      }

      cachedPlans = plans || [];
      const bpSelect = $('beat_plan_select');
      if (bpSelect && cachedPlans.length > 0) {
        bpSelect.innerHTML = cachedPlans.map((p) => `
          <option value="${p.id}">${p.plan_number} - ${p.territory} (${p.status})</option>
        `).join('');
        $('beat_plan_id').value = cachedPlans[0].id;
      } else if (cachedPlans.length > 0) {
        $('beat_plan_id').value = cachedPlans[0].id;
      }

      $('inquiry_rep_id').value = repId;
      $('settle_rep_id').value = repId;

      if (!cachedPlans.length) {
        $('beat-list').innerHTML = '<div style="padding:16px; color:#94a3b8;">Tidak ada beat plan aktif hari ini untuk salesman.</div>';
      } else {
        $('beat-list').innerHTML = cachedPlans.map((p) => `
          <div class="beat-item">
            <div>
              <strong style="color:#f8fafc; font-size:0.95rem;">${p.plan_number}</strong>
              <div style="color:#94a3b8; font-size:0.8rem; margin-top:2px;">
                Wilayah: <strong>${p.territory}</strong> • Tanggal: <strong>${p.plan_date}</strong>
              </div>
            </div>
            <span style="background:rgba(16,185,129,0.15); color:#34d399; border:1px solid rgba(16,185,129,0.3); font-size:0.75rem; font-weight:600; padding:4px 10px; border-radius:999px;">
              ${p.status}
            </span>
          </div>
        `).join('');
      }

      refreshRepCash(repId);
      setStatus($('checkin-status'), '✓ Data Salesman Dimuat: ' + currentRep.name + ' (' + currentRep.employee_code + ')', 'ok');
    } catch (err) {
      setStatus($('checkin-status'), 'Gagal memuat rute: ' + err.message, 'err');
    }
  }

  function acquireGPS() {
    if (navigator.geolocation) {
      navigator.geolocation.getCurrentPosition((pos) => {
        $('latitude').value = pos.coords.latitude.toFixed(6);
        $('longitude').value = pos.coords.longitude.toFixed(6);
        setStatus($('checkin-status'), `GPS Terkunci: ${pos.coords.latitude.toFixed(6)}, ${pos.coords.longitude.toFixed(6)}`, 'ok');
      }, () => {
        setExactStoreGPS();
      });
    } else {
      setExactStoreGPS();
    }
  }

  async function onCheckIn(ev) {
    ev.preventDefault();
    const latStr = $('latitude') ? String($('latitude').value).replace(',', '.') : '0';
    const lngStr = $('longitude') ? String($('longitude').value).replace(',', '.') : '0';
    const body = {
      beat_plan_id: $('beat_plan_id') ? $('beat_plan_id').value : '',
      store_id: $('store_id') ? $('store_id').value : '',
      latitude: parseFloat(latStr) || 0,
      longitude: parseFloat(lngStr) || 0,
    };

    if (!body.store_id && $('store_select')) {
      body.store_id = $('store_select').value;
    }

    let repId = (currentRep && currentRep.id);
    if (!repId) {
      try {
        const reps = await Force.listReps();
        if (reps.length === 0) { setStatus($('checkin-status'), 'Tidak ada data salesman.', 'err'); return; }
        currentRep = reps[0];
        repId = currentRep.id;
      } catch (err) { setStatus($('checkin-status'), 'Error: ' + err.message, 'err'); return; }
    }

    setStatus($('checkin-status'), 'Memverifikasi koordinat GPS check-in…', '');
    try {
      const out = await Force.checkIn(repId, body);
      const pass = out.geofence_verified;
      const tag = pass ? 'ok' : 'err';
      setStatus($('checkin-status'),
        `Check-in ${out.status}!\n` +
        `• Toko: ${out.store_name}\n` +
        `• Jarak dari Titik Toko: ${out.distance_meters} meter\n` +
        `• Status Geofence: ${pass ? '[VERIFIED] DALAM RADIUS (Lolos)' : '[ALERT] DI LUAR RADIUS (>100m)'}\n` +
        `• Visit ID: ${out.id} (Disalin ke Form Penagihan Kas)`,
        tag);

      // Auto-fill visit ID and sync store in collection form
      if ($('collect_visit_id')) $('collect_visit_id').value = out.id;
      if ($('collect_store_select')) {
        $('collect_store_select').value = body.store_id;
        onCollectStoreSelectChanged();
      }
    } catch (err) {
      setStatus($('checkin-status'), 'Check-in gagal: ' + err.message, 'err');
    }
  }

  async function onCollect(ev) {
    ev.preventDefault();
    let repId = (currentRep && currentRep.id);
    if (!repId) {
      try {
        const reps = await Force.listReps();
        if (reps.length === 0) { setStatus($('collect-status'), 'Tidak ada salesman terdaftar.', 'err'); return; }
        currentRep = reps[0];
        repId = currentRep.id;
      } catch (err) { setStatus($('collect-status'), 'Error: ' + err.message, 'err'); return; }
    }

    const body = {
      visit_id: $('collect_visit_id').value || null,
      store_id: $('collect_store_id').value,
      fledger_invoice_id: $('collect_invoice_id').value || uuid(),
      amount: parseInt($('collect_amount').value, 10),
      payer_name: $('collect_payer').value,
      payer_phone: $('collect_payer_phone').value,
    };
    setStatus($('collect-status'), 'Mencatat penerimaan kas & mentransfer liability ke dompet sales…', '');
    try {
      const out = await Force.collect(repId, body);
      const cash = out.sales_rep_current_cash_held;
      setStatus($('collect-status'),
        `✓ Kas Berhasil Diterima!\n` +
        `• No Kwitansi: ${out.collection.receipt_number}\n` +
        `• Nominal: ${fmtIDR(out.collection.amount)}\n` +
        `• Saldo Kas Salesman Saat Ini: ${fmtIDR(cash)}\n` +
        `• WhatsApp Kwitansi: ${out.wa_receipt_payload ? out.wa_receipt_payload.message : 'Terkirim'}`,
        'ok');
      refreshRepCash(repId);
      // Auto-refresh invoice ID for next payment
      if ($('collect_invoice_id')) $('collect_invoice_id').value = uuid();
    } catch (err) {
      const body = err.body || {};
      if (body.code === 'cash_limit_exceeded') {
        const d = body.detail || {};
        setStatus($('collect-status'),
          `⚠️ Batas Plafon Kas Tercapai!\n` +
          `• Nominal Ditagih: ${fmtIDR(d.attempted_amount)}\n` +
          `• Saldo Kas Dipegang: ${fmtIDR(d.current_cash_held)}\n` +
          `• Plafon Kas Maksimal: ${fmtIDR(d.max_cash_limit)}\n` +
          `Petunjuk: Salesman wajib menyetorkan kas fisik ke Kasir Gudang (Menu Meja Kasir Gudang) terlebih dahulu sebelum menerima kas baru.`,
          'err');
      } else {
        setStatus($('collect-status'), 'Gagal: ' + err.message + '\n' + JSON.stringify(body, null, 2), 'err');
      }
    }
  }

  async function refreshRepCash(repId) {
    if (!repId) return;
    try {
      const rep = await Force.getRep(repId);
      $('rep-cash').innerHTML = `
        <div class="kv-grid">
          <div class="kv-item">
            <div class="kv-label">Nama Salesman</div>
            <div class="kv-val">${rep.name}</div>
          </div>
          <div class="kv-item">
            <div class="kv-label">Employee Code</div>
            <div class="kv-val" style="font-family:'JetBrains Mono', monospace;">${rep.employee_code}</div>
          </div>
          <div class="kv-item">
            <div class="kv-label">Saldo Kas Dipegang</div>
            <div class="kv-val" style="color:#10b981;">${fmtIDR(rep.current_cash_held)}</div>
          </div>
          <div class="kv-item">
            <div class="kv-label">Plafon Kas Maksimal</div>
            <div class="kv-val" style="color:#38bdf8;">${fmtIDR(rep.max_cash_limit)}</div>
          </div>
          <div class="kv-item">
            <div class="kv-label">Status Operasional</div>
            <div class="kv-val" style="color:${rep.status === 'ACTIVE' ? '#10b981' : '#f59e0b'};">${rep.status}</div>
          </div>
          <div class="kv-item">
            <div class="kv-label">Wallet ID (Core Ledger)</div>
            <div class="kv-val" style="font-size:0.75rem; font-family:'JetBrains Mono', monospace; color:#94a3b8;">${rep.fledger_wallet_account_id}</div>
          </div>
        </div>
      `;
    } catch (err) {
      $('rep-cash').innerHTML = `<div style="color:#ef4444; padding:12px;">Gagal memuat kas: ${err.message}</div>`;
    }
  }

  async function onInquiry(ev) {
    ev.preventDefault();
    const repId = $('inquiry_rep_id').value;
    setStatus($('inquiry-result'), 'Memeriksa rekap penerimaan kas salesman…', '');
    try {
      const out = await Force.inquiry(repId);
      setStatus($('inquiry-result'),
        `Rekap Setoran Salesman:\n` +
        `• Salesman: ${out.sales_rep_name} (${out.employee_code})\n` +
        `• Total Kas Wajib Setor: ${fmtIDR(out.total_cash_held)}\n` +
        `• Jumlah Kwitansi Tertunda: ${out.collections_count} transaksi`,
        'ok');
      $('settle_rep_id').value = repId;
      $('settle_physical').value = out.total_cash_held;
      refreshRepCash(repId);
    } catch (err) {
      setStatus($('inquiry-result'), 'Gagal inquiry: ' + err.message, 'err');
    }
  }

  async function onSettle(ev) {
    ev.preventDefault();
    const body = {
      sales_rep_id: $('settle_rep_id').value,
      physical_cash_received: parseInt($('settle_physical').value, 10),
      cashier_notes: $('settle_notes').value,
    };
    setStatus($('settle-status'), 'Memproses validasi setoran & membandingkan dengan buku besar…', '');
    try {
      const out = await Force.settle(body);
      const isBalanced = out.discrepancy_amount === 0;
      setStatus($('settle-status'),
        `✓ DAILY SETTLEMENT SELESAI!\n` +
        `• No Settlement: ${out.settlement.settlement_number}\n` +
        `• Selisih Fisik vs Sistem: ${fmtIDR(out.discrepancy_amount)} ${isBalanced ? '(SEIMBANG/BALANCED)' : '(ADA SELISIH)'}\n` +
        `• Saldo Kas Salesman Baru: ${fmtIDR(out.sales_rep_new_cash_held)}\n` +
        `• Status Salesman: ${out.sales_rep_status} (Kunci Dibuka)\n` +
        `• Outbox Status: ${out.outbox_status}`,
        isBalanced ? 'ok' : 'err');
      refreshRepCash(body.sales_rep_id);
      refreshOutbox();
    } catch (err) {
      setStatus($('settle-status'), 'Settlement gagal: ' + err.message, 'err');
    }
  }

  function refreshOutbox() {
    Force.outboxCounts()
      .then((c) => {
        $('outbox-counts').innerHTML = `
          <div class="kpi-card pending"><div class="kpi-title">Pending</div><div class="kpi-value">${c.pending ?? 0}</div></div>
          <div class="kpi-card processing"><div class="kpi-title">Processing</div><div class="kpi-value">${c.processing ?? 0}</div></div>
          <div class="kpi-card sent"><div class="kpi-title">Sent</div><div class="kpi-value">${c.sent ?? 0}</div></div>
          <div class="kpi-card failed"><div class="kpi-title">Failed</div><div class="kpi-value">${c.failed ?? 0}</div></div>
        `;
      })
      .catch((err) => {
        $('outbox-counts').innerHTML = `<div style="color:#ef4444; padding:12px;">Gagal memuat outbox: ${err.message}</div>`;
      });
  }
})();