/**
 * Fledger Fleet API Client SDK
 * Reusable client for Dispatcher Portals and Driver Mobile Web (including ekspedisi-dashboard).
 */
class FledgerFleetClient {
  constructor(baseUrl = 'http://localhost:8082', token = '', tenantId = '00000000-0000-0000-0000-000000000001') {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.token = token;
    this.tenantId = tenantId;
  }

  setToken(token) {
    this.token = token;
  }

  setTenant(tenantId) {
    this.tenantId = tenantId;
  }

  async request(path, options = {}) {
    const url = `${this.baseUrl}${path}`;
    const headers = {
      'Content-Type': 'application/json',
      'X-Tenant-ID': this.tenantId,
      ...(this.token ? { 'Authorization': `Bearer ${this.token}` } : {}),
      ...(options.headers || {})
    };

    const response = await fetch(url, { ...options, headers });
    const data = await response.json().catch(() => ({}));

    if (!response.ok) {
      const err = new Error(data.message || data.error || `HTTP ${response.status}`);
      err.status = response.status;
      err.data = data;
      throw err;
    }
    return data;
  }

  // Auth / Dev Login
  async devLogin(tenantId = this.tenantId) {
    const res = await this.request(`/v1/dev/login?tenant_id=${tenantId}`, { method: 'POST' });
    if (res.access_token) {
      this.token = res.access_token;
    }
    return res;
  }

  // Health
  async health() {
    return this.request('/healthz');
  }

  // Master Data: Vehicles
  async listVehicles(status = '') {
    const q = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request(`/v1/fleet/vehicles${q}`);
  }

  async createVehicle(payload) {
    return this.request('/v1/fleet/vehicles', {
      method: 'POST',
      body: JSON.stringify(payload)
    });
  }

  // Master Data: Drivers
  async listDrivers(status = '') {
    const q = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request(`/v1/fleet/drivers${q}`);
  }

  async createDriver(payload) {
    return this.request('/v1/fleet/drivers', {
      method: 'POST',
      body: JSON.stringify(payload)
    });
  }

  // Trips & Dispatching
  async listTrips(status = '') {
    const q = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request(`/v1/fleet/trips${q}`);
  }

  async listTodayTrips() {
    return this.request('/v1/fleet/trips/today');
  }

  async createTrip(payload) {
    return this.request('/v1/fleet/trips', {
      method: 'POST',
      body: JSON.stringify(payload)
    });
  }

  async dispatchTrip(tripId) {
    return this.request(`/v1/fleet/trips/${tripId}/dispatch`, {
      method: 'POST'
    });
  }

  // Delivery Orders (DO)
  async listDeliveryOrders(status = '') {
    const q = status ? `?status=${encodeURIComponent(status)}` : '';
    return this.request(`/v1/fleet/delivery-orders${q}`);
  }

  async getDeliveryOrder(doId) {
    return this.request(`/v1/fleet/delivery-orders/${doId}`);
  }

  async createDeliveryOrder(payload) {
    return this.request('/v1/fleet/delivery-orders', {
      method: 'POST',
      body: JSON.stringify(payload)
    });
  }

  // Digital Proof of Delivery (POD)
  async submitPOD(doId, podPayload) {
    return this.request(`/v1/fleet/delivery-orders/${doId}/pod`, {
      method: 'POST',
      body: JSON.stringify(podPayload)
    });
  }

  // Outbox Status
  async getOutboxCounts() {
    return this.request('/v1/fleet/outbox/counts');
  }
}

// Export for module/commonjs/browser
if (typeof module !== 'undefined' && module.exports) {
  module.exports = FledgerFleetClient;
} else if (typeof window !== 'undefined') {
  window.FledgerFleetClient = FledgerFleetClient;
}
