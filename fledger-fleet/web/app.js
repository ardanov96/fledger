/**
 * Fledger Fleet — Dispatcher & Driver Digital POD Application Logic
 * Integrates directly with Fledger Fleet REST API (:8082).
 * Includes Full Multi-Entity Pagination & Launch UI Standard Design.
 */

document.addEventListener('DOMContentLoaded', () => {
  const subpath = window.location.pathname.startsWith('/fleet') ? '/fleet' : '';
  const defaultBaseUrl = (window.location.origin && window.location.origin !== 'null')
    ? (window.location.origin + subpath)
    : 'http://localhost:8082';

  const client = new FledgerFleetClient(defaultBaseUrl);
  
  const state = {
    token: localStorage.getItem('fleet_jwt_token') || '',
    activeTab: 'driver-tab',
    activeSubtab: 'subtab-dos',
    vehicles: [],
    drivers: [],
    trips: [],
    dos: [],
    assignedTripDOs: [],
    todayTrip: null,
    selectedDO: null,
    hasSignature: false
  };

  // Pagination states for every tab and table
  const paginationState = {
    driverDO: { page: 1, pageSize: 5 },
    podItems: { page: 1, pageSize: 5 },
    trips: { page: 1, pageSize: 5 },
    masterDO: { page: 1, pageSize: 5 },
    masterVehicles: { page: 1, pageSize: 5 },
    masterDrivers: { page: 1, pageSize: 5 }
  };

  if (state.token) {
    client.setToken(state.token);
  }

  // DOM Elements - Global
  const healthDot = document.querySelector('#coreHealthIndicator .dot');
  const healthLabel = document.querySelector('#coreHealthIndicator .label');
  const btnQuickLogin = document.getElementById('btnQuickLogin');
  const toastEl = document.getElementById('toast');

  // DOM Elements - Driver Tab
  const activeTripBox = document.getElementById('activeTripBox');
  const driverDOList = document.getElementById('driverDOList');
  const btnRefreshDriver = document.getElementById('btnRefreshDriver');
  const noDOSelectedState = document.getElementById('noDOSelectedState');
  const podForm = document.getElementById('podForm');
  const podSuccessCard = document.getElementById('podSuccessCard');
  const selectedDOBadge = document.getElementById('selectedDOBadge');
  const podCustomerName = document.getElementById('podCustomerName');
  const podCustomerAddress = document.getElementById('podCustomerAddress');
  const podDONumber = document.getElementById('podDONumber');
  const podNominalOrdered = document.getElementById('podNominalOrdered');
  const podItemsTbody = document.getElementById('podItemsTbody');
  const podItemsCountBadge = document.getElementById('podItemsCountBadge');
  const podCleanNominal = document.getElementById('podCleanNominal');
  const photoSection = document.getElementById('photoSection');
  const podPhotoUrl = document.getElementById('podPhotoUrl');
  const btnFillSamplePhoto = document.getElementById('btnFillSamplePhoto');
  const photoPreviewBox = document.getElementById('photoPreviewBox');
  const photoPreviewImg = document.getElementById('photoPreviewImg');
  const signatureCanvas = document.getElementById('signatureCanvas');
  const btnClearSignature = document.getElementById('btnClearSignature');
  const podRecipientName = document.getElementById('podRecipientName');
  const podRecipientPhone = document.getElementById('podRecipientPhone');
  const podDriverNotes = document.getElementById('podDriverNotes');
  const btnSubmitPOD = document.getElementById('btnSubmitPOD');
  const btnBackToDOList = document.getElementById('btnBackToDOList');
  const resStatus = document.getElementById('resStatus');
  const resNominal = document.getElementById('resNominal');
  const resInvoiceId = document.getElementById('resInvoiceId');

  // Driver DO Pagination Elements
  const driverDOPageInfo = document.getElementById('driverDOPageInfo');
  const driverDOPageNav = document.getElementById('driverDOPageNav');

  // POD Items Pagination Elements
  const podItemsPageInfo = document.getElementById('podItemsPageInfo');
  const podItemsPageNav = document.getElementById('podItemsPageNav');

  // DOM Elements - Dispatcher Tab
  const tripForm = document.getElementById('tripForm');
  const tripNumberInput = document.getElementById('tripNumber');
  const tripVehicleSelect = document.getElementById('tripVehicleSelect');
  const tripDriverSelect = document.getElementById('tripDriverSelect');
  const tripNotesInput = document.getElementById('tripNotes');
  const tripDOCheckboxes = document.getElementById('tripDOCheckboxes');
  const estWeightKg = document.getElementById('estWeightKg');
  const maxCapacityKg = document.getElementById('maxCapacityKg');
  const weightProgressFill = document.getElementById('weightProgressFill');
  const capacityWarning = document.getElementById('capacityWarning');
  const tripsTbody = document.getElementById('tripsTbody');
  const btnRefreshTrips = document.getElementById('btnRefreshTrips');
  const outboxPending = document.getElementById('outboxPending');
  const outboxSent = document.getElementById('outboxSent');
  const outboxFailed = document.getElementById('outboxFailed');

  // Trips Pagination Elements
  const tripsPageInfo = document.getElementById('tripsPageInfo');
  const tripsPageNav = document.getElementById('tripsPageNav');

  // DOM Elements - Master Tab
  const vehicleForm = document.getElementById('vehicleForm');
  const driverForm = document.getElementById('driverForm');
  const btnGenerateDemoDO = document.getElementById('btnGenerateDemoDO');
  const masterDOTbody = document.getElementById('masterDOTbody');
  const masterVehiclesTbody = document.getElementById('masterVehiclesTbody');
  const masterDriversTbody = document.getElementById('masterDriversTbody');

  // Master DO Pagination Elements
  const masterDOPageInfo = document.getElementById('masterDOPageInfo');
  const masterDOPageNav = document.getElementById('masterDOPageNav');

  // Master Vehicles Pagination Elements
  const masterVehiclesPageInfo = document.getElementById('masterVehiclesPageInfo');
  const masterVehiclesPageNav = document.getElementById('masterVehiclesPageNav');

  // Master Drivers Pagination Elements
  const masterDriversPageInfo = document.getElementById('masterDriversPageInfo');
  const masterDriversPageNav = document.getElementById('masterDriversPageNav');

  // ----------------------------------------------------
  // Helpers
  // ----------------------------------------------------
  function showToast(message, type = 'info') {
    toastEl.textContent = message;
    toastEl.className = `toast show ${type}`;
    setTimeout(() => {
      toastEl.className = 'toast hidden';
    }, 3800);
  }

  function formatIDR(amount) {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      maximumFractionDigits: 0
    }).format(amount || 0);
  }

  function generateId(prefix = 'ID') {
    const d = new Date();
    const dateStr = d.toISOString().slice(2, 10).replace(/-/g, '');
    const rand = Math.floor(1000 + Math.random() * 9000);
    return `${prefix}-${dateStr}-${rand}`;
  }

  // ----------------------------------------------------
  // Generic Pagination Logic
  // ----------------------------------------------------
  function paginateItems(items, currentPage, pageSize) {
    const total = items.length;
    const totalPages = Math.max(1, Math.ceil(total / pageSize));
    const page = Math.min(Math.max(1, currentPage), totalPages);
    const startIdx = (page - 1) * pageSize;
    const endIdx = Math.min(startIdx + pageSize, total);
    const pageItems = items.slice(startIdx, endIdx);
    return {
      total,
      totalPages,
      page,
      startIdx,
      endIdx,
      pageItems
    };
  }

  function renderPagination(infoEl, navEl, paginatedData, unitName, onPageChange) {
    if (!infoEl || !navEl) return;
    const { total, totalPages, page, startIdx, endIdx } = paginatedData;

    if (total === 0) {
      infoEl.textContent = `Menampilkan 0–0 dari 0 ${unitName}`;
      navEl.innerHTML = '';
      return;
    }

    infoEl.textContent = `Menampilkan ${startIdx + 1}–${endIdx} dari ${total} ${unitName} (Hal. ${page}/${totalPages})`;

    let html = '';
    html += `<button type="button" class="page-btn" ${page <= 1 ? 'disabled' : ''} data-page="${page - 1}">« Prev</button>`;

    const maxButtons = 5;
    let startP = Math.max(1, page - Math.floor(maxButtons / 2));
    let endP = Math.min(totalPages, startP + maxButtons - 1);
    if (endP - startP + 1 < maxButtons) {
      startP = Math.max(1, endP - maxButtons + 1);
    }

    for (let p = startP; p <= endP; p++) {
      html += `<button type="button" class="page-btn ${p === page ? 'active' : ''}" data-page="${p}">${p}</button>`;
    }

    html += `<button type="button" class="page-btn" ${page >= totalPages ? 'disabled' : ''} data-page="${page + 1}">Next »</button>`;
    navEl.innerHTML = html;

    navEl.querySelectorAll('.page-btn:not(:disabled)').forEach(btn => {
      btn.addEventListener('click', (e) => {
        e.preventDefault();
        const targetPage = parseInt(btn.getAttribute('data-page'), 10);
        if (targetPage && targetPage !== page) {
          onPageChange(targetPage);
        }
      });
    });
  }

  // ----------------------------------------------------
  // Canvas Signature Pad
  // ----------------------------------------------------
  const ctx = signatureCanvas.getContext('2d');
  let isDrawing = false;

  function initCanvas() {
    ctx.lineWidth = 2.5;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.strokeStyle = '#0f172a';
    clearCanvas();
  }

  function clearCanvas() {
    ctx.clearRect(0, 0, signatureCanvas.width, signatureCanvas.height);
    state.hasSignature = false;
  }

  function getCanvasCoords(e) {
    const rect = signatureCanvas.getBoundingClientRect();
    const scaleX = signatureCanvas.width / rect.width;
    const scaleY = signatureCanvas.height / rect.height;

    let clientX, clientY;
    if (e.touches && e.touches.length > 0) {
      clientX = e.touches[0].clientX;
      clientY = e.touches[0].clientY;
    } else {
      clientX = e.clientX;
      clientY = e.clientY;
    }

    return {
      x: (clientX - rect.left) * scaleX,
      y: (clientY - rect.top) * scaleY
    };
  }

  function startDrawing(e) {
    if (e.type.startsWith('touch')) e.preventDefault();
    isDrawing = true;
    const { x, y } = getCanvasCoords(e);
    ctx.beginPath();
    ctx.moveTo(x, y);
    state.hasSignature = true;
  }

  function draw(e) {
    if (!isDrawing) return;
    if (e.type.startsWith('touch')) e.preventDefault();
    const { x, y } = getCanvasCoords(e);
    ctx.lineTo(x, y);
    ctx.stroke();
  }

  function stopDrawing(e) {
    if (e.type.startsWith('touch')) e.preventDefault();
    isDrawing = false;
  }

  signatureCanvas.addEventListener('mousedown', startDrawing);
  signatureCanvas.addEventListener('mousemove', draw);
  signatureCanvas.addEventListener('mouseup', stopDrawing);
  signatureCanvas.addEventListener('mouseleave', stopDrawing);

  signatureCanvas.addEventListener('touchstart', startDrawing, { passive: false });
  signatureCanvas.addEventListener('touchmove', draw, { passive: false });
  signatureCanvas.addEventListener('touchend', stopDrawing, { passive: false });
  signatureCanvas.addEventListener('touchcancel', stopDrawing, { passive: false });

  btnClearSignature.addEventListener('click', clearCanvas);

  // ----------------------------------------------------
  // Health & Authentication
  // ----------------------------------------------------
  async function checkHealth() {
    try {
      await client.health();
      healthDot.className = 'dot online';
      healthLabel.textContent = 'Fleet API :8082 Online';
    } catch (err) {
      healthDot.className = 'dot offline';
      healthLabel.textContent = 'Fleet API Offline';
    }
  }

  async function performLogin(silent = false) {
    try {
      const res = await client.devLogin();
      state.token = res.access_token;
      localStorage.setItem('fleet_jwt_token', res.access_token);
      if (!silent) {
        showToast('Dev Token berhasil dibuat & tersimpan!', 'success');
      }
      refreshAll();
    } catch (err) {
      if (!silent) {
        showToast(`Gagal login dev: ${err.message}`, 'error');
      }
    }
  }

  btnQuickLogin.addEventListener('click', () => performLogin(false));

  // ----------------------------------------------------
  // Navigation Tabs
  // ----------------------------------------------------
  document.querySelectorAll('.nav-tab, .mode-btn').forEach(tabBtn => {
    tabBtn.addEventListener('click', () => {
      document.querySelectorAll('.nav-tab, .mode-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.tab-pane').forEach(p => p.classList.remove('active'));

      tabBtn.classList.add('active');
      const targetId = tabBtn.getAttribute('data-tab');
      if (!targetId) return;
      const targetPane = document.getElementById(targetId);
      if (targetPane) targetPane.classList.add('active');
      state.activeTab = targetId;

      if (targetId === 'driver-tab') loadDriverView();
      if (targetId === 'dispatcher-tab') loadDispatcherView();
      if (targetId === 'master-tab') loadMasterView();
    });
  });

  document.querySelectorAll('.subtab-btn').forEach(subBtn => {
    subBtn.addEventListener('click', () => {
      document.querySelectorAll('.subtab-btn').forEach(b => b.classList.remove('active'));
      document.querySelectorAll('.subtab-content').forEach(c => c.classList.remove('active'));

      subBtn.classList.add('active');
      const targetSub = subBtn.getAttribute('data-subtab');
      document.getElementById(targetSub).classList.add('active');
      state.activeSubtab = targetSub;
    });
  });

  // ----------------------------------------------------
  // Tab 1: Driver Mode & Digital POD
  // ----------------------------------------------------
  async function loadDriverView() {
    activeTripBox.innerHTML = '<span class="badge badge-info">Memuat data trip pengiriman...</span>';
    driverDOList.innerHTML = '';

    try {
      // 1. Fetch today's trips
      let trips = [];
      try {
        const todayRes = await client.listTodayTrips();
        trips = Array.isArray(todayRes) ? todayRes : (todayRes.trips || []);
      } catch (e) {
        const allTripsRes = await client.listTrips();
        trips = Array.isArray(allTripsRes) ? allTripsRes : (allTripsRes.trips || []);
      }

      state.trips = trips;
      // Prioritize DISPATCHED or IN_TRANSIT, fallback to DRAFT or first trip
      state.todayTrip = trips.find(t => t.status === 'DISPATCHED' || t.status === 'IN_TRANSIT') || trips[0] || null;

      if (!state.todayTrip) {
        activeTripBox.innerHTML = `
          <div class="empty-state-sm">
            <span>Tidak ada trip berjalan hari ini.</span>
            <small>Buat dan berangkatkan trip di tab Dispatcher.</small>
          </div>
        `;
        driverDOList.innerHTML = `
          <div class="p-3 text-center text-muted">
            Belum ada Surat Jalan (DO) yang ditugaskan.
          </div>
        `;
        renderPagination(driverDOPageInfo, driverDOPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Surat Jalan', () => {});
        return;
      }

      // Render Active Trip info banner
      const trip = state.todayTrip;
      activeTripBox.innerHTML = `
        <div class="active-trip-card">
          <div class="flex-between">
            <strong>${trip.trip_number}</strong>
            <span class="badge badge-${trip.status === 'COMPLETED' ? 'success' : (trip.status === 'DISPATCHED' ? 'warning' : 'info')}">${trip.status}</span>
          </div>
          <div class="trip-meta mt-1">
            <span>🚛 Truk: <strong>${trip.vehicle_plate || trip.vehicle_id?.slice(0, 8) || '-'}</strong></span>
            <span>👤 Supir: <strong>${trip.driver_name || trip.driver_id?.slice(0, 8) || '-'}</strong></span>
          </div>
          <div class="trip-stops-count mt-1">
            <span>📦 Total Perhentian: <strong>${trip.stops ? trip.stops.length : (trip.do_ids ? trip.do_ids.length : 0)} Toko</strong></span>
          </div>
        </div>
      `;

      // 2. Fetch DO details
      const doRes = await client.listDeliveryOrders();
      const allDOs = Array.isArray(doRes) ? doRes : (doRes.delivery_orders || []);
      state.dos = allDOs;

      // Filter DOs belonging to this trip or show all confirmed
      let tripDOIds = [];
      if (trip.stops && trip.stops.length > 0) {
        tripDOIds = trip.stops.map(s => s.delivery_order_id || s.do_id);
      } else if (trip.do_ids && trip.do_ids.length > 0) {
        tripDOIds = trip.do_ids;
      }

      state.assignedTripDOs = tripDOIds.length > 0 
        ? allDOs.filter(d => tripDOIds.includes(d.id))
        : allDOs;

      renderDriverDOList();

    } catch (err) {
      activeTripBox.innerHTML = `<span class="badge badge-danger">Gagal memuat trip: ${err.message}</span>`;
    }
  }

  function renderDriverDOList() {
    const assignedDOs = state.assignedTripDOs || [];

    if (assignedDOs.length === 0) {
      driverDOList.innerHTML = `<div class="p-3 text-center text-muted">Tidak ada Surat Jalan dalam trip ini.</div>`;
      renderPagination(driverDOPageInfo, driverDOPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Surat Jalan', () => {});
      return;
    }

    const paginated = paginateItems(assignedDOs, paginationState.driverDO.page, paginationState.driverDO.pageSize);
    paginationState.driverDO.page = paginated.page;

    driverDOList.innerHTML = paginated.pageItems.map(d => {
      const isDelivered = d.status === 'DELIVERED';
      const badgeClass = isDelivered ? 'badge-success' : (d.status === 'IN_TRANSIT' ? 'badge-warning' : 'badge-info');
      const isSelected = state.selectedDO && state.selectedDO.id === d.id;
      return `
        <div class="do-item-card ${isSelected ? 'selected' : ''} ${isDelivered ? 'delivered' : ''}" data-do-id="${d.id}">
          <div class="flex-between">
            <span class="do-number"><strong>${d.do_number}</strong></span>
            <span class="badge ${badgeClass}">${d.status}</span>
          </div>
          <div class="customer-title mt-1">${d.customer_name || 'Pelanggan Toko'}</div>
          <div class="customer-address text-muted text-sm">${d.destination_address || '-'}</div>
          <div class="flex-between text-sm mt-2">
            <span>${(d.items || []).length} SKU Barang</span>
            <span><strong>${formatIDR(d.total_nominal || 0)}</strong></span>
          </div>
        </div>
      `;
    }).join('');

    // Attach click handlers
    driverDOList.querySelectorAll('.do-item-card').forEach(card => {
      card.addEventListener('click', () => {
        const doId = card.getAttribute('data-do-id');
        const found = state.dos.find(d => d.id === doId);
        if (found) selectDO(found);
      });
    });

    renderPagination(driverDOPageInfo, driverDOPageNav, paginated, 'Surat Jalan', (newPage) => {
      paginationState.driverDO.page = newPage;
      renderDriverDOList();
    });
  }

  async function selectDO(doItem) {
    state.selectedDO = doItem;

    // Highlight selected card in list
    document.querySelectorAll('.do-item-card').forEach(c => {
      c.classList.toggle('selected', c.getAttribute('data-do-id') === doItem.id);
    });

    selectedDOBadge.textContent = doItem.do_number;
    selectedDOBadge.className = `badge badge-${doItem.status === 'DELIVERED' ? 'success' : 'info'}`;

    // If already delivered, show success card directly
    if (doItem.status === 'DELIVERED') {
      noDOSelectedState.classList.add('hidden');
      podForm.classList.add('hidden');
      podSuccessCard.classList.remove('hidden');

      resStatus.textContent = 'DELIVERED';
      resNominal.textContent = formatIDR(doItem.clean_nominal || doItem.total_nominal);
      resInvoiceId.textContent = doItem.fledger_invoice_id || 'FC-INV-SYNCED';
      return;
    }

    // Show form, hide other states
    noDOSelectedState.classList.add('hidden');
    podSuccessCard.classList.add('hidden');
    podForm.classList.remove('hidden');

    // Populate outlet summary
    podCustomerName.textContent = doItem.customer_name || 'Pelanggan Toko';
    podCustomerAddress.textContent = doItem.destination_address || '-';
    podDONumber.textContent = doItem.do_number;
    podNominalOrdered.textContent = formatIDR(doItem.total_nominal || 0);

    // Initialize item state tracking if not already set
    (doItem.items || []).forEach(item => {
      if (item.delivered_qty === undefined) {
        item.delivered_qty = item.quantity || item.ordered_qty || 1;
      }
      if (item.rejected_qty === undefined) {
        item.rejected_qty = 0;
      }
      if (item.reject_reason === undefined) {
        item.reject_reason = '';
      }
    });

    // Reset pagination to page 1 for new selected DO
    paginationState.podItems.page = 1;
    renderPODItemsTable(doItem.items || []);
    
    // Reset Form fields
    podRecipientName.value = '';
    podRecipientPhone.value = '';
    podDriverNotes.value = '';
    podPhotoUrl.value = '';
    photoPreviewBox.classList.add('hidden');
    photoPreviewImg.src = '';
    clearCanvas();
  }

  function renderPODItemsTable(items) {
    if (!items || items.length === 0) {
      podItemsTbody.innerHTML = `<tr><td colspan="5" class="text-center text-muted">Tidak ada rincian item SKU.</td></tr>`;
      podCleanNominal.textContent = formatIDR(0);
      if (podItemsCountBadge) podItemsCountBadge.textContent = '0 SKU';
      renderPagination(podItemsPageInfo, podItemsPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'SKU', () => {});
      return;
    }

    if (podItemsCountBadge) {
      podItemsCountBadge.textContent = `${items.length} SKU Total`;
    }

    const paginated = paginateItems(items, paginationState.podItems.page, paginationState.podItems.pageSize);
    paginationState.podItems.page = paginated.page;

    podItemsTbody.innerHTML = paginated.pageItems.map((item, localIdx) => {
      const globalIndex = paginated.startIdx + localIdx;
      const unitPrice = item.unit_price || item.unit_price_cents || 0;
      const qtyOrdered = item.quantity || item.ordered_qty || 1;
      const deliveredVal = item.delivered_qty !== undefined ? item.delivered_qty : qtyOrdered;
      const rejectedVal = item.rejected_qty !== undefined ? item.rejected_qty : 0;
      const reasonVal = item.reject_reason || '';

      return `
        <tr data-item-idx="${globalIndex}" data-sku-id="${item.sku_id || item.sku || `SKU-${globalIndex}`}" data-unit-price="${unitPrice}" data-ordered-qty="${qtyOrdered}">
          <td>
            <strong>${item.name || item.product_name || item.sku_id || `Produk #${globalIndex + 1}`}</strong>
            <div class="text-sm text-muted">@ ${formatIDR(unitPrice)}</div>
          </td>
          <td class="text-center"><strong>${qtyOrdered}</strong></td>
          <td class="text-center">
            <input type="number" class="form-control text-center input-delivered" min="0" max="${qtyOrdered}" value="${deliveredVal}">
          </td>
          <td class="text-center">
            <input type="number" class="form-control text-center input-rejected" min="0" max="${qtyOrdered}" value="${rejectedVal}">
          </td>
          <td>
            <select class="form-control select-reason">
              <option value="" ${reasonVal === '' ? 'selected' : ''}>- Kondisi Baik -</option>
              <option value="KEMASAN_RUSAK" ${reasonVal === 'KEMASAN_RUSAK' ? 'selected' : ''}>Kemasan Rusak / Pecah</option>
              <option value="KADALUWARSA" ${reasonVal === 'KADALUWARSA' ? 'selected' : ''}>Dekat Kadaluwarsa</option>
              <option value="SALAH_BARANG" ${reasonVal === 'SALAH_BARANG' ? 'selected' : ''}>Salah Kirim Barang</option>
              <option value="TOKO_TUTUP" ${reasonVal === 'TOKO_TUTUP' ? 'selected' : ''}>Toko Menolak / Lebih</option>
            </select>
          </td>
        </tr>
      `;
    }).join('');

    // Attach listeners for dynamic calculation and state update
    podItemsTbody.querySelectorAll('tr').forEach(row => {
      const itemIdx = parseInt(row.getAttribute('data-item-idx'), 10);
      const deliveredInput = row.querySelector('.input-delivered');
      const rejectedInput = row.querySelector('.input-rejected');
      const reasonSelect = row.querySelector('.select-reason');
      const maxQty = parseInt(row.getAttribute('data-ordered-qty'), 10) || 0;
      const targetItem = items[itemIdx];

      deliveredInput.addEventListener('input', () => {
        let delivered = parseInt(deliveredInput.value, 10);
        if (isNaN(delivered)) delivered = 0;
        if (delivered > maxQty) delivered = maxQty;
        if (delivered < 0) delivered = 0;
        deliveredInput.value = delivered;

        // Auto balance rejected
        const rejected = maxQty - delivered;
        rejectedInput.value = rejected;
        if (rejected > 0 && !reasonSelect.value) {
          reasonSelect.value = 'KEMASAN_RUSAK';
        } else if (rejected === 0) {
          reasonSelect.value = '';
        }

        if (targetItem) {
          targetItem.delivered_qty = delivered;
          targetItem.rejected_qty = rejected;
          targetItem.reject_reason = reasonSelect.value;
        }
        recalculateCleanNominal();
      });

      rejectedInput.addEventListener('input', () => {
        let rejected = parseInt(rejectedInput.value, 10);
        if (isNaN(rejected)) rejected = 0;
        if (rejected > maxQty) rejected = maxQty;
        if (rejected < 0) rejected = 0;
        rejectedInput.value = rejected;

        // Auto balance delivered
        const delivered = maxQty - rejected;
        deliveredInput.value = delivered;
        if (rejected > 0 && !reasonSelect.value) {
          reasonSelect.value = 'KEMASAN_RUSAK';
        } else if (rejected === 0) {
          reasonSelect.value = '';
        }

        if (targetItem) {
          targetItem.delivered_qty = delivered;
          targetItem.rejected_qty = rejected;
          targetItem.reject_reason = reasonSelect.value;
        }
        recalculateCleanNominal();
      });

      reasonSelect.addEventListener('change', () => {
        if (targetItem) {
          targetItem.reject_reason = reasonSelect.value;
        }
      });
    });

    renderPagination(podItemsPageInfo, podItemsPageNav, paginated, 'SKU', (newPage) => {
      paginationState.podItems.page = newPage;
      renderPODItemsTable(items);
    });

    recalculateCleanNominal();
  }

  function recalculateCleanNominal() {
    let cleanTotal = 0;
    let anyRejected = false;

    const items = state.selectedDO ? (state.selectedDO.items || []) : [];

    items.forEach(item => {
      const unitPrice = item.unit_price || item.unit_price_cents || 0;
      const delivered = item.delivered_qty !== undefined ? item.delivered_qty : (item.quantity || item.ordered_qty || 1);
      const rejected = item.rejected_qty !== undefined ? item.rejected_qty : 0;

      cleanTotal += (delivered * unitPrice);
      if (rejected > 0) anyRejected = true;
    });

    podCleanNominal.textContent = formatIDR(cleanTotal);

    // Highlight photo requirement if any items rejected
    if (anyRejected) {
      photoSection.classList.add('required-highlight');
    } else {
      photoSection.classList.remove('required-highlight');
    }
  }

  // Photo sample demo helper
  btnFillSamplePhoto.addEventListener('click', () => {
    podPhotoUrl.value = 'https://images.unsplash.com/photo-1584308666744-24d5c474f2ae?w=600&auto=format&fit=crop&q=80';
    photoPreviewImg.src = podPhotoUrl.value;
    photoPreviewBox.classList.remove('hidden');
    showToast('Foto bukti demo berhasil disematkan', 'info');
  });

  podPhotoUrl.addEventListener('input', () => {
    const url = podPhotoUrl.value.trim();
    if (url) {
      photoPreviewImg.src = url;
      photoPreviewBox.classList.remove('hidden');
    } else {
      photoPreviewBox.classList.add('hidden');
    }
  });

  // Submit POD Action
  podForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    if (!state.selectedDO) return;

    const recipientName = podRecipientName.value.trim();
    if (!recipientName) {
      showToast('Nama penerima barang wajib diisi!', 'warning');
      podRecipientName.focus();
      return;
    }

    if (!state.hasSignature) {
      showToast('Tanda tangan digital penerima wajib dibubuhkan di kotak canvas!', 'warning');
      return;
    }

    // Collect all items across all pages
    const itemsResult = [];
    let hasRejected = false;

    const allItems = state.selectedDO.items || [];
    allItems.forEach(item => {
      const skuId = item.sku_id || item.product_sku || item.sku;
      const delivered = item.delivered_qty !== undefined ? item.delivered_qty : (item.quantity || item.ordered_qty || 1);
      const rejected = item.rejected_qty !== undefined ? item.rejected_qty : 0;
      const reason = item.reject_reason || '';

      if (rejected > 0) hasRejected = true;

      itemsResult.push({
        sku_id: skuId,
        product_sku: skuId,
        delivered_qty: delivered,
        qty_delivered: delivered,
        rejected_qty: rejected,
        qty_rejected: rejected,
        reject_reason: rejected > 0 ? (reason || 'KEMASAN_RUSAK') : '',
        rejection_reason: rejected > 0 ? (reason || 'DAMAGED_LEAK') : ''
      });
    });

    // Validate photo evidence when rejection occurs
    const photoUrl = podPhotoUrl.value.trim();
    if (hasRejected && !photoUrl) {
      showToast('Wajib melampirkan foto bukti fisik untuk barang yang rusak/ditolak!', 'warning');
      podPhotoUrl.focus();
      return;
    }

    const signatureData = signatureCanvas.toDataURL('image/png');

    const podPayload = {
      recipient_name: recipientName,
      recipient_phone: podRecipientPhone.value.trim(),
      driver_notes: podDriverNotes.value.trim(),
      signature_data_url: signatureData,
      photo_evidence_url: photoUrl,
      photo_evidence_urls: photoUrl ? [photoUrl] : [],
      items_result: itemsResult,
      items: itemsResult
    };

    btnSubmitPOD.disabled = true;
    btnSubmitPOD.innerHTML = `<span class="icon">⏳</span> Menerbitkan Faktur Bersih ke Core...`;

    try {
      const result = await client.submitPOD(state.selectedDO.id, podPayload);
      showToast('Proof of Delivery sukses! Faktur otomatis diterbitkan.', 'success');

      // Update state & show Success Card
      podForm.classList.add('hidden');
      podSuccessCard.classList.remove('hidden');

      resStatus.textContent = result.status || 'DELIVERED';
      resNominal.textContent = podCleanNominal.textContent;
      resInvoiceId.textContent = result.fledger_invoice_id || result.invoice_id || 'OUTBOX_SYNC_QUEUED';

      // Reload driver lists & master
      await loadDriverView();
    } catch (err) {
      showToast(`Gagal submit POD: ${err.message}`, 'error');
    } finally {
      btnSubmitPOD.disabled = false;
      btnSubmitPOD.innerHTML = `<span class="icon">⚡</span> Submit POD & Terbitkan Faktur ke Fledger Core`;
    }
  });

  btnBackToDOList.addEventListener('click', () => {
    podSuccessCard.classList.add('hidden');
    noDOSelectedState.classList.remove('hidden');
    state.selectedDO = null;
    loadDriverView();
  });

  btnRefreshDriver.addEventListener('click', loadDriverView);

  // ----------------------------------------------------
  // Tab 2: Dispatcher & Trip Hub
  // ----------------------------------------------------
  async function loadDispatcherView() {
    tripNumberInput.value = generateId('TRIP');
    await Promise.all([
      loadVehiclesAndDriversForDispatch(),
      loadPendingDOsForDispatch(),
      loadTripsList(),
      loadOutboxStats()
    ]);
  }

  async function loadVehiclesAndDriversForDispatch() {
    try {
      const [vRes, dRes] = await Promise.all([
        client.listVehicles(),
        client.listDrivers()
      ]);

      const vehicles = Array.isArray(vRes) ? vRes : (vRes.vehicles || []);
      const drivers = Array.isArray(dRes) ? dRes : (dRes.drivers || []);

      state.vehicles = vehicles;
      state.drivers = drivers;

      // Populate Vehicle Dropdown
      tripVehicleSelect.innerHTML = vehicles.length === 0
        ? '<option value="">Belum ada armada terdaftar</option>'
        : '<option value="">-- Pilih Kendaraan Tersedia --</option>' + vehicles.map(v => `
            <option value="${v.id}" data-capacity="${v.capacity_kg || 1500}">
              ${v.license_plate} - ${v.type} (${v.capacity_kg} kg) [${v.status}]
            </option>
          `).join('');

      // Populate Driver Dropdown
      tripDriverSelect.innerHTML = drivers.length === 0
        ? '<option value="">Belum ada supir terdaftar</option>'
        : '<option value="">-- Pilih Supir Ekspedisi --</option>' + drivers.map(d => `
            <option value="${d.id}">
              ${d.name} (${d.phone}) [${d.status}]
            </option>
          `).join('');

    } catch (err) {
      console.error('Failed loading vehicles/drivers for dispatch:', err);
    }
  }

  async function loadPendingDOsForDispatch() {
    try {
      const doRes = await client.listDeliveryOrders();
      const dos = Array.isArray(doRes) ? doRes : (doRes.delivery_orders || []);
      state.dos = dos;

      // Filter DOs ready for dispatch (not yet delivered or assigned)
      const availableDOs = dos.filter(d => d.status !== 'DELIVERED');

      if (availableDOs.length === 0) {
        tripDOCheckboxes.innerHTML = `
          <div class="text-sm text-muted p-2">
            Semua Surat Jalan sudah selesai atau belum ada DO aktif. Gunakan tab Master Data untuk buat DO baru.
          </div>
        `;
        recalculateTripCapacity();
        return;
      }

      tripDOCheckboxes.innerHTML = availableDOs.map(d => {
        const estWeight = d.total_weight_kg || 250;
        return `
          <label class="do-checkbox-item" data-do-id="${d.id}" data-weight="${estWeight}">
            <input type="checkbox" name="dispatch_dos" value="${d.id}">
            <span>
              <strong>${d.do_number}</strong> — ${d.customer_name} 
              <span class="text-muted">(${estWeight} kg | ${formatIDR(d.total_nominal || 0)})</span>
            </span>
          </label>
        `;
      }).join('');

      // Attach listener to checkboxes
      tripDOCheckboxes.querySelectorAll('input[type="checkbox"]').forEach(cb => {
        cb.addEventListener('change', recalculateTripCapacity);
      });

      recalculateTripCapacity();
    } catch (err) {
      console.error('Failed loading pending DOs:', err);
    }
  }

  function recalculateTripCapacity() {
    let totalWeight = 0;
    tripDOCheckboxes.querySelectorAll('input[type="checkbox"]:checked').forEach(cb => {
      const row = cb.closest('.do-checkbox-item');
      const w = parseFloat(row.getAttribute('data-weight')) || 0;
      totalWeight += w;
    });

    const selectedOption = tripVehicleSelect.selectedOptions[0];
    const capacity = parseFloat(selectedOption?.getAttribute('data-capacity')) || 0;

    estWeightKg.textContent = totalWeight;
    maxCapacityKg.textContent = capacity;

    if (capacity > 0) {
      const pct = Math.min(100, Math.round((totalWeight / capacity) * 100));
      weightProgressFill.style.width = `${pct}%`;

      if (totalWeight > capacity) {
        weightProgressFill.style.backgroundColor = '#ef4444';
        capacityWarning.classList.remove('hidden');
      } else if (pct > 80) {
        weightProgressFill.style.backgroundColor = '#f59e0b';
        capacityWarning.classList.add('hidden');
      } else {
        weightProgressFill.style.backgroundColor = '#10b981';
        capacityWarning.classList.add('hidden');
      }
    } else {
      weightProgressFill.style.width = '0%';
      capacityWarning.classList.add('hidden');
    }
  }

  tripVehicleSelect.addEventListener('change', recalculateTripCapacity);

  // Create Trip
  tripForm.addEventListener('submit', async (e) => {
    e.preventDefault();

    const tripNumber = tripNumberInput.value.trim();
    const vehicleId = tripVehicleSelect.value;
    const driverId = tripDriverSelect.value;
    const notes = tripNotesInput.value.trim();

    const checkedDOs = Array.from(tripDOCheckboxes.querySelectorAll('input[type="checkbox"]:checked'))
      .map(cb => cb.value);

    if (!vehicleId) {
      showToast('Pilih kendaraan ekspedisi!', 'warning');
      return;
    }
    if (!driverId) {
      showToast('Pilih supir untuk trip ini!', 'warning');
      return;
    }
    if (checkedDOs.length === 0) {
      showToast('Pilih minimal 1 Surat Jalan untuk dimuat ke truk!', 'warning');
      return;
    }

    try {
      await client.createTrip({
        trip_number: tripNumber,
        vehicle_id: vehicleId,
        driver_id: driverId,
        notes: notes,
        do_ids: checkedDOs
      });

      showToast(`Trip ${tripNumber} berhasil dibuat!`, 'success');
      tripNumberInput.value = generateId('TRIP');
      tripNotesInput.value = '';
      loadDispatcherView();
    } catch (err) {
      showToast(`Gagal membuat trip: ${err.message}`, 'error');
    }
  });

  async function loadTripsList() {
    try {
      const res = await client.listTrips();
      const trips = Array.isArray(res) ? res : (res.trips || []);
      state.trips = trips;

      if (trips.length === 0) {
        tripsTbody.innerHTML = `<tr><td colspan="5" class="text-center text-muted">Belum ada trip aktif.</td></tr>`;
        renderPagination(tripsPageInfo, tripsPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Trip', () => {});
        return;
      }

      renderTripsTable();
    } catch (err) {
      tripsTbody.innerHTML = `<tr><td colspan="5" class="text-danger">Gagal memuat trips: ${err.message}</td></tr>`;
      renderPagination(tripsPageInfo, tripsPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Trip', () => {});
    }
  }

  function renderTripsTable() {
    const trips = state.trips || [];
    const paginated = paginateItems(trips, paginationState.trips.page, paginationState.trips.pageSize);
    paginationState.trips.page = paginated.page;

    tripsTbody.innerHTML = paginated.pageItems.map(t => {
      const isDraft = t.status === 'DRAFT';
      const isCompleted = t.status === 'COMPLETED';
      const isDispatched = t.status === 'DISPATCHED' || t.status === 'IN_TRANSIT';

      let badgeClass = 'badge-info';
      if (isCompleted) badgeClass = 'badge-success';
      if (isDispatched) badgeClass = 'badge-warning';

      const stopsCount = (t.stops || t.do_ids || []).length;

      return `
        <tr>
          <td><strong>${t.trip_number}</strong></td>
          <td>
            <div>🚚 ${t.vehicle_plate || t.vehicle_id?.slice(0, 8) || '-'}</div>
            <div class="text-sm text-muted">👤 ${t.driver_name || t.driver_id?.slice(0, 8) || '-'}</div>
          </td>
          <td><span class="badge ${badgeClass}">${t.status}</span></td>
          <td>${stopsCount} Toko</td>
          <td>
            ${isDraft ? `
              <button class="btn btn-sm btn-primary btn-dispatch-trip" data-trip-id="${t.id}" type="button">
                🚀 Berangkatkan
              </button>
            ` : (isDispatched ? `<span class="text-sm text-warning">🚚 Sedang Jalan</span>` : `<span class="text-sm text-success">✅ Selesai</span>`)}
          </td>
        </tr>
      `;
    }).join('');

    // Attach dispatch handlers
    tripsTbody.querySelectorAll('.btn-dispatch-trip').forEach(btn => {
      btn.addEventListener('click', async () => {
        const tripId = btn.getAttribute('data-trip-id');
        btn.disabled = true;
        try {
          await client.dispatchTrip(tripId);
          showToast('Trip diberangkatkan! Armada dalam perjalanan.', 'success');
          loadTripsList();
        } catch (err) {
          showToast(`Gagal memberangkatkan trip: ${err.message}`, 'error');
          btn.disabled = false;
        }
      });
    });

    renderPagination(tripsPageInfo, tripsPageNav, paginated, 'Trip', (newPage) => {
      paginationState.trips.page = newPage;
      renderTripsTable();
    });
  }

  async function loadOutboxStats() {
    try {
      const counts = await client.getOutboxCounts();
      outboxPending.textContent = counts.pending || 0;
      outboxSent.textContent = counts.sent || 0;
      outboxFailed.textContent = counts.failed || 0;
    } catch (err) {
      console.warn('Failed to load outbox counts:', err);
    }
  }

  btnRefreshTrips.addEventListener('click', loadDispatcherView);

  // ----------------------------------------------------
  // Tab 3: Master Data
  // ----------------------------------------------------
  async function loadMasterView() {
    await Promise.all([
      loadMasterDOs(),
      loadMasterVehicles(),
      loadMasterDrivers()
    ]);
  }

  async function loadMasterDOs() {
    try {
      const res = await client.listDeliveryOrders();
      const dos = Array.isArray(res) ? res : (res.delivery_orders || []);
      state.dos = dos;

      if (dos.length === 0) {
        masterDOTbody.innerHTML = `<tr><td colspan="5" class="text-center text-muted">Belum ada Surat Jalan. Klik 'Penerbitan Surat Jalan Demo' di sebelah kiri.</td></tr>`;
        renderPagination(masterDOPageInfo, masterDOPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Surat Jalan', () => {});
        return;
      }

      renderMasterDOTable();
    } catch (err) {
      masterDOTbody.innerHTML = `<tr><td colspan="5" class="text-danger">Error: ${err.message}</td></tr>`;
      renderPagination(masterDOPageInfo, masterDOPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Surat Jalan', () => {});
    }
  }

  function renderMasterDOTable() {
    const dos = state.dos || [];
    const paginated = paginateItems(dos, paginationState.masterDO.page, paginationState.masterDO.pageSize);
    paginationState.masterDO.page = paginated.page;

    masterDOTbody.innerHTML = paginated.pageItems.map(d => `
      <tr>
        <td><strong>${d.do_number}</strong></td>
        <td>
          <div>${d.customer_name}</div>
          <div class="text-sm text-muted">${d.destination_address || ''}</div>
        </td>
        <td>${(d.items || []).length} SKU</td>
        <td><strong>${formatIDR(d.total_nominal || 0)}</strong></td>
        <td><span class="badge badge-${d.status === 'DELIVERED' ? 'success' : 'info'}">${d.status}</span></td>
      </tr>
    `).join('');

    renderPagination(masterDOPageInfo, masterDOPageNav, paginated, 'Surat Jalan', (newPage) => {
      paginationState.masterDO.page = newPage;
      renderMasterDOTable();
    });
  }

  async function loadMasterVehicles() {
    try {
      const res = await client.listVehicles();
      const vehicles = Array.isArray(res) ? res : (res.vehicles || []);
      state.vehicles = vehicles;

      if (vehicles.length === 0) {
        masterVehiclesTbody.innerHTML = `<tr><td colspan="4" class="text-center text-muted">Belum ada kendaraan.</td></tr>`;
        renderPagination(masterVehiclesPageInfo, masterVehiclesPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Kendaraan', () => {});
        return;
      }

      renderMasterVehiclesTable();
    } catch (err) {
      masterVehiclesTbody.innerHTML = `<tr><td colspan="4" class="text-danger">Error: ${err.message}</td></tr>`;
      renderPagination(masterVehiclesPageInfo, masterVehiclesPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Kendaraan', () => {});
    }
  }

  function renderMasterVehiclesTable() {
    const vehicles = state.vehicles || [];
    const paginated = paginateItems(vehicles, paginationState.masterVehicles.page, paginationState.masterVehicles.pageSize);
    paginationState.masterVehicles.page = paginated.page;

    masterVehiclesTbody.innerHTML = paginated.pageItems.map(v => `
      <tr>
        <td><strong>${v.license_plate}</strong></td>
        <td>${v.type}</td>
        <td>${v.capacity_kg} kg</td>
        <td><span class="badge badge-${v.status === 'AVAILABLE' ? 'success' : 'warning'}">${v.status}</span></td>
      </tr>
    `).join('');

    renderPagination(masterVehiclesPageInfo, masterVehiclesPageNav, paginated, 'Kendaraan', (newPage) => {
      paginationState.masterVehicles.page = newPage;
      renderMasterVehiclesTable();
    });
  }

  async function loadMasterDrivers() {
    try {
      const res = await client.listDrivers();
      const drivers = Array.isArray(res) ? res : (res.drivers || []);
      state.drivers = drivers;

      if (drivers.length === 0) {
        masterDriversTbody.innerHTML = `<tr><td colspan="4" class="text-center text-muted">Belum ada supir.</td></tr>`;
        renderPagination(masterDriversPageInfo, masterDriversPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Supir', () => {});
        return;
      }

      renderMasterDriversTable();
    } catch (err) {
      masterDriversTbody.innerHTML = `<tr><td colspan="4" class="text-danger">Error: ${err.message}</td></tr>`;
      renderPagination(masterDriversPageInfo, masterDriversPageNav, { total: 0, totalPages: 1, page: 1, startIdx: 0, endIdx: 0 }, 'Supir', () => {});
    }
  }

  function renderMasterDriversTable() {
    const drivers = state.drivers || [];
    const paginated = paginateItems(drivers, paginationState.masterDrivers.page, paginationState.masterDrivers.pageSize);
    paginationState.masterDrivers.page = paginated.page;

    masterDriversTbody.innerHTML = paginated.pageItems.map(d => `
      <tr>
        <td><strong>${d.name}</strong></td>
        <td>${d.phone}</td>
        <td>${d.sim_number || '-'}</td>
        <td><span class="badge badge-${d.status === 'AVAILABLE' ? 'success' : 'warning'}">${d.status}</span></td>
      </tr>
    `).join('');

    renderPagination(masterDriversPageInfo, masterDriversPageNav, paginated, 'Supir', (newPage) => {
      paginationState.masterDrivers.page = newPage;
      renderMasterDriversTable();
    });
  }

  // Create Vehicle
  vehicleForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const plate = document.getElementById('vPlate').value.trim();
    const type = document.getElementById('vType').value;
    const capacity = parseInt(document.getElementById('vCapacity').value, 10) || 1500;

    try {
      await client.createVehicle({ license_plate: plate, type: type, capacity_kg: capacity });
      showToast(`Truk ${plate} berhasil ditambahkan!`, 'success');
      document.getElementById('vPlate').value = '';
      loadMasterVehicles();
    } catch (err) {
      showToast(`Gagal menambahkan truk: ${err.message}`, 'error');
    }
  });

  // Create Driver
  driverForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    const name = document.getElementById('dName').value.trim();
    const phone = document.getElementById('dPhone').value.trim();
    const sim = document.getElementById('dSim').value.trim();

    try {
      await client.createDriver({ name: name, phone: phone, sim_number: sim });
      showToast(`Supir ${name} berhasil ditambahkan!`, 'success');
      document.getElementById('dName').value = '';
      document.getElementById('dPhone').value = '';
      document.getElementById('dSim').value = '';
      loadMasterDrivers();
    } catch (err) {
      showToast(`Gagal menambahkan supir: ${err.message}`, 'error');
    }
  });

  // Generate Quick Demo DO
  btnGenerateDemoDO.addEventListener('click', async () => {
    btnGenerateDemoDO.disabled = true;
    btnGenerateDemoDO.innerHTML = `<span>⏳ Menerbitkan DO Demo...</span>`;

    const customerNames = [
      { name: 'Toko Sumber Rezeki', addr: 'Jl. Raya Daan Mogot No. 45, Jakarta Barat' },
      { name: 'Minimarket Barokah Jaya', addr: 'Jl. Surya Kencana No. 12, Bogor' },
      { name: 'Grosir Sembako Makmur', addr: 'Jl. KH Hasyim Ashari No. 88, Tangerang' },
      { name: 'Warung Madura Berkah', addr: 'Jl. Fatmawati Raya No. 104, Jakarta Selatan' },
      { name: 'Toko Sembako Sejahtera', addr: 'Jl. Mangga Dua Raya No. 15, Jakarta Pusat' },
      { name: 'Kios Beras Bu Rahma', addr: 'Pasar Induk Kramat Jati Blok C No. 22', Jakarta Timur }
    ];
    const picked = customerNames[Math.floor(Math.random() * customerNames.length)];
    const doNumber = generateId('DO');

    const demoPayload = {
      do_number: doNumber,
      customer_id: 'a0000000-0000-0000-0000-000000000002',
      customer_name: picked.name,
      destination_address: picked.addr,
      total_nominal: 1850000,
      total_weight_kg: 320,
      items: [
        { sku_id: 'SKU-OIL-BIMOLI-2L', product_sku: 'SKU-OIL-BIMOLI-2L', name: 'Minyak Goreng Bimoli 2L (Dus)', product_name: 'Minyak Goreng Bimoli 2L (Dus)', quantity: 10, qty_ordered: 10, unit_price: 120000, unit_price_cents: 120000 },
        { sku_id: 'SKU-MIE-INDOMIE-GRG', product_sku: 'SKU-MIE-INDOMIE-GRG', name: 'Indomie Goreng Original (Dus)', product_name: 'Indomie Goreng Original (Dus)', quantity: 5, qty_ordered: 5, unit_price: 110000, unit_price_cents: 110000 },
        { sku_id: 'SKU-GULA-GULAKU-1KG', product_sku: 'SKU-GULA-GULAKU-1KG', name: 'Gulaku Tebu Kuning 1kg (Karton)', product_name: 'Gulaku Tebu Kuning 1kg (Karton)', quantity: 2, qty_ordered: 2, unit_price: 50000, unit_price_cents: 50000 }
      ]
    };

    try {
      await client.createDeliveryOrder(demoPayload);
      showToast(`Surat Jalan ${doNumber} berhasil diterbitkan!`, 'success');
      loadMasterDOs();
    } catch (err) {
      showToast(`Gagal membuat DO Demo: ${err.message}`, 'error');
    } finally {
      btnGenerateDemoDO.disabled = false;
      btnGenerateDemoDO.innerHTML = `
        <svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="m7.5 4.27 9 5.15"/>
          <path d="M21 8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16Z"/>
          <path d="m3.3 7 8.7 5 8.7-5"/>
          <path d="M12 22V12"/>
        </svg>
        <span>Terbitkan 1 Surat Jalan Demo (Minyak Goreng &amp; Mie Instan)</span>
      `;
    }
  });

  // ----------------------------------------------------
  // Global Refresh & Initialization
  // ----------------------------------------------------
  async function refreshAll() {
    await checkHealth();
    if (state.activeTab === 'driver-tab') await loadDriverView();
    if (state.activeTab === 'dispatcher-tab') await loadDispatcherView();
    if (state.activeTab === 'master-tab') await loadMasterView();
  }

  // Initialize
  initCanvas();
  checkHealth();

  // If token is missing, attempt auto dev login
  if (!state.token) {
    performLogin(true);
  } else {
    refreshAll();
  }

  // Periodic health check & outbox refresh every 15s
  setInterval(() => {
    checkHealth();
    if (state.activeTab === 'dispatcher-tab') loadOutboxStats();
  }, 15000);
});
