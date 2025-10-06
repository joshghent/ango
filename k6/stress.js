import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '2m', target: 1000 },  // Ramp up to 1000 VUs over 2 minutes
    { duration: '3m', target: 1000 },  // Hold at 1000 VUs for 3 minutes
    { duration: '2m', target: 0 },     // Ramp down over 2 minutes
  ],
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<1000'], // More lenient under stress
    errors: ['rate<0.2'], // Error rate < 20% under stress
    http_req_failed: ['rate<0.1'], // Request failure rate < 10%
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:3000';

const TEST_BATCHES = [
  { batchId: '11111111-1111-1111-1111-111111111111', clientId: '217be7c8-679c-4e08-bffc-db3451bdcdbf' },
  { batchId: '22222222-2222-2222-2222-222222222222', clientId: '317be7c8-679c-4e08-bffc-db3451bdcdbf' },
  { batchId: '33333333-3333-3333-3333-333333333333', clientId: '417be7c8-679c-4e08-bffc-db3451bdcdbf' },
];

function generateCustomerId() {
  const chars = '0123456789abcdef';
  let result = '';
  for (let i = 0; i < 32; i++) {
    result += chars[Math.floor(Math.random() * 16)];
    if (i === 7 || i === 11 || i === 15 || i === 19) {
      result += '-';
    }
  }
  return result;
}

export default function () {
  // More aggressive - 90% redemptions, 10% queries
  if (Math.random() < 0.9) {
    testCodeRedemption();
  } else {
    testBatchQuery();
  }

  // Shorter sleep for more aggressive testing
  sleep(0.05);
}

function testCodeRedemption() {
  const batch = TEST_BATCHES[Math.floor(Math.random() * TEST_BATCHES.length)];
  const customerId = generateCustomerId();

  const payload = JSON.stringify({
    batchid: batch.batchId,
    clientid: batch.clientId,
    customerid: customerId,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
    },
    tags: { name: 'stress_redemption' },
  };

  const response = http.post(`${BASE_URL}/api/v1/code/redeem`, payload, params);

  const success = check(response, {
    'stress redemption status is valid': (r) => [200, 403, 404, 503].includes(r.status), // Include 503 for circuit breaker
    'stress redemption response time < 1s': (r) => r.timings.duration < 1000,
  });

  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }
}

function testBatchQuery() {
  const response = http.get(`${BASE_URL}/api/v1/batches`);

  const success = check(response, {
    'stress batch query status is valid': (r) => [200, 503].includes(r.status),
    'stress batch query response time < 500ms': (r) => r.timings.duration < 500,
  });

  if (!success) {
    errorRate.add(1);
  } else {
    errorRate.add(0);
  }
}

export function handleSummary(data) {
  return {
    'stress-results.json': JSON.stringify(data, null, 2),
    stdout: `
    ========== STRESS TEST RESULTS ==========

    Total Requests: ${data.metrics.http_reqs.values.count}
    Request Rate: ${data.metrics.http_reqs.values.rate.toFixed(2)}/s

    Performance Under Stress:
    - p50: ${data.metrics.http_req_duration.values['p(50)'].toFixed(2)}ms
    - p95: ${data.metrics.http_req_duration.values['p(95)'].toFixed(2)}ms
    - p99: ${data.metrics.http_req_duration.values['p(99)'].toFixed(2)}ms

    Error Rate: ${(data.metrics.errors.values.rate * 100).toFixed(2)}%
    Failed Requests: ${(data.metrics.http_req_failed.values.rate * 100).toFixed(2)}%

    ==========================================
    `,
  };
}