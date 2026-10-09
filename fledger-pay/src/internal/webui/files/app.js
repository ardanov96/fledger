// Fledger Pay Simulator — UI controller.
(function () {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const fmtIDR = (n) => 'Rp ' + new Intl.NumberFormat('id-ID').format(Number(n) || 0);
  const fmtUTC = (s) => s ? new Date(s).toLocaleString('id-ID') : '-';

  // Initial tokens to populate the form.
  const SAMPLE_INVOICE = randomUUID();
  const SAMPLE_CUSTOMER = randomUUID();

  document.addEventListener('DOMContentLoaded', function () {
    $('fledger_invoice_id').value = SAMPLE_INVOICE;
    $('customer_id').value = SAMPLE_CUSTOMER;
    $('base-url').textContent = PayClient.baseURL;
    $('create-form').addEventListener('submit', onCreate);
    $('settle-form').addEventListener('submit', onSettle);
    refreshOutbox();
    setInterval(refreshOutbox, 5000);
  });

  function setStatus(el, msg, kind) {
    el.className = 'status ' + (kind || '');
    el.textContent = msg;
  }

  function onCreate(ev) {
    ev.preventDefault();
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
    setStatus($('create-status'), 'Membuat payment request…', '');
    PayClient.createRequest(body)
      .then((out) => {
        setStatus($('create-status'), 'Created: ' + out.request.request_number, 'ok');
        renderDetail(out);
      })
      .catch((err) => {
        setStatus($('create-status'), 'ERR: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
      });
  }

  function renderDetail(out) {
    $('detail-card').hidden = false;
    $('d_request_number').textContent = out.request.request_number;
    $('d_status').textContent = out.request.status;
    $('d_amount').textContent = fmtIDR(out.request.amount);
    $('d_expires').textContent = fmtUTC(out.request.expires_at);

    // VA table
    const tbody = $('va-table').querySelector('tbody');
    tbody.innerHTML = '';
    (out.virtual_accounts || []).forEach((va) => {
      const tr = document.createElement('tr');
      tr.innerHTML = `
        <td>${va.bank_code}</td>
        <td class="va-number">${va.va_number}</td>
        <td>${va.va_name}</td>
        <td><button class="copy-btn" data-copy="${va.va_number}">Salin</button></td>
      `;
      tbody.appendChild(tr);
    });
    tbody.querySelectorAll('.copy-btn').forEach((btn) => {
      btn.addEventListener('click', () => {
        navigator.clipboard.writeText(btn.dataset.copy);
        btn.textContent = 'Disalin ✓';
        setTimeout(() => { btn.textContent = 'Salin'; }, 1500);
      });
    });

    // QRIS
    $('qr-string').textContent = out.qris ? out.qris.qr_string : '(QRIS tidak diaktifkan)';

    // Update settle amount default
    $('settle_amount').value = out.request.amount;
    window._lastRequest = out;
  }

  function onSettle(ev) {
    ev.preventDefault();
    if (!window._lastRequest) {
      setStatus($('settle-status'), 'Belum ada payment request aktif', 'err');
      return;
    }
    const body = {
      request_number: window._lastRequest.request.request_number,
      channel: $('settle_channel').value,
      amount: parseInt($('settle_amount').value, 10),
      payer_name: $('settle_payer').value.trim(),
    };
    setStatus($('settle-status'), 'Mengirim settlement…', '');
    PayClient.simulateSettle(body)
      .then((out) => {
        const msg = 'SETTLED ✓\n' +
          'tx_id: ' + out.transaction_id + '\n' +
          'invoice: ' + out.fledger_invoice_id + '\n' +
          'outbox: ' + out.outbox_status;
        setStatus($('settle-status'), msg, 'ok');
        // Refresh detail to show new status
        PayClient.getRequest(window._lastRequest.request.id).then((d) => {
          renderDetail(d);
        });
        refreshOutbox();
      })
      .catch((err) => {
        setStatus($('settle-status'), 'ERR: ' + err.message + '\n' + JSON.stringify(err.body || {}, null, 2), 'err');
      });
  }

  function refreshOutbox() {
    PayClient.outboxCounts()
      .then((c) => {
        $('outbox-counts').innerHTML = `
          <div class="kv">
            <div><strong>Pending:</strong> <span>${c.pending}</span></div>
            <div><strong>Processing:</strong> <span>${c.processing}</span></div>
            <div><strong>Sent:</strong> <span>${c.sent}</span></div>
            <div><strong>Failed:</strong> <span>${c.failed}</span></div>
          </div>
        `;
      })
      .catch((err) => {
        $('outbox-counts').textContent = 'ERR: ' + err.message;
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