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

    try {
      const r = await Force.login();
      $('who').textContent = 'Token aktif (berakhir ' + new Date(r.expires_at).toLocaleTimeString('id-ID') + ')';
    } catch (e) {
      $('who').textContent = 'Dev Mode (Local)';
    }

    // Initial placeholders
    $('beat_plan_id').value = uuid();
    $('store_id').value = uuid();
    $('collect_store_id').value = uuid();
    $('collect_invoice_id').value = uuid();
    $('inquiry_rep_id').value = uuid();
    $('settle_rep_id').value = uuid();

    // Default: Monas, Jakarta Pusat
    $('latitude').value = -6.175392;
    $('longitude').value = 106.827153;

    bindEvents();
    initPWA();
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
      const reps = await Force.listReps();
      if (reps.length === 0) {
        setStatus($('checkin-status'), 'Belum ada salesman terdaftar dalam sistem.', 'err');
        return;
      }
      const repId = reps[0].id;
      const plans = await Force.todayBeatPlans(repId);
      $('beat_plan_id').value = plans[0] ? plans[0].id : uuid();
      $('inquiry_rep_id').value = repId;
      $('settle_rep_id').value = repId;

      if (!plans.length) {
        $('beat-list').innerHTML = '<div style="padding:16px; color:#94a3b8;">Tidak ada beat plan aktif hari ini untuk salesman pertama.</div>';
      } else {
        $('beat-list').innerHTML = plans.map((p) => `
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
      setStatus($('checkin-status'), '✓ Data Salesman Dimuat: ' + reps[0].name + ' (' + reps[0].employee_code + ')', 'ok');
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
        $('latitude').value = -6.175392;
        $('longitude').value = 106.827153;
        setStatus($('checkin-status'), 'Menggunakan koordinat demo: Monas, Jakarta Pusat', '');
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

    let repId;
    try {
      const reps = await Force.listReps();
      if (reps.length === 0) { setStatus($('checkin-status'), 'Tidak ada data salesman.', 'err'); return; }
      repId = reps[0].id;
    } catch (err) { setStatus($('checkin-status'), 'Error: ' + err.message, 'err'); return; }

    setStatus($('checkin-status'), 'Memverifikasi koordinat GPS check-in…', '');
    try {
      const out = await Force.checkIn(repId, body);
      const pass = out.geofence_verified;
      const tag = pass ? 'ok' : 'err';
      setStatus($('checkin-status'),
        `Check-in ${out.status}!\n` +
        `• Toko: ${out.store_name}\n` +
        `• Jarak dari Titik Toko: ${out.distance_meters} meter\n` +
        `• Status Geofence: ${pass ? '[VERIFIED] DALAM RADIUS (Lolos)' : '[ALERT] DI LUAR RADIUS (>100m)'}`,
        tag);
    } catch (err) {
      setStatus($('checkin-status'), 'Check-in gagal: ' + err.message, 'err');
    }
  }

  async function onCollect(ev) {
    ev.preventDefault();
    let repId;
    try {
      const reps = await Force.listReps();
      if (reps.length === 0) { setStatus($('collect-status'), 'Tidak ada salesman terdaftar.', 'err'); return; }
      repId = reps[0].id;
    } catch (err) { setStatus($('collect-status'), 'Error: ' + err.message, 'err'); return; }

    const body = {
      visit_id: $('collect_visit_id').value || null,
      store_id: $('collect_store_id').value,
      fledger_invoice_id: $('collect_invoice_id').value,
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
        `• WhatsApp Receipt: ${out.wa_receipt_payload ? out.wa_receipt_payload.message : 'Terkirim'}`,
        'ok');
      refreshRepCash(repId);
    } catch (err) {
      setStatus($('collect-status'), 'Gagal: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
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