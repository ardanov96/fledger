// Load test (Sprint 18) — k6 script for /v1/transfers endpoint.
//
// Run with:
//   docker run --rm -i grafana/k6 run - <load-test/transfer.js
// or locally after `k6 install`:
//   k6 run --duration 30s --vus 50 load-test/transfer.js
//
// Assumes you have a running API + a pre-seeded test user with 2 accounts.
// Replace BASE_URL, TOKEN, FROM_ACCT, TO_ACCT, AMOUNT_MINOR before running.

import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
  stages: [
    { duration: '10s', target: 20 },   // ramp up
    { duration: '20s', target: 50 },   // steady-state load
    { duration: '10s', target: 100 },  // spike
    { duration: '10s', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<500'],  // 95th < 200ms, 99th < 500ms
    http_req_failed:   ['rate<0.01'],               // < 1% errors
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN    = __ENV.TOKEN    || 'test-jwt-token';
const FROM     = __ENV.FROM_ACCT || '00000000-0000-0000-0000-000000000001';
const TO       = __ENV.TO_ACCT   || '00000000-0000-0000-0000-000000000002';

export default function () {
  const url = `${BASE_URL}/v1/transfers`;
  const idemKey = `loadtest-${__VU}-${__ITER}-${Date.now()}`;
  const payload = JSON.stringify({
    from_account_id: FROM,
    to_account_id:   TO,
    amount_minor:    100,         // IDR 1.00 per transfer
    currency:        'IDR',
    description:     'k6 load test transfer',
  });

  const params = {
    headers: {
      'Content-Type':      'application/json',
      'Authorization':     `Bearer ${TOKEN}`,
      'Idempotency-Key':   idemKey,
      'X-Tenant-ID':       '00000000-0000-0000-0000-000000000001',
    },
  };

  const res = http.post(url, payload, params);

  check(res, {
    'status is 201 or 200': (r) => r.status === 201 || r.status === 200,
    'has transaction_id':    (r) => r.json('data') !== null,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });

  sleep(0.05); // 50ms between requests
}
