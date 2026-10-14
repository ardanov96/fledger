// Fledger Pay Simulator — UI controller (Enhanced Launch UI Style).
(function () {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const fmtIDR = (n) => 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(n) || 0);
  const fmtUTC = (s) => s ? new Date(s).toLocaleString('id-ID') : '-';

  document.addEventListener('DOMContentLoaded', async function () {
    resetSampleData();
    $('base-url').textContent = PayClient.baseURL;
    $('create-form').addEventListener('submit', onCreate);
    $('settle-form').addEventListener('submit', onSettle);

    // Seed fresh dev token on load so simulator functions immediately without 401
    try {
      await PayClient.login();
      console.log('Dev auto-login successful');
    } catch (e) {
      console.warn('Dev auto-login skipped/failed:', e);
    }

    refreshOutbox();
    setInterval(refreshOutbox, 5000);
  });

  function resetSampleData() {
    $('fledger_invoice_id').value = randomUUID();
    $('customer_id').value = randomUUID();
  }

  function setStatus(el, msg, kind) {
    if (!el) return;
    el.className = 'status ' + (kind || '');
    el.textContent = msg;
  }

  async function onCreate(ev) {
    ev.preventDefault();
    if (!PayClient.token) {
      try { await PayClient.login(); } catch (e) {}
    }
    const banks = Array.from($('enabled_banks').selectedOptions).map((o) => o.value);
    const body = {
      fledger_invoice_id: $('fledger_invoice_id').value.trim(),
      customer_id: $('customer_id').value.trim(),
      customer_name: $('customer_name').value.trim(),
      customer_phone: $('customer_phone').value.trim(),
      amount: parseInt($('amount').value, 10),
      expiry_minutes: parseInt($('expiry_minutes').value, 10) || 1440,
      enabled_banks: banks,
      enable_qris: $('enable_qris').checked,
    };
    setStatus($('create-status'), 'Membuat permintaan pembayaran B2B…', '');
    try {
      const out = await PayClient.createRequest(body);
      setStatus($('create-status'), '✓ Berhasil Dibuat: Nomor ' + out.request.request_number, 'ok');
      renderDetail(out);
      refreshOutbox();
      $('detail-card').scrollIntoView({ behavior: 'smooth' });
    } catch (err) {
      setStatus($('create-status'), 'Gagal: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
    }
  }

  function renderDetail(out) {
    $('detail-card').hidden = false;
    $('d_request_number').textContent = out.request.request_number;
    $('d_status').textContent = out.request.status;
    $('d_amount').textContent = fmtIDR(out.request.amount);
    $('d_expires').textContent = fmtUTC(out.request.expires_at);

    // Render Virtual Account Cards
    const vaContainer = $('va-cards-container');
    vaContainer.innerHTML = '';
    (out.virtual_accounts || []).forEach((va) => {
      const card = document.createElement('div');
      card.className = 'va-card';
      card.innerHTML = `
        <div>
          <div class="va-card-header">
            <span class="bank-badge ${va.bank_code}">${va.bank_code}</span>
            <span style="font-size:0.75rem; color:#94a3b8;">Virtual Account</span>
          </div>
          <div class="va-number-box">
            <span class="va-number">${va.va_number}</span>
            <button type="button" class="copy-btn" data-copy="${va.va_number}">Salin</button>
          </div>
        </div>
        <div class="va-customer">Nama: <strong>${va.va_name}</strong></div>
      `;
      vaContainer.appendChild(card);
    });

    vaContainer.querySelectorAll('.copy-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        navigator.clipboard.writeText(btn.dataset.copy);
        btn.textContent = 'Tersalin ✓';
        btn.style.color = '#34d399';
        setTimeout(() => { btn.textContent = 'Salin'; btn.style.color = ''; }, 1500);
      });
    });

    // QRIS
    const qrisWrap = $('qris-wrapper');
    if (out.qris && out.qris.qr_string) {
      qrisWrap.hidden = false;
      $('qris-store-display').textContent = out.request.customer_name || 'Toko Sumber Rezeki';
      $('qris-amount-display').textContent = fmtIDR(out.request.amount);
      $('qr-string').textContent = out.qris.qr_string;
      renderQRISCode(out.qris.qr_string);
    } else {
      qrisWrap.hidden = true;
    }

    // Update settle amount default
    $('settle_amount').value = out.request.amount;
    window._lastRequest = out;
  }

  // Generate real, 100% standards-compliant scannable EMVCo QRIS code
  let qrisCodeInstance = null;
  function renderQRISCode(qrString) {
    const container = $('qris-qrcode');
    if (!container) return;
    container.innerHTML = '';

    if (typeof QRCode !== 'undefined') {
      try {
        qrisCodeInstance = new QRCode(container, {
          text: qrString,
          width: 200,
          height: 200,
          colorDark: '#0f172a',
          colorLight: '#ffffff',
          correctLevel: QRCode.CorrectLevel.M
        });
        return;
      } catch (err) {
        console.error('Failed to generate real QR Code via QRCode:', err);
      }
    }

    container.innerHTML = '<div style="color:#ef4444; font-size:0.8rem; padding:12px;">Gagal menginisialisasi QR Code Generator</div>';
  }

  async function onSettle(ev) {
    ev.preventDefault();
    if (!window._lastRequest) {
      setStatus($('settle-status'), 'Belum ada payment request aktif untuk diselesaikan.', 'err');
      return;
    }
    const body = {
      request_number: window._lastRequest.request.request_number,
      channel: $('settle_channel').value,
      amount: parseInt($('settle_amount').value, 10),
      payer_name: $('settle_payer').value.trim(),
    };
    setStatus($('settle-status'), 'Mengirimkan sinyal pelunasan pembayaran & webhook…', '');
    try {
      const out = await PayClient.simulateSettle(body);
      const msg = `✓ PEMBAYARAN BERHASIL DISELESAIKAN (SETTLED)!\n` +
        `• Transaction ID: ${out.transaction_id}\n` +
        `• Fledger Invoice ID: ${out.fledger_invoice_id}\n` +
        `• Status Outbox: ${out.outbox_status}\n` +
        `Sistem telah memicu auto-settlement ke Core Ledger dan auto-cancellation antrean Dunning.`;
      setStatus($('settle-status'), msg, 'ok');
      const updated = await PayClient.getRequest(window._lastRequest.request.id);
      renderDetail(updated);
      refreshOutbox();
    } catch (err) {
      setStatus($('settle-status'), 'Gagal: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
    }
  }

  function refreshOutbox() {
    PayClient.outboxCounts()
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

  function randomUUID() {
    if (window.crypto && window.crypto.randomUUID) {
      return window.crypto.randomUUID();
    }
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
      const r = Math.random() * 16 | 0;
      const v = c === 'x' ? r : (r & 0x3 | 0x8);
      return v.toString(16);
    });
  }
})();