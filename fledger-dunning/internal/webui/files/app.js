// Fledger Dunning PWA — UI controller.
(function () {
  'use strict';
  const $ = (id) => document.getElementById(id);
  const fmtIDR = (n) => 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(n) || 0);
  const fmtDT = (s) => s ? new Date(s).toLocaleString('id-ID') : '-';
  const uuid = () => (window.crypto && window.crypto.randomUUID) ? window.crypto.randomUUID()
    : 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
        const r = Math.random() * 16 | 0, v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
      });
  function setStatus(el, msg, kind) { el.className = 'status ' + (kind || ''); el.textContent = msg; }

  function stageLabel(stage) {
    return {
      'PRE_DUE_H3': 'PRE_DUE_H3 — Pengingat Sopan',
      'DUE_DATE':    'DUE_DATE — Penagihan Resmi',
      'OVERDUE_H3':  'OVERDUE_H3 — Keterlambatan',
      'OVERDUE_H7':  'OVERDUE_H7 — Peringatan Keras',
      'OVERDUE_H14': 'OVERDUE_H14 — Pemblokiran Kredit',
    }[stage] || stage;
  }

  document.addEventListener('DOMContentLoaded', async () => {
    // Login on load to seed the JWT (if dev login enabled).
    try { const r = await FD.login(); if (r && r.access_token) localStorage.setItem('fdunning_token', r.access_token); } catch (e) {}
    $('who').textContent = 'Tenant: ' + FD.tenant;

    $('btn-pair').addEventListener('click', onPair);
    $('btn-test-send').addEventListener('click', onTestSend);
    $('btn-disconnect').addEventListener('click', onDisconnect);
    $('btn-ingest').addEventListener('click', onIngest);
    $('btn-refresh').addEventListener('click', refreshAll);
    $('btn-cron').addEventListener('click', onCron);
    $('btn-simulate-pay').addEventListener('click', onSimulatePay);
    $('btn-generate-stmt').addEventListener('click', onGenerateStmt);

    // Prefill ingest form with sensible defaults.
    $('f-invoice-id').value = 'INV-DEMO-' + Date.now();
    $('f-invoice-num').value = 'INV/DEMO/' + Date.now();
    $('f-store-id').value = 'TKO-DEMO';
    $('f-due-date').value = new Date(Date.now() + 7 * 86400 * 1000).toISOString().slice(0, 10);
    $('s-store-id').value = 'TKO-DEMO';

    await refreshAll();
    setInterval(refreshAll, 5000);
  });

  async function refreshAll() {
    await Promise.all([refreshWhatsApp(), refreshQueues(), refreshCounts(), refreshStatements()]);
  }

  async function refreshWhatsApp() {
    try {
      const s = await FD.whatsappStatus();
      $('wa-provider').textContent = s.data ? s.data.session_name || 'default' : '-';
      $('wa-session').textContent = s.data ? s.data.session_name || '-' : '-';
      $('wa-phone').textContent = s.data && s.data.phone_connected ? s.data.phone_connected : '-';
      $('wa-status').textContent = s.data ? s.data.connection_status : 'UNKNOWN';
      const pill = $('status-pill');
      const st = s.data ? s.data.connection_status : 'DISCONNECTED';
      pill.className = 'pill ' + (st === 'CONNECTED' ? 'on' : st === 'SCAN_QR' ? 'scan' : 'off');
      pill.textContent = st;
    } catch (e) { /* offline */ }
  }

  async function refreshQueues() {
    try {
      const items = await FD.listQueues('');
      const tbody = $('queue-table').querySelector('tbody');
      if (!items || !items.length) {
        tbody.innerHTML = '<tr><td colspan="5" class="muted" style="text-align:center;color:#94a3b8;">Belum ada antrian.</td></tr>';
        return;
      }
      tbody.innerHTML = items.map((q) => `
        <tr>
          <td>${q.invoice_number}</td>
          <td>${stageLabel(q.stage)}</td>
          <td>${q.phone_number}</td>
          <td><span class="pill ${q.status === 'SENT' ? 'on' : q.status === 'CANCELLED_BY_PAYMENT' ? 'scan' : 'off'}">${q.status}</span></td>
          <td>${fmtDT(q.scheduled_at)}</td>
        </tr>
      `).join('');
    } catch (e) {}
  }

  async function refreshCounts() {
    try {
      const c = await FD.queueCounts();
      $('queue-counts').innerHTML = `
        <div><strong>Queued:</strong> <span>${c.queued}</span></div>
        <div><strong>Sent:</strong> <span>${c.sent}</span></div>
        <div><strong>Failed:</strong> <span>${c.failed}</span></div>
        <div><strong>Cancelled by Payment:</strong> <span>${c.cancelled}</span></div>
      `;
    } catch (e) {}
  }

  async function refreshStatements() {
    try {
      const rows = await FD.listStatements();
      const tbody = $('stmt-table').querySelector('tbody');
      if (!rows || !rows.length) {
        tbody.innerHTML = '<tr><td colspan="6" class="muted" style="text-align:center;color:#94a3b8;">Belum ada rekening koran.</td></tr>';
        return;
      }
      tbody.innerHTML = rows.map((s) => `
        <tr>
          <td>${s.store_id}</td>
          <td>${s.statement_month}</td>
          <td>${fmtIDR(s.total_invoiced_minor)}</td>
          <td>${fmtIDR(s.total_paid_minor)}</td>
          <td><strong>${fmtIDR(s.closing_balance_minor)}</strong></td>
          <td>${s.pdf_file_path || '-'}</td>
        </tr>
      `).join('');
    } catch (e) {}
  }

  async function onPair() {
    setStatus($('stmt-status'), 'Generating pairing QR...', '');
    try {
      const r = await FD.whatsappQR('official-distributor-wa');
      const qr = r.data ? r.data.qr_code : '';
      $('qr-area').classList.remove('hidden');
      $('qr-token').textContent = qr;
      $('status-pill').textContent = 'SCAN_QR';
      $('status-pill').className = 'pill scan';
      setStatus($('stmt-status'), 'QR pairing siap. Scan dari HP Anda.', 'ok');
      await refreshWhatsApp();
    } catch (e) {
      setStatus($('stmt-status'), 'Gagal: ' + e.message, 'err');
    }
  }

  async function onTestSend() {
    const phone = prompt('Kirim pesan uji ke nomor (format E.164, contoh 6281234567890):', '6281234567890');
    if (!phone) return;
    setStatus($('stmt-status'), 'Mengirim test message...', '');
    try {
      const r = await FD.whatsappTestSend({ phone_number: phone, body: '🔔 Test message dari Fledger Dunning PWA ' + new Date().toLocaleString('id-ID') });
      setStatus($('stmt-status'), 'Test message terkirim. Provider id: ' + (r.data ? r.data.provider_message_id : '-'), 'ok');
    } catch (e) {
      setStatus($('stmt-status'), 'Gagal: ' + e.message, 'err');
    }
  }

  async function onDisconnect() {
    if (!confirm('Putuskan koneksi WhatsApp?')) return;
    try {
      await FD.whatsappDisconnect();
      await refreshWhatsApp();
    } catch (e) {}
  }

  async function onIngest() {
    setStatus($('ingest-status'), 'Membuat 5 jadwal dunning...', '');
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
      // Also upsert the contact so the cancel/inquiry path has a phone.
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
        `Berhasil membuat ${sched.length} jadwal:\n` + sched.map((s) => `  • ${s.stage}  →  ${fmtDT(s.scheduled_at)}`).join('\n'),
        'ok');
      await refreshAll();
    } catch (e) {
      setStatus($('ingest-status'), 'Gagal: ' + e.message, 'err');
    }
  }

  async function onCron() {
    setStatus($('ingest-status'), 'Menjalankan cron dispatch...', '');
    try {
      const r = await FD.cronRun();
      setStatus($('ingest-status'),
        `Cron selesai: sent=${r.sent || 0} failed=${r.failed || 0}`, 'ok');
      await refreshAll();
    } catch (e) {
      setStatus($('ingest-status'), 'Gagal: ' + e.message, 'err');
    }
  }

  async function onSimulatePay() {
    const inv = prompt('Invoice ID yang akan dianggap LUNAS oleh toko (self-healing demo):', $('f-invoice-id').value);
    if (!inv) return;
    setStatus($('ingest-status'), 'Membatalkan semua antrian dunning untuk invoice ' + inv + '...', '');
    try {
      const c = await FD.cancelInvoice(inv);
      setStatus($('ingest-status'),
        `✅ Self-healing OK. ${c.count} antrian dibatalkan otomatis. Pesan WA tanda terima lunas sudah dikirim ke toko.`,
        'ok');
      await refreshAll();
      // Render a fake WA message in the phone simulator
      renderWAMessage('OVERDUE_H7', 'Toko Sumber Rezeki', inv, 4500000);
    } catch (e) {
      setStatus($('ingest-status'), 'Gagal: ' + e.message, 'err');
    }
  }

  async function onGenerateStmt() {
    setStatus($('stmt-status'), 'Rendering PDF e-Statement...', '');
    try {
      const r = await FD.generateStatement({
        store_id: $('s-store-id').value,
        statement_month: $('s-month').value,
      });
      setStatus($('stmt-status'),
        `✅ PDF tersimpan: ${r.data && r.data.statement ? r.data.statement.pdf_file_path : '-'}`, 'ok');
      await refreshStatements();
    } catch (e) {
      setStatus($('stmt-status'), 'Gagal: ' + e.message, 'err');
    }
  }

  // Render a synthetic WA message into the phone simulator.
  function renderWAMessage(stage, store, inv, amount) {
    const msgs = $('wa-messages');
    msgs.innerHTML = '';
    const stageLabelText = stageLabel(stage);
    const body = `🔔 *${stageLabelText}*\n\nYth. Pemilik *${store}*,\nFaktur *${inv}* senilai *${fmtIDR(amount)}* telah kami terima pembayaran-nya.\n\nTerima kasih! 🚀`;
    const div = document.createElement('div');
    div.innerHTML = `
      <div class="wa-stage">${stageLabelText}</div>
      <div class="wa-msg">${body.replace(/\n/g, '<br>')}</div>
    `;
    msgs.appendChild(div);
    msgs.scrollTop = msgs.scrollHeight;
  }
})();