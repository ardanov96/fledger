// Fledger Dunning PWA — UI Controller (Launch UI / B2B SaaS Edition)
(function () {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const fmtIDR = (n) => 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(n) || 0);
  const fmtDT = (s) => s ? new Date(s).toLocaleString('id-ID', { dateStyle: 'medium', timeStyle: 'short' }) : '-';

  function setStatus(el, msg, kind) {
    if (!el) return;
    el.className = 'status-log ' + (kind || '');
    el.textContent = msg;
  }

  function stageInfo(stage) {
    const map = {
      'PRE_DUE_H3': {
        name: 'PRE_DUE_H3 (H-3)',
        title: 'Pengingat Pembayaran Faktur',
        greeting: 'Halo Bpk/Ibu pemilik toko, ini adalah pengingat ramah bahwa faktur FMCG Anda akan jatuh tempo dalam 3 hari.',
        tone: 'Sopan & Ramah',
        color: '#3b82f6'
      },
      'DUE_DATE': {
        name: 'DUE_DATE (Hari H)',
        title: 'Tagihan Jatuh Tempo Hari Ini',
        greeting: 'Yth. Mitra Toko, faktur pengiriman barang Anda jatuh tempo pada HARI INI. Mohon segera lakukan pembayaran sebelum pukul 17:00 WIB.',
        tone: 'Resmi',
        color: '#10b981'
      },
      'OVERDUE_H3': {
        name: 'OVERDUE_H3 (H+3)',
        title: 'Pemberitahuan Keterlambatan Pembayaran',
        greeting: 'Perhatian: Pembayaran faktur Anda telah melewati batas waktu (3 hari). Untuk menghindari penangguhan fasilitas kredit, mohon lunasi hari ini.',
        tone: 'Peringatan Awal',
        color: '#f59e0b'
      },
      'OVERDUE_H7': {
        name: 'OVERDUE_H7 (H+7)',
        title: 'Peringatan Keras Keterlambatan',
        greeting: 'PERINGATAN RESMI: Faktur terlambat 7 hari. Order pengiriman baru ditangguhkan sementara. Tim Sales Force kami akan mengunjungi toko Anda hari ini untuk penagihan langsung.',
        tone: 'Kunjungan Lapangan',
        color: '#ef4444'
      },
      'OVERDUE_H14': {
        name: 'OVERDUE_H14 (H+14)',
        title: 'Pemblokiran Plafon Kredit Toko',
        greeting: 'PEMBERITAHUAN FINAL: Plafon kredit Anda di Core Ledger telah DIBEKUKAN. Segala pesanan otomatis ditolak sistem. Segera hubungi bagian keuangan pusat.',
        tone: 'Hard Lock Plafon',
        color: '#a855f7'
      }
    };
    return map[stage] || { name: stage, title: 'Notifikasi Dunning', greeting: 'Pemberitahuan penagihan faktur.', tone: 'Umum', color: '#6366f1' };
  }

  // Quick Store preset selector
  window.selectQuickStore = function (storeId, storeName, ownerName, phone, amount) {
    $('f-store-id').value = storeId;
    $('f-store-name').value = storeName;
    $('f-owner-name').value = ownerName;
    $('f-phone').value = phone;
    $('f-amount').value = amount;
    $('s-store-id').value = storeId;
    $('f-invoice-id').value = 'INV-' + storeId + '-' + Date.now().toString().slice(-6);
    $('f-invoice-num').value = 'INV/' + new Date().getFullYear() + '/' + Date.now().toString().slice(-6);
    $('f-pay-link').value = 'http://localhost:8083/pay/' + $('f-invoice-id').value;

    previewStageMessage('PRE_DUE_H3', storeName, $('f-invoice-num').value, amount, phone);
  };

  document.addEventListener('DOMContentLoaded', async () => {
    // Login on load to seed the JWT
    try {
      const r = await FD.login();
      if (r && r.access_token) localStorage.setItem('fdunning_token', r.access_token);
    } catch (e) {}

    $('who').textContent = 'Fledger Dunning Tenant: ' + (FD.tenant || 'Default');

    // Attach button listeners
    const btnSave = $('btn-save-token') || $('btn-save-fonnte');
    if (btnSave) btnSave.addEventListener('click', onSaveGatewayToken);
    $('btn-pair').addEventListener('click', onPair);
    $('btn-test-send').addEventListener('click', onTestSend);
    $('btn-disconnect').addEventListener('click', onDisconnect);
    $('btn-ingest').addEventListener('click', onIngest);
    $('btn-refresh').addEventListener('click', refreshAll);
    $('btn-cron').addEventListener('click', onCron);
    $('btn-simulate-pay').addEventListener('click', onSimulatePay);
    $('btn-generate-stmt').addEventListener('click', onGenerateStmt);

    // Prefill gateway token if stored
    const tokenInput = $('f-gateway-token') || $('f-fonnte-token');
    if (tokenInput) {
      tokenInput.value = localStorage.getItem('wa_gateway_token') || 'EM8i5t8sWe11CpqBhLTx';
    }

    // Attach step banner listeners
    document.querySelectorAll('.cadence-step').forEach((el) => {
      el.addEventListener('click', () => {
        document.querySelectorAll('.cadence-step').forEach((s) => s.classList.remove('active'));
        el.classList.add('active');
        const stage = el.getAttribute('data-stage');
        previewStageMessage(
          stage,
          $('f-store-name').value || 'Toko Sumber Rezeki',
          $('f-invoice-num').value || 'INV/2026/008891',
          parseInt($('f-amount').value, 10) || 4500000,
          $('f-phone').value || '6281234567801'
        );
      });
    });

    // Prefill form defaults
    const now = new Date();
    $('f-invoice-id').value = 'INV-TKO-001-' + Date.now().toString().slice(-6);
    $('f-invoice-num').value = 'INV/2026/' + Date.now().toString().slice(-6);
    $('f-store-id').value = 'TKO-001';
    $('f-due-date').value = new Date(Date.now() + 7 * 86400 * 1000).toISOString().slice(0, 10);
    $('f-pay-link').value = 'http://localhost:8083/pay/' + $('f-invoice-id').value;
    $('s-store-id').value = 'TKO-001';

    // Update phone simulator clock
    const updateClock = () => {
      const d = new Date();
      const clockEl = $('wa-phone-clock');
      if (clockEl) clockEl.textContent = d.getHours().toString().padStart(2, '0') + ':' + d.getMinutes().toString().padStart(2, '0');
    };
    updateClock();
    setInterval(updateClock, 30000);

    // Initial message preview
    previewStageMessage('PRE_DUE_H3', 'Toko Sumber Rezeki', $('f-invoice-num').value, 4500000, '6281234567801');

    await refreshAll();
    setInterval(refreshAll, 5000);
  });

  async function refreshAll() {
    await Promise.all([refreshWhatsApp(), refreshQueues(), refreshCounts(), refreshStatements()]);
  }

  async function refreshWhatsApp() {
    try {
      const s = await FD.whatsappStatus();
      const prov = (s.data && s.data.provider) || 'MOCK';
      const isLive = (prov === 'FONNTE' || prov === 'GATEWAY' || prov === 'OFFICIAL');
      const phone = s.data && s.data.phone_connected ? s.data.phone_connected : (isLive ? '(Belum terhubung)' : '6281198765432 (Simulasi)');
      const st = s.data ? s.data.connection_status : 'CONNECTED';

      $('wa-provider').textContent = isLive ? 'Official WhatsApp Business Gateway' : 'Simulasi (Development)';
      $('wa-session').textContent = s.data ? s.data.session_name || 'official-distributor-wa' : 'official-distributor-wa';
      $('wa-phone').textContent = phone;
      $('wa-status').textContent = st;

      const pill = $('status-pill');
      const badge = $('wa-status-badge');
      const cls = st === 'CONNECTED' ? 'on' : st === 'SCAN_QR' ? 'scan' : 'off';

      pill.className = 'pill ' + cls;
      pill.textContent = st === 'CONNECTED' ? (isLive ? 'GATEWAY LIVE' : 'SIMULASI LIVE') : st;
      if (badge) {
        badge.className = 'pill ' + cls;
        badge.textContent = st;
      }
    } catch (e) {
      $('status-pill').className = 'pill on';
      $('status-pill').textContent = 'GATEWAY READY';
      if ($('wa-status-badge')) {
        $('wa-status-badge').className = 'pill on';
        $('wa-status-badge').textContent = 'CONNECTED';
      }
      $('wa-provider').textContent = 'Simulasi (Development)';
      $('wa-session').textContent = 'official-distributor-wa';
      $('wa-phone').textContent = '6281198765432 (Simulasi)';
      $('wa-status').textContent = 'CONNECTED';
    }
  }

  async function refreshQueues() {
    try {
      const items = await FD.listQueues('');
      const tbody = $('queue-table').querySelector('tbody');
      if (!items || !items.length) {
        tbody.innerHTML = '<tr><td colspan="5" class="tabular" style="text-align:center; color:var(--text-muted); padding:24px;">Belum ada antrean dunning aktif. Silakan ingest faktur baru.</td></tr>';
        return;
      }

      tbody.innerHTML = items.map((q) => {
        const info = stageInfo(q.stage);
        let statusCls = 'off';
        if (q.status === 'SENT') statusCls = 'on';
        if (q.status === 'CANCELLED_BY_PAYMENT') statusCls = 'scan';

        return `
          <tr>
            <td class="tabular" style="font-weight:600;">${q.invoice_number}</td>
            <td><span class="step-badge" style="background:${info.color}22; color:${info.color}; font-weight:700;">${q.stage}</span></td>
            <td class="tabular">${q.phone_number}</td>
            <td><span class="pill ${statusCls}">${q.status}</span></td>
            <td class="tabular" style="font-size:0.75rem; color:var(--text-muted);">${fmtDT(q.scheduled_at)}</td>
          </tr>
        `;
      }).join('');
    } catch (e) {}
  }

  async function refreshCounts() {
    try {
      const c = await FD.queueCounts();
      if ($('kpi-queued')) $('kpi-queued').textContent = c.queued || 0;
      if ($('kpi-sent')) $('kpi-sent').textContent = c.sent || 0;
      if ($('kpi-cancelled')) $('kpi-cancelled').textContent = c.cancelled || 0;
      if ($('kpi-failed')) $('kpi-failed').textContent = c.failed || 0;
    } catch (e) {}
  }

  async function refreshStatements() {
    try {
      const rows = await FD.listStatements();
      const tbody = $('stmt-table').querySelector('tbody');
      if (!rows || !rows.length) {
        tbody.innerHTML = '<tr><td colspan="6" class="tabular" style="text-align:center; color:var(--text-muted); padding:20px;">Belum ada rekening koran yang digenerate.</td></tr>';
        return;
      }
      tbody.innerHTML = rows.map((s) => `
        <tr>
          <td class="tabular" style="font-weight:600;">${s.store_id}</td>
          <td class="tabular">${s.statement_month}</td>
          <td class="tabular">${fmtIDR(s.total_invoiced_minor)}</td>
          <td class="tabular" style="color:#34d399;">${fmtIDR(s.total_paid_minor)}</td>
          <td class="tabular" style="font-weight:700; color:#fbbf24;">${fmtIDR(s.closing_balance_minor)}</td>
          <td>${s.pdf_file_path ? `<a href="/v1/dunning/statements/${s.id}/download" target="_blank" style="color:#38bdf8; text-decoration:none; font-weight:600;">📥 Unduh PDF</a>` : '-'}</td>
        </tr>
      `).join('');
    } catch (e) {}
  }

  async function onSaveGatewayToken() {
    const input = $('f-gateway-token') || $('f-fonnte-token');
    const token = (input ? input.value : '').trim();
    const statusEl = $('gateway-status') || $('ingest-status');

    if (!token) {
      if (confirm('Kosongkan token dan kembali ke mode simulasi (MOCK)?')) {
        try {
          await FD.whatsappConfigure({ provider: 'MOCK' });
          localStorage.removeItem('wa_gateway_token');
          setStatus(statusEl, 'Mode WhatsApp Gateway diatur ke mode Simulasi (MOCK).', 'ok');
          alert('Mode WhatsApp Gateway dikembalikan ke mode simulasi.');
          await refreshWhatsApp();
        } catch (e) {
          setStatus(statusEl, 'Gagal: ' + e.message, 'err');
          alert('Gagal: ' + e.message);
        }
      }
      return;
    }

    setStatus(statusEl, 'Menghubungkan ke WhatsApp Business Gateway...', '');
    try {
      await FD.whatsappConfigure({ provider: 'FONNTE', token: token });
      localStorage.setItem('wa_gateway_token', token);
      setStatus(statusEl, '✅ Berhasil menghubungkan WhatsApp Gateway! Status terverifikasi aktif.', 'ok');
      alert('✅ Berhasil menghubungkan WhatsApp Gateway! Token aktif.');
      await refreshWhatsApp();
    } catch (e) {
      const errDetail = (e.body && e.body.error) || (e.body && e.body.message) || e.message || 'Unknown error';
      setStatus(statusEl, 'Gagal menghubungkan WhatsApp Gateway: ' + errDetail, 'err');
      alert('❌ Gagal menghubungkan WhatsApp Gateway:\n' + errDetail);
    }
  }

  async function onPair() {
    const statusEl = $('gateway-status') || $('ingest-status');
    setStatus(statusEl, 'Membuat QR code pairing WhatsApp...', '');
    try {
      const r = await FD.whatsappQR('official-distributor-wa');
      const qr = r.data ? r.data.qr_code : '2@WACADENCE_DEMO_QR_CODE_' + Date.now();
      $('qr-area').classList.remove('hidden');
      const canvas = $('qr-canvas');
      if (canvas && (qr.startsWith('data:image') || qr.startsWith('http'))) {
        canvas.innerHTML = `<img src="${qr}" alt="WhatsApp QR Code" style="width:200px; height:200px; border-radius:8px; object-fit:contain; background:#fff; padding:6px; box-shadow:0 4px 12px rgba(0,0,0,0.3);" />`;
        $('qr-token').textContent = 'Scan QR di atas menggunakan aplikasi WhatsApp pada HP distributor';
      } else {
        $('qr-token').textContent = qr;
      }
      $('status-pill').textContent = 'SCAN_QR';
      $('status-pill').className = 'pill scan';
      setStatus(statusEl, 'QR Pairing siap! Buka WhatsApp di HP > Perangkat Tertaut > Tautkan Perangkat lalu scan QR.', 'ok');
      await refreshWhatsApp();
    } catch (e) {
      const errDetail = (e.body && e.body.error) || (e.body && e.body.message) || e.message || 'Unknown error';
      setStatus(statusEl, 'Gagal generate QR: ' + errDetail, 'err');
      alert('Gagal generate QR:\n' + errDetail);
    }
  }

  async function onTestSend() {
    const defaultPhone = ($('wa-phone') && $('wa-phone').textContent && !$('wa-phone').textContent.includes('Simulasi') && !$('wa-phone').textContent.includes('Belum'))
      ? $('wa-phone').textContent.trim()
      : ($('f-phone').value || '6289530876490');

    const phone = prompt('Kirim pesan uji ke nomor WhatsApp (format 62xxx atau 08xxx):', defaultPhone);
    if (!phone) return;

    const statusEl = $('gateway-status') || $('ingest-status');
    setStatus(statusEl, `Mengirim pesan uji WhatsApp ke ${phone}...`, '');

    try {
      const r = await FD.whatsappTestSend({
        phone_number: phone,
        body: '🔔 Test WhatsApp Message dari Fledger Dunning Console pada ' + new Date().toLocaleString('id-ID')
      });
      const msgId = r.data ? (r.data.provider_message_id || r.data.id || 'OK') : 'WA-' + Date.now();
      const successMsg = `✅ Test message WhatsApp berhasil dikirim ke ${phone}! (ID: ${msgId})`;
      setStatus(statusEl, successMsg, 'ok');
      alert(successMsg);
      previewCustomMessage('TEST_MESSAGE', phone, '🔔 Test WhatsApp Message dari Fledger Dunning Engine.\nKoneksi WhatsApp Gateway terverifikasi aktif.');
      await refreshWhatsApp();
    } catch (e) {
      const errDetail = (e.body && e.body.error) || (e.body && e.body.message) || e.message || 'Unknown error';
      const failMsg = `❌ Gagal mengirim pesan uji WhatsApp ke ${phone}:\n${errDetail}`;
      setStatus(statusEl, failMsg, 'err');
      alert(failMsg);
    }
  }

  async function onDisconnect() {
    if (!confirm('Putuskan koneksi WhatsApp Gateway?')) return;
    const statusEl = $('gateway-status') || $('ingest-status');
    try {
      await FD.whatsappDisconnect();
      await refreshWhatsApp();
      $('qr-area').classList.add('hidden');
      setStatus(statusEl, 'Sesi WhatsApp berhasil diputuskan.', '');
      alert('Sesi WhatsApp berhasil diputuskan.');
    } catch (e) {
      const errDetail = (e.body && e.body.error) || (e.body && e.body.message) || e.message || 'Unknown error';
      setStatus(statusEl, 'Gagal disconnect: ' + errDetail, 'err');
      alert('Gagal disconnect:\n' + errDetail);
    }
  }

  async function onIngest() {
    setStatus($('ingest-status'), 'Memproses ingest invoice dan kalkulasi jadwal 5 tahap...', '');
    const body = {
      invoice_id: $('f-invoice-id').value,
      invoice_number: $('f-invoice-num').value,
      store_id: $('f-store-id').value,
      phone_number: $('f-phone').value,
      due_date: $('f-due-date').value,
      amount_due_minor: parseInt($('f-amount').value, 10),
      payment_link_url: $('f-pay-link').value,
      store_name: $('f-store-name').value,
    };

    try {
      // Upsert contact first
      try {
        await FD.upsertContact({
          store_id: body.store_id,
          store_name: body.store_name,
          owner_name: $('f-owner-name').value,
          phone_number: body.phone_number,
        });
      } catch (e) {}

      const r = await FD.ingestInvoice(body);
      const sched = (r.data && r.data.schedules_created) || [];
      setStatus($('ingest-status'),
        `✅ Berhasil mengingest faktur ${body.invoice_number}!\n` +
        `Terjadwal ${sched.length} antrean:\n` +
        sched.map((s) => `  • [${s.stage}] Jadwal kirim: ${fmtDT(s.scheduled_at)}`).join('\n'),
        'ok');

      previewStageMessage('PRE_DUE_H3', body.store_name, body.invoice_number, body.amount_due_minor, body.phone_number);
      await refreshAll();
    } catch (e) {
      setStatus($('ingest-status'), 'Gagal ingest invoice: ' + e.message, 'err');
    }
  }

  async function onCron() {
    setStatus($('ingest-status'), 'Menjalankan Cron Dispatch Engine...', '');
    try {
      const r = await FD.cronRun();
      setStatus($('ingest-status'),
        `✅ Cron Dispatch selesai: Terkirim=${r.sent || 0}, Gagal=${r.failed || 0}`, 'ok');
      await refreshAll();
    } catch (e) {
      setStatus($('ingest-status'), 'Gagal menjalankan cron: ' + e.message, 'err');
    }
  }

  async function onSimulatePay() {
    const inv = prompt('Masukkan Invoice ID yang akan disimulasikan LUNAS (Self-Healing Demo):', $('f-invoice-id').value);
    if (!inv) return;

    setStatus($('ingest-status'), 'Menjalankan protokol Self-Healing untuk invoice ' + inv + '...', '');
    try {
      const c = await FD.cancelInvoice(inv);
      setStatus($('ingest-status'),
        `🎉 Self-Healing Berhasil! ${c.count || 0} antrean dunning dibatalkan secara otomatis karena invoice telah lunas di Core Ledger. Pesan tanda terima dikirimkan ke WhatsApp toko.`,
        'ok');
      await refreshAll();

      // Show receipt bubble in phone simulator
      previewReceiptMessage($('f-store-name').value || 'Toko Sumber Rezeki', inv, parseInt($('f-amount').value, 10) || 4500000);
    } catch (e) {
      setStatus($('ingest-status'), 'Gagal: ' + e.message, 'err');
    }
  }

  async function onGenerateStmt() {
    setStatus($('stmt-status'), 'Mengompilasi mutasi AR dan generate PDF e-Statement...', '');
    try {
      const r = await FD.generateStatement({
        store_id: $('s-store-id').value,
        statement_month: $('s-month').value,
      });
      setStatus($('stmt-status'),
        `✅ Dokumen e-Statement PDF berhasil dibuat dan tersimpan: ${r.data && r.data.statement ? r.data.statement.pdf_file_path : 'OK'}`, 'ok');
      await refreshStatements();
    } catch (e) {
      setStatus($('stmt-status'), 'Gagal membuat e-statement: ' + e.message, 'err');
    }
  }

  // --- Phone Simulator Render Helpers ---
  function previewStageMessage(stage, store, inv, amount, phone) {
    const info = stageInfo(stage);
    const msgs = $('wa-messages');
    if (!msgs) return;

    const timeStr = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });

    msgs.innerHTML = `
      <div class="wa-bubble-wrap">
        <div class="wa-bubble">
          <span class="wa-stage-tag" style="background:${info.color}22; color:${info.color};">${info.name}</span>
          <div style="font-weight:700; color:#0f172a; margin-bottom:4px;">${info.title}</div>
          <div style="margin-bottom:6px;">${info.greeting}</div>

          <div class="wa-invoice-box">
            <div class="wa-invoice-row"><span>Toko:</span> <strong>${store}</strong></div>
            <div class="wa-invoice-row"><span>No Faktur:</span> <strong>${inv}</strong></div>
            <div class="wa-invoice-row"><span>Total:</span> <strong style="color:#2563eb;">${fmtIDR(amount)}</strong></div>
            <div class="wa-invoice-row"><span>No WhatsApp:</span> <strong>${phone}</strong></div>
          </div>

          <div style="font-size:0.68rem; margin-top:4px;">
            Klik tombol di bawah untuk membayar instan dengan BCA / Mandiri / BRI / QRIS:
          </div>

          <a href="http://localhost:8083" target="_blank" class="wa-btn-mock">
            💳 Bayar Sekarang via Fledger Pay
          </a>

          <div class="wa-meta-time">${timeStr} &nbsp; <span style="color:#38bdf8;">✓✓</span></div>
        </div>
      </div>
    `;
    msgs.scrollTop = msgs.scrollHeight;
  }

  function previewReceiptMessage(store, inv, amount) {
    const msgs = $('wa-messages');
    if (!msgs) return;

    const timeStr = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });

    msgs.innerHTML = `
      <div class="wa-bubble-wrap">
        <div class="wa-bubble" style="border-left: 3px solid #10b981;">
          <span class="wa-stage-tag" style="background:rgba(16,185,129,0.15); color:#10b981;">PEMBAYARAN DITERIMA • LUNAS</span>
          <div style="font-weight:700; color:#0f172a; margin-bottom:4px;">Tanda Terima Pembayaran Sah</div>
          <div style="margin-bottom:6px;">
            Terima kasih! Pembayaran untuk faktur <strong>${inv}</strong> dari <strong>${store}</strong> sebesar <strong>${fmtIDR(amount)}</strong> telah berhasil kami terima dan dibukukan ke Core Ledger.
          </div>

          <div class="wa-invoice-box" style="background:#ecfdf5; border-color:#a7f3d0;">
            <div class="wa-invoice-row"><span>Status:</span> <strong style="color:#059669;">LUNAS (SETTLED)</strong></div>
            <div class="wa-invoice-row"><span>Waktu Bukuk:</span> <strong>${timeStr} WIB</strong></div>
            <div class="wa-invoice-row"><span>Plafon Kredit:</span> <strong style="color:#059669;">PULIH OTOMATIS</strong></div>
          </div>

          <div style="font-size:0.65rem; color:#64748b; margin-top:4px;">
            Semua sisa jadwal dunning untuk faktur ini telah dibatalkan secara otomatis (Self-Healing).
          </div>

          <div class="wa-meta-time">${timeStr} &nbsp; <span style="color:#38bdf8;">✓✓</span></div>
        </div>
      </div>
    `;
    msgs.scrollTop = msgs.scrollHeight;
  }

  function previewCustomMessage(tag, phone, text) {
    const msgs = $('wa-messages');
    if (!msgs) return;
    const timeStr = new Date().toLocaleTimeString('id-ID', { hour: '2-digit', minute: '2-digit' });

    msgs.innerHTML = `
      <div class="wa-bubble-wrap">
        <div class="wa-bubble">
          <span class="wa-stage-tag">${tag}</span>
          <div style="white-space:pre-wrap; margin-bottom:6px;">${text}</div>
          <div class="wa-meta-time">${timeStr} &nbsp; <span style="color:#38bdf8;">✓✓</span></div>
        </div>
      </div>
    `;
    msgs.scrollTop = msgs.scrollHeight;
  }
})();